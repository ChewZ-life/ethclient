package message

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
)

type Message struct {
	Req    *Request
	Resp   *Response // not nil if inflight
	Status MessageStatus
}

func (m *Message) Id() common.Hash {
	return m.Req.id
}

type Request struct {
	id                    common.Hash
	From                  common.Address  // the sender of the 'transaction'
	To                    *common.Address // the destination contract (nil for contract creation)
	Value                 *big.Int        // amount of wei sent along with the call
	Gas                   uint64          // if 0, the call executes with near-infinite gas
	GasOnEstimationFailed *uint64         // how much gas you wanna provide when the msg estimation failed. As much as possible, so you can debug on-chain
	GasPrice              *big.Int        // wei <-> gas exchange ratio
	Data                  []byte          // input data, usually an ABI-encoded contract method invocation

	AccessList types.AccessList // EIP-2930 access list.
}

type MessageStatus uint8

const (
	MessageStatusSubmitted MessageStatus = iota
	MessageStatusScheduled
	MessageStatusQueued
	MessageStatusNonceAssigned
	MessageStatusInflight // Broadcasted but not on chain
	MessageStatusOnChain
	MessageStatusFinalized
	// it was broadcasted but not included on-chain until timeout, so the nonce was released
	MessageStatusNonceReleased
	MessageStatusExpired
)

type Response struct {
	Id  common.Hash
	Tx  *types.Transaction
	Err error
}

func AssignMessageId(msg *Request) *Request {
	uid, _ := uuid.NewUUID()
	uidBytes, _ := uid.MarshalBinary()
	msg.id = crypto.Keccak256Hash(uidBytes)
	return msg
}

func AssignMessageIdWithNonce(msg *Request, nonce int64) *Request {
	msg.id = *GenerateMessageIdByNonce(nonce)
	return msg
}

func GenerateMessageIdByNonce(nonce int64) *common.Hash {
	hash := crypto.Keccak256Hash(big.NewInt(nonce).Bytes())
	return &hash
}

func (m *Request) Id() common.Hash {
	return m.id
}

func (q *Request) SetId(id common.Hash) {
	q.id = id
}

func (q *Request) SetIdWithNonce(nonce int64) {
	q.id = *GenerateMessageIdByNonce(nonce)
}
