// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"container/heap"
	"errors"
	"io"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

//go:generate gencodec -type txdata -field-override txdataMarshaling -out gen_tx_json.go

var (
	ErrInvalidSig = errors.New("invalid transaction v, r, s values")
)

// author : zr
// CM 承诺结构，包含承诺字段以及判断该承诺是否被使用过的spent字段，true表示已使用
type CM struct {
	Cm    uint64
	Spent bool
}

func NewCM(cm uint64, spent bool) *CM {
	return &CM{
		Cm:    cm,
		Spent: spent,
	}
}

func (cm *CM) Hash() common.Hash {
	return rlpHash(cm)
}

type Transaction struct {
	data txdata //一个不限制大小的字节数组，用来指定消息调用的输入数据
	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

type txdata struct {
	AccountNonce uint64          `json:"nonce"    gencodec:"required"` //由交易发送者发出的的交易的数量，由 Tn 表示
	Price        *big.Int        `json:"gasPrice" gencodec:"required"` //为执行这个交易所需要进行的计算步骤消 耗的每单位 gas 的价格，以 Wei 为单位，由 Tp 表 示。
	GasLimit     uint64          `json:"gas"      gencodec:"required"` //用于执行这个交易的最大 gas 数量。这个值须在交易开始前设置，且设定后不能再增加，由Tg 表示。
	Recipient    *common.Address `json:"to"       rlp:"nil"`           // nil means contract creation 160 位的消息调用接收者地址；对与合约创建交易，用 ∅ 表示 B0 的唯一成员。此字段由 Tt 表示
	Amount       *big.Int        `json:"value"    gencodec:"required"` //转移到接收者账户的 Wei 的数量；对于合约 创建，则代表给新建合约地址的初始捐款。由 Tv 表示。
	Payload      []byte          `json:"input"    gencodec:"required"` //如果目标账户包含代码，该代码会执行，payload就是输入数据。如果目标账户是零账户（账户地址是0），交易将创建一个新合约。这个合约地址不是零地址，而是由合约创建者的地址和该地址发出过的交易数量（被称为nonce）计算得到。创建合约交易的payload被当作EVM字节码执行。执行的输出做为合约代码被永久存储。这意味着，为了创建一个合约，你不需要向合约发送真正的合约代码，而是发送能够返回真正代码的代码。
	SnO          uint64          `json:"SnO"      gencodec:"required"` //代币序列号
	Rr1          uint64          `json:"Rr1"      gencodec:"required"` //随机数，交易时对交易金额v_r进行加密
	CmSpk        uint64          `json:"CmSpk"    gencodec:"required"` //发送方公钥的承诺
	CmRpk        uint64          `json:"CmRpk"    gencodec:"required"` //接收方公钥的承诺
	CmO          uint64          `json:"CmO"      gencodec:"required"` //原始金额承诺
	CmS          uint64          `json:"CmS"      gencodec:"required"` //消费金额承诺
	CmR          uint64          `json:"CmR"      gencodec:"required"` //找零金额承诺
	EvR          uint64          `json:"EvR"      gencodec:"required"` //E(v_r) = (v_r * G1_R + r_r2 * H_R, r_r2 * G2_R)
	EvR0         uint64          `json:"EvR0"     gencodec:"required"` //EvR 的后64位
	EvR_         uint64          `json:"EvR_"     gencodec:"required"` //E(v_r)’ = (v_r * G1 + r_r3 * H, r_r3 * G2；S_pk * G1 + r_spk * H，r_spk * G2；R_pk * G1 + r_rpk * H，r_rpk * G2)
	EvR_0        uint64          `json:"EvR_0"    gencodec:"required"` //EvR_ 的后64位
	PI           uint64          `json:"PI"       gencodec:"required"` //零知识证明Π
	ID           uint64          `json:"ID"       gencodec:"required"` //购币标识
	Sig          string          `json:"Sig"      gencodec:"required"` //发行者签名
	CmV          uint64          `json:"CmV"      gencodec:"required"` //购币承诺
	EpkV         uint64          `json:"EpkV"     gencodec:"required"` //E(pk,v),监管者公钥对购币用户公钥和购币金额的加密
	// Signature values
	V *big.Int `json:"v" gencodec:"required"` //v, r, s: 与交易签名相符的若干数值，用于确定交易的发送者，由 Tw，Tr 和 Ts 表示。
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

type txdataMarshaling struct {
	AccountNonce hexutil.Uint64
	Price        *hexutil.Big
	GasLimit     hexutil.Uint64
	Amount       *hexutil.Big
	Payload      hexutil.Bytes
	V            *hexutil.Big
	R            *hexutil.Big
	S            *hexutil.Big
}

func NewTransaction(nonce uint64, to common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, SnO uint64, rR1 uint64, CmSpk uint64, CmRpk uint64, CmO uint64,
	CmS uint64, CmR uint64, EvR uint64, EvR0 uint64, EvR_ uint64, EvR_0 uint64, PI uint64, ID uint64, Sig string, CmV uint64, EpkV uint64) *Transaction {
	return newTransaction(nonce, &to, amount, gasLimit, gasPrice, data, SnO, rR1, CmSpk, CmRpk, CmO, CmS, CmR, EvR, EvR0, EvR_, EvR_0, PI, ID, Sig, CmV, EpkV)
}

func NewContractCreation(nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, SnO uint64, rR1 uint64, CmSpk uint64, CmRpk uint64, CmO uint64,
	CmS uint64, CmR uint64, EvR uint64, EvR0 uint64, EvR_ uint64, EvR_0 uint64, PI uint64, ID uint64, Sig string, CmV uint64, EpkV uint64) *Transaction {
	return newTransaction(nonce, nil, amount, gasLimit, gasPrice, data, SnO, rR1, CmSpk, CmRpk, CmO, CmS, CmR, EvR, EvR0, EvR_, EvR_0, PI, ID, Sig, CmV, EpkV)
}

func newTransaction(nonce uint64, to *common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, SnO uint64, rR1 uint64, CmSpk uint64, CmRpk uint64, CmO uint64,
	CmS uint64, CmR uint64, EvR uint64, EvR0 uint64, EvR_ uint64, EvR_0 uint64, PI uint64, ID uint64, Sig string, CmV uint64, EpkV uint64) *Transaction {
	if len(data) > 0 {
		data = common.CopyBytes(data)
	}
	d := txdata{
		AccountNonce: nonce,
		Recipient:    to,
		Payload:      data,
		Amount:       new(big.Int),
		GasLimit:     gasLimit,
		Price:        new(big.Int),
		V:            new(big.Int),
		R:            new(big.Int),
		S:            new(big.Int),
		SnO:          SnO,
		Rr1:          rR1,
		CmSpk:        CmSpk,
		CmRpk:        CmRpk,
		CmO:          CmO,
		CmS:          CmS,
		CmR:          CmR,
		EvR:          EvR,
		EvR0:         EvR0,
		EvR_:         EvR_,
		EvR_0:        EvR_0,
		PI:           PI,
		ID:           ID,
		Sig:          Sig,
		CmV:          CmV,
		EpkV:         EpkV,
	}
	if amount != nil {
		d.Amount.Set(amount)
	}
	if gasPrice != nil {
		d.Price.Set(gasPrice)
	}

	return &Transaction{data: d}
}

// ChainId returns which chain id this transaction was signed for (if at all)
func (tx *Transaction) ChainId() *big.Int {
	return deriveChainId(tx.data.V)
}

// Protected returns whether the transaction is protected from replay protection.
func (tx *Transaction) Protected() bool {
	return isProtectedV(tx.data.V)
}

func isProtectedV(V *big.Int) bool {
	if V.BitLen() <= 8 {
		v := V.Uint64()
		return v != 27 && v != 28
	}
	// anything not 27 or 28 is considered protected
	return true
}

// EncodeRLP implements rlp.Encoder
func (tx *Transaction) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &tx.data)
}

