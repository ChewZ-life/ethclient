package ethclient

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type Client struct {
	*ethclient.Client
	rpcClient *rpc.Client
	NonceManager
}

func Dial(rawurl string) (*Client, error) {
	rpcClient, err := rpc.Dial(rawurl)
	if err != nil {
		return nil, err
	}

	c := ethclient.NewClient(rpcClient)

	nm, err := GetSimpleNonceManager(c)
	if err != nil {
		return nil, err
	}

	return &Client{
		Client:       c,
		rpcClient:    rpcClient,
		NonceManager: nm,
	}, nil
}

func (c *Client) Close() {
	c.Client.Close()
}

// RawClient returns underlying ethclient
func (c *Client) RawClient() *ethclient.Client {
	return c.Client
}

func (c *Client) PendingNonceAt(ctx context.Context, account common.Address) (uint64, error) {
	return c.NonceManager.PendingNonceAt(ctx, account)
}

func (c *Client) SuggestGasPrice(ctx context.Context) (gasPrice *big.Int, err error) {
	return c.NonceManager.SuggestGasPrice(ctx)
}

func (c *Client) WaitTxReceipt(txHash common.Hash, confirmations uint64, timeout time.Duration) (*types.Receipt, bool) {
	startTime := time.Now()
	for {
		currTime := time.Now()
		elapsedTime := currTime.Sub(startTime)
		if elapsedTime >= timeout {
			return nil, false
		}

		receipt, err := c.Client.TransactionReceipt(context.Background(), txHash)
		if err != nil {
			continue
		}

		block, err := c.Client.BlockNumber(context.Background())
		if err != nil {
			continue
		}

		if block >= receipt.BlockNumber.Uint64()+confirmations {
			return receipt, true
		}
	}
}