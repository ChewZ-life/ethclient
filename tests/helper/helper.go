package helper

import (
	"context"
	"os"
	"testing"

	"github.com/ChewZ-life/ethclient"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

var (
	PrivateKey, _ = crypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	Addr          = crypto.PubkeyToAddress(PrivateKey.PublicKey)
)

func SetUpClient(t *testing.T) *ethclient.Client {
	handler := log.NewTerminalHandler(os.Stdout, true)
	logger := log.NewLogger(handler)
	log.SetDefault(logger)

	client, err := ethclient.Dial("http://localhost:8545")
	if err != nil {
		t.Fatal(err)
	}

	err = client.RegisterPrivateKey(context.Background(), PrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	return client
}
