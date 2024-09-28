package client_test

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ChewZ-life/ethclient"
	"github.com/ChewZ-life/ethclient/message"
	"github.com/ChewZ-life/ethclient/tests/helper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestScheduleMsg(t *testing.T) {
	client := helper.SetUpClient(t)

	testScheduleMsg(t, client)
}

func testScheduleMsg(t *testing.T, client *ethclient.Client) {
	buffer := 10
	go func() {
		for i := 0; i < 2*buffer; i++ {
			to := common.HexToAddress("0x06514D014e997bcd4A9381bF0C4Dc21bD32718D4")
			req := &message.Request{
				From: helper.Addr,
				To:   &to,
			}

			message.AssignMessageId(req)

			t.Logf("ScheduleMsg#%v", i)
			client.ScheduleMsg(*req)
			t.Log("Write MSG to channel")
		}

		time.Sleep(5 * time.Second)
		t.Log("Close client")
		client.Close()
	}()

	for resp := range client.ScheduleMsgResponse() {
		tx := resp.Tx
		err := resp.Err
		var js []byte
		if tx != nil {
			js, _ = tx.MarshalJSON()
		}

		log.Info("Get Transaction", "tx", string(js), "err", err)
		assert.Equal(t, nil, err)
	}
	t.Log("Exit")
}
