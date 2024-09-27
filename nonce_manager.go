package ethclient

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type NonceManager interface {
	PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
	PeekNonce(account common.Address) uint64
	ResetNonce(ctx context.Context, account common.Address) error
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
}

type NonceAtFunc func(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error)

type SimpleNonceManager struct {
	nonceMap map[common.Address]uint64
	lock     sync.Mutex
	client   *ethclient.Client
	NonceAt  NonceAtFunc
}

var snm *SimpleNonceManager
var snmOnce sync.Once

func GetSimpleNonceManager(client *ethclient.Client) (*SimpleNonceManager, error) {
	snmOnce.Do(func() {
		snm = &SimpleNonceManager{
			nonceMap: make(map[common.Address]uint64),
			client:   client,
		}

		snm.SetNonceAt(client.NonceAt)
	})

	return snm, nil
}

func NewSimpleNonceManager(client *ethclient.Client) (*SimpleNonceManager, error) {
	return &SimpleNonceManager{
		nonceMap: make(map[common.Address]uint64),
		client:   client,
	}, nil
}

func (nm *SimpleNonceManager) PendingNonceAt(ctx context.Context, account common.Address) (uint64, error) {
	nm.lock.Lock()
	defer nm.lock.Unlock()

	var (
		nonce uint64
		err   error
	)

	fmt.Printf("PendingNonceAt starts account=%v\n", account.Hex())

	var nonceInLatest uint64
	if nm.NonceAt == nil {
		nonceInLatest, err = nm.client.NonceAt(ctx, account, nil)
		if err != nil {
			return 0, err
		}
	} else {
		nonceInLatest, err = nm.NonceAt(ctx, account, nil)
		if err != nil {
			return 0, err
		}
	}

	nonce, ok := nm.nonceMap[account]
	if !ok || nonceInLatest > nonce {
		fmt.Printf("PendingNonceAt replace nonce account=%v, nonceInLatest=%v,nonce=%v\n", account.Hex(), nonceInLatest, nonce)
		nonce = nonceInLatest
	}

	nm.nonceMap[account] = nonce + 1

	fmt.Printf("PendingNonceAt states account=%v, nonceInLatest=%v,nonce=%v\n", account.Hex(), nonceInLatest, nonce)

	return nonce, nil
}

func (nm *SimpleNonceManager) SuggestGasPrice(ctx context.Context) (gasPrice *big.Int, err error) {
	gasPrice, err = nm.client.SuggestGasPrice(ctx)
	if err != nil {
		return
	}

	// Multiplier 1.5
	gasPrice.Mul(gasPrice, big.NewInt(1500))
	gasPrice.Div(gasPrice, big.NewInt(1000))

	return
}

func (nm *SimpleNonceManager) PeekNonce(account common.Address) uint64 {
	nm.lock.Lock()
	defer nm.lock.Unlock()

	nonce := nm.nonceMap[account]
	return nonce
}

func (nm *SimpleNonceManager) ResetNonce(ctx context.Context, account common.Address) (err error) {
	nm.lock.Lock()
	defer nm.lock.Unlock()

	var nonceInLatest uint64
	if nm.NonceAt == nil {
		nonceInLatest, err = nm.client.NonceAt(ctx, account, nil)
		if err != nil {
			return err
		}
	} else {
		nonceInLatest, err = nm.NonceAt(ctx, account, nil)
		if err != nil {
			return err
		}
	}

	nm.nonceMap[account] = nonceInLatest

	return nil
}

func (nm *SimpleNonceManager) SetNonceAt(nonceAt NonceAtFunc) {
	nm.NonceAt = nonceAt
}
