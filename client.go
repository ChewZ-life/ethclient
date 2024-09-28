package ethclient

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ChewZ-life/ethclient/common/consts"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ChewZ-life/ethclient/message"
	"github.com/ChewZ-life/ethclient/nonce"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type Client struct {
	*ethclient.Client
	rpcClient *rpc.Client
	nonce.Manager
	signers []bind.SignerFn // Method to use for signing the transaction (mandatory)

	reqChannel  chan message.Request
	respChannel chan message.Response
}

func Dial(rawurl string) (*Client, error) {
	rpcClient, err := rpc.Dial(rawurl)
	if err != nil {
		return nil, err
	}

	c := ethclient.NewClient(rpcClient)

	nm, err := nonce.NewSimpleManager(c, nonce.NewMemoryStorage())
	if err != nil {
		return nil, err
	}

	cli, err := NewClient(rpcClient, nm)
	if err != nil {
		return nil, err
	}

	return cli, nil
}

func NewClient(
	c *rpc.Client,
	nonceManager nonce.Manager,
) (*Client, error) {
	ethc := ethclient.NewClient(c)

	cli := &Client{
		Client:      ethc,
		rpcClient:   c,
		reqChannel:  make(chan message.Request, consts.DefaultMsgBuffer),
		respChannel: make(chan message.Response, consts.DefaultMsgBuffer),
		Manager:     nonceManager,
	}

	go cli.sendMsgTask(context.Background())

	return cli, nil
}

func (c *Client) Close() {
	close(c.reqChannel)

	c.Client.Close()
}

// RawClient returns underlying ethclient
func (c *Client) RawClient() *ethclient.Client {
	return c.Client
}

func (c *Client) SetNonceManager(nm nonce.Manager) {
	c.Manager = nm
}

func (c *Client) GetSigner() bind.SignerFn {
	// combine all signerFns
	return func(a common.Address, t *types.Transaction) (tx *types.Transaction, err error) {
		if len(c.signers) == 0 {
			return nil, fmt.Errorf("no signerFn registered")
		}

		for i, fn := range c.signers {
			tx, err = fn(a, t)
			log.Debug("try to call signerFn", "index", i, "err", err, "account", a)

			if err != nil {
				continue
			}

			return tx, nil
		}

		return nil, bind.ErrNotAuthorized
	}
}

func (c *Client) RegisterSigner(signerFn bind.SignerFn) {
	log.Info("register signerFn for signing...")
	c.signers = append(c.signers, signerFn)
}

// Registers the private key used for signing txs.
func (c *Client) RegisterPrivateKey(ctx context.Context, key *ecdsa.PrivateKey) error {
	chainID, err := c.ChainID(ctx)
	if err != nil {
		return err
	}
	keyAddr := crypto.PubkeyToAddress(key.PublicKey)
	if chainID == nil {
		return bind.ErrNoChainID
	}
	signer := types.LatestSignerForChainID(chainID)
	signerFn := func(address common.Address, tx *types.Transaction) (*types.Transaction, error) {
		if address != keyAddr {
			return nil, bind.ErrNotAuthorized
		}
		signature, err := crypto.Sign(signer.Hash(tx).Bytes(), key)
		if err != nil {
			return nil, err
		}
		return tx.WithSignature(signer, signature)
	}

	c.RegisterSigner(signerFn)

	return nil
}

func (c *Client) sendMsgTask(ctx context.Context) {
	// Pipepline: reqChannel => scheduler => broadcaster

	go c.schedule(c.respChannel)
}

func (c *Client) ScheduleMsg(req message.Request) {
	c.reqChannel <- req
}

func (c *Client) ScheduleMsgResponse() <-chan message.Response {
	return c.respChannel
}

func (c *Client) schedule(msgResChan chan<- message.Response) {
	for req := range c.reqChannel {
		log.Debug("start scheduling msg...", "msgId", req.Id())
		if req.Id() == common.BytesToHash([]byte{}) {
			msgResChan <- message.Response{
				Id:  req.Id(),
				Err: fmt.Errorf("no msgId provided"),
			}
			continue
		}

		log.Info("broadcast msg", "msg", req)
		tx, err := c.sendMsg(context.Background(), req)
		msgResChan <- message.Response{Id: req.Id(), Tx: tx, Err: err}
	}

	log.Debug("close scheduler...")
	close(c.respChannel)
}

func (c *Client) sendMsg(ctx context.Context, msg message.Request) (signedTx *types.Transaction, err error) {
	log.Debug("broadcast msg", "msg", msg)
	tx, err := c.NewTransaction(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("NewTransaction err: %v", err)
	}

	// chainID, err := c.Client.ChainID(ctx)
	// if err != nil {
	// 	return nil, fmt.Errorf("get Chain ID err: %v", err)
	// }

	// signedTx, err = types.SignTx(tx, types.NewEIP2930Signer(chainID), msg.PrivateKey)
	// if err != nil {
	// 	return nil, fmt.Errorf("SignTx err: %v", err)
	// }

	signerFn := c.GetSigner()
	signedTx, err = signerFn(msg.From, tx)
	if err != nil {
		return nil, err
	}

	err = c.Client.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("SendTransaction err: %v", err)
	}

	resp := message.Response{
		Id:  msg.Id(),
		Tx:  signedTx,
		Err: err,
	}

	c.respChannel <- resp

	log.Debug("Send Message successfully", "txHash", signedTx.Hash().Hex(), "from", msg.From.Hex(),
		"to", msg.To.Hex(), "value", msg.Value)

	return signedTx, nil
}

func (c *Client) NewTransaction(ctx context.Context, msg message.Request) (*types.Transaction, error) {
	if msg.To == nil {
		to := common.HexToAddress("0x0")
		msg.To = &to
	}

	if msg.Gas == 0 {
		ethMesg := ethereum.CallMsg{
			From:       msg.From,
			To:         msg.To,
			Gas:        msg.Gas,
			GasPrice:   msg.GasPrice,
			Value:      msg.Value,
			Data:       msg.Data,
			AccessList: msg.AccessList,
		}

		gas, err := c.EstimateGas(ctx, ethMesg)
		if err != nil {
			if msg.GasOnEstimationFailed == nil {
				return nil, err
			}

			msg.Gas = *msg.GasOnEstimationFailed
		} else {
			// Multiplier 1.5
			msg.Gas = gas * 1500 / 1000
		}
	}

	if msg.GasPrice == nil || msg.GasPrice.Uint64() == 0 {
		var err error
		msg.GasPrice, err = c.SuggestGasPrice(ctx)
		if err != nil {
			return nil, err
		}
	}

	nonce, err := c.PendingNonceAt(ctx, msg.From)
	if err != nil {
		return nil, err
	}

	tx := types.NewTransaction(nonce, *msg.To, msg.Value, msg.Gas, msg.GasPrice, msg.Data)

	return tx, nil
}

func (c *Client) PendingNonceAt(ctx context.Context, account common.Address) (uint64, error) {
	return c.Manager.PendingNonceAt(ctx, account)
}

func (c *Client) SuggestGasPrice(ctx context.Context) (gasPrice *big.Int, err error) {
	return c.Manager.SuggestGasPrice(ctx)
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