// DecodeRLP implements rlp.Decoder
func (tx *Transaction) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx.data)
	if err == nil {
		tx.size.Store(common.StorageSize(rlp.ListSize(size)))
	}

	return err
}

// MarshalJSON encodes the web3 RPC transaction format.
func (tx *Transaction) MarshalJSON() ([]byte, error) {
	hash := tx.Hash()
	data := tx.data
	data.Hash = &hash
	return data.MarshalJSON()
}

// UnmarshalJSON decodes the web3 RPC transaction format.
func (tx *Transaction) UnmarshalJSON(input []byte) error {
	var dec txdata
	if err := dec.UnmarshalJSON(input); err != nil {
		return err
	}

	withSignature := dec.V.Sign() != 0 || dec.R.Sign() != 0 || dec.S.Sign() != 0
	if withSignature {
		var V byte
		if isProtectedV(dec.V) {
			chainID := deriveChainId(dec.V).Uint64()
			V = byte(dec.V.Uint64() - 35 - 2*chainID)
		} else {
			V = byte(dec.V.Uint64() - 27)
		}
		if !crypto.ValidateSignatureValues(V, dec.R, dec.S, false) {
			return ErrInvalidSig
		}
	}

	*tx = Transaction{data: dec}
	return nil
}
func (tx *Transaction) Data() []byte       { return common.CopyBytes(tx.data.Payload) }
func (tx *Transaction) Gas() uint64        { return tx.data.GasLimit }
func (tx *Transaction) SnO() uint64        { return tx.data.SnO }
func (tx *Transaction) CmSpk() uint64      { return tx.data.CmSpk }
func (tx *Transaction) CmRpk() uint64      { return tx.data.CmRpk }
func (tx *Transaction) Rr1() uint64        { return tx.data.Rr1 }
func (tx *Transaction) CmO() uint64        { return tx.data.CmO }
func (tx *Transaction) CmS() uint64        { return tx.data.CmS }
func (tx *Transaction) CmR() uint64        { return tx.data.CmR }
func (tx *Transaction) EvR() uint64        { return tx.data.EvR }
func (tx *Transaction) EvR0() uint64       { return tx.data.EvR0 }
func (tx *Transaction) EvR_() uint64       { return tx.data.EvR_ }
func (tx *Transaction) EvR_0() uint64      { return tx.data.EvR_0 }
func (tx *Transaction) PI() uint64         { return tx.data.PI }
func (tx *Transaction) ID() uint64         { return tx.data.ID }
func (tx *Transaction) Sig() string        { return tx.data.Sig }
func (tx *Transaction) CmV() uint64        { return tx.data.CmV }
func (tx *Transaction) EpkV() uint64       { return tx.data.EpkV }
func (tx *Transaction) GasPrice() *big.Int { return new(big.Int).Set(tx.data.Price) }
func (tx *Transaction) Value() *big.Int    { return new(big.Int).Set(tx.data.Amount) }
func (tx *Transaction) Nonce() uint64      { return tx.data.AccountNonce }
func (tx *Transaction) CheckNonce() bool   { return true }

// To returns the recipient address of the transaction.
// It returns nil if the transaction is a contract creation.
func (tx *Transaction) To() *common.Address {
	if tx.data.Recipient == nil {
		return nil
	}
	to := *tx.data.Recipient
	return &to
}

// Hash hashes the RLP encoding of tx.
// It uniquely identifies the transaction.
func (tx *Transaction) Hash() common.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := rlpHash(tx)
	tx.hash.Store(v)
	return v
}

// Size returns the true RLP encoded storage size of the transaction, either by
// encoding and returning it, or returning a previsouly cached value.
func (tx *Transaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, &tx.data)
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// AsMessage returns the transaction as a core.Message.
//
// AsMessage requires a signer to derive the sender.
//
// XXX Rename message to something less arbitrary?
func (tx *Transaction) AsMessage(s Signer) (Message, error) {
	msg := Message{
		nonce:      tx.data.AccountNonce,
		gasLimit:   tx.data.GasLimit,
		gasPrice:   new(big.Int).Set(tx.data.Price),
		to:         tx.data.Recipient,
		amount:     tx.data.Amount,
		data:       tx.data.Payload,
		checkNonce: true,
	}

	var err error
	msg.from, err = Sender(s, tx)
	return msg, err
}

// WithSignature returns a new transaction with the given signature.
// This signature needs to be in the [R || S || V] format where V is 0 or 1.
func (tx *Transaction) WithSignature(signer Signer, sig []byte) (*Transaction, error) {
	r, s, v, err := signer.SignatureValues(tx, sig)
	if err != nil {
		return nil, err
	}
	cpy := &Transaction{data: tx.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v
	return cpy, nil
}

// Cost returns amount + gasprice * gaslimit.
func (tx *Transaction) Cost() *big.Int {
	total := new(big.Int).Mul(tx.data.Price, new(big.Int).SetUint64(tx.data.GasLimit))
	total.Add(total, tx.data.Amount)
	return total
}

// RawSignatureValues returns the V, R, S signature values of the transaction.
// The return values should not be modified by the caller.
func (tx *Transaction) RawSignatureValues() (v, r, s *big.Int) {
	return tx.data.V, tx.data.R, tx.data.S
}

// Transactions is a Transaction slice type for basic sorting.
type Transactions []*Transaction

// Len returns the length of s.
func (s Transactions) Len() int { return len(s) }

// Swap swaps the i'th and the j'th element in s.
func (s Transactions) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (s Transactions) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(s[i])
	return enc
}

// TxDifference returns a new set which is the difference between a and b.
func TxDifference(a, b Transactions) Transactions {
	keep := make(Transactions, 0, len(a))

	remove := make(map[common.Hash]struct{})
	for _, tx := range b {
		remove[tx.Hash()] = struct{}{}
	}

	for _, tx := range a {
		if _, ok := remove[tx.Hash()]; !ok {
			keep = append(keep, tx)
		}
	}

	return keep
}

// TxByNonce implements the sort interface to allow sorting a list of transactions
// by their nonces. This is usually only useful for sorting transactions from a
// single account, otherwise a nonce comparison doesn't make much sense.
type TxByNonce Transactions

func (s TxByNonce) Len() int           { return len(s) }
func (s TxByNonce) Less(i, j int) bool { return s[i].data.AccountNonce < s[j].data.AccountNonce }
func (s TxByNonce) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// TxByPrice implements both the sort and the heap interface, making it useful
// for all at once sorting as well as individually adding and removing elements.
type TxByPrice Transactions

func (s TxByPrice) Len() int           { return len(s) }
func (s TxByPrice) Less(i, j int) bool { return s[i].data.Price.Cmp(s[j].data.Price) > 0 }
func (s TxByPrice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (s *TxByPrice) Push(x interface{}) {
	*s = append(*s, x.(*Transaction))
}

func (s *TxByPrice) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

// TransactionsByPriceAndNonce represents a set of transactions that can return
// transactions in a profit-maximizing sorted order, while supporting removing
// entire batches of transactions for non-executable accounts.
type TransactionsByPriceAndNonce struct {
	txs    map[common.Address]Transactions // Per account nonce-sorted list of transactions
	heads  TxByPrice                       // Next transaction for each unique account (price heap)
	signer Signer                          // Signer for the set of transactions
}

// NewTransactionsByPriceAndNonce creates a transaction set that can retrieve
// price sorted transactions in a nonce-honouring way.
//
// Note, the input map is reowned so the caller should not interact any more with
// if after providing it to the constructor.
func NewTransactionsByPriceAndNonce(signer Signer, txs map[common.Address]Transactions) *TransactionsByPriceAndNonce {
	// Initialize a price based heap with the head transactions
	heads := make(TxByPrice, 0, len(txs))
	for from, accTxs := range txs {
		heads = append(heads, accTxs[0])
		// Ensure the sender address is from the signer
		acc, _ := Sender(signer, accTxs[0])
		txs[acc] = accTxs[1:]
		if from != acc {
			delete(txs, from)
		}
	}
	heap.Init(&heads)

	// Assemble and return the transaction set
	return &TransactionsByPriceAndNonce{
		txs:    txs,
		heads:  heads,
		signer: signer,
	}
}

// Peek returns the next transaction by price.
func (t *TransactionsByPriceAndNonce) Peek() *Transaction {
	if len(t.heads) == 0 {
		return nil
	}
	return t.heads[0]
}

// Shift replaces the current best head with the next one from the same account.
func (t *TransactionsByPriceAndNonce) Shift() {
	acc, _ := Sender(t.signer, t.heads[0])
	if txs, ok := t.txs[acc]; ok && len(txs) > 0 {
		t.heads[0], t.txs[acc] = txs[0], txs[1:]
		heap.Fix(&t.heads, 0)
	} else {
		heap.Pop(&t.heads)
	}
}

// Pop removes the best transaction, *not* replacing it with the next one from
// the same account. This should be used when a transaction cannot be executed
// and hence all subsequent ones should be discarded from the same account.
func (t *TransactionsByPriceAndNonce) Pop() {
	heap.Pop(&t.heads)
}

// Message is a fully derived transaction and implements core.Message
//
// NOTE: In a future PR this will be removed.
type Message struct {
	to         *common.Address
	from       common.Address
	nonce      uint64
	amount     *big.Int
	gasLimit   uint64
	gasPrice   *big.Int
	data       []byte
	checkNonce bool
}

func NewMessage(from common.Address, to *common.Address, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, checkNonce bool) Message {
	return Message{
		from:       from,
		to:         to,
		nonce:      nonce,
		amount:     amount,
		gasLimit:   gasLimit,
		gasPrice:   gasPrice,
		data:       data,
		checkNonce: checkNonce,
	}
}

func (m Message) From() common.Address { return m.from }
func (m Message) To() *common.Address  { return m.to }
func (m Message) GasPrice() *big.Int   { return m.gasPrice }
func (m Message) Value() *big.Int      { return m.amount }
func (m Message) Gas() uint64          { return m.gasLimit }
func (m Message) Nonce() uint64        { return m.nonce }
func (m Message) Data() []byte         { return m.data }
func (m Message) CheckNonce() bool     { return m.checkNonce }
