package btc_rpc

import (
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

const (
	timeSyncAndUpdate = 2 * time.Second // time for generate blocks and update tx
)

func TestNewRpcClient(t *testing.T) {
	cfg := &rpcclient.ConnConfig{}
	client, err := NewRpcClient(cfg)

	assert.NotNil(t, err)
	assert.Nil(t, client)

	cfg.Pass = "123456"
	cfg.User = "pilotuser"
	cfg.Host = "localhost:8332"
	cfg.DisableTLS = true
	cfg.HTTPPostMode = true
	client, err = NewRpcClient(cfg)
	assert.NotNil(t, client)
	assert.Nil(t, err)

	err = client.rpcClient.Ping()
	assert.Nilf(t, err, "error: %s", err)
}

func TestSend(t *testing.T) {
	cfg := &rpcclient.ConnConfig{}
	cfg.Pass = "123456"
	cfg.User = "pilotuser"
	cfg.Host = "localhost:8332"
	cfg.DisableTLS = true
	cfg.HTTPPostMode = true

	client, err := NewRpcClient(cfg)
	err = client.rpcClient.Ping()
	assert.NoError(t, err)

	cfg.Host = "localhost:8334"
	client2, err := NewRpcClient(cfg)
	err = client2.rpcClient.Ping()
	assert.NoError(t, err)

	doBTCNoise(client, t, client2)
	time.Sleep(timeSyncAndUpdate)

	addrb, err := client.GetNewAddress()
	assert.NoError(t, err)

	amount := decimal.NewFromFloat(1)
	fee := decimal.NewFromFloat(0.0001)

	// send
	err = client.WalletPassphrase("passphrase", 2)
	assert.NoError(t, err)

	// make tx
	returnAddr, err := client.GetRawChangeAddress()
	assert.NoError(t, err)

	listUnspentResult, err := client.rpcClient.ListUnspentMin(6)
	assert.NoError(t, err)

	inputs, foundAmount := findInputs(f64(amount.Float64())+f64(fee.Float64()), listUnspentResult)
	assert.NoError(t, err)

	toAmount, err := btcutil.NewAmount(f64(amount.Float64()))
	assert.NoError(t, err)

	amounts := map[btcutil.Address]btcutil.Amount{
		addrb: toAmount,
	}

	if change := foundAmount - f64(amount.Float64()) - f64(fee.Float64()); change > 0 {
		changeAmount, err := btcutil.NewAmount(change)
		assert.NoError(t, err)
		amounts[returnAddr] = changeAmount
	}

	tx, err := client.CreateRawTransaction(inputs, amounts, nil)
	assert.NoError(t, err)

	newTx, _, err := client.SignRawTransactionWithWallet(tx)
	assert.NoError(t, err)

	hash, err := client.SendRawTransaction(newTx, false)
	assert.NoError(t, err)

	txHash := hash.String()

	err = client.WalletLock()
	assert.NoError(t, err)

	assert.NoError(t, err)
	assert.NotEmpty(t, txHash)

	_, err = client.rpcClient.GetRawChangeAddress("")
	assert.NotNil(t, err)

	_, err = client.GetRawChangeAddress()
	assert.NoError(t, err)

	doBTCNoise(client, t, client2)
	time.Sleep(timeSyncAndUpdate)

	err = client.WalletPassphrase("passphrase", 2)
	assert.NoError(t, err)
	err = client.KeyPoolRefill()
	assert.NoError(t, err)

	rs1, err := client.GetTransaction(hash)
	assert.NoError(t, err)
	assert.NotNil(t, rs1)
	assert.Equal(t, rs1.Confirmations, int64(122))
}

func doBTCNoise(client *RpcClient, t *testing.T, client2 *RpcClient) {
	client.rpcClient.Generate(116)
	for i := 1; i <= 16; i++ {
		// create 16 trx with fee = 1 and mining (generate) 5 blocks for calculate feeRate
		// for details: see estimateSmartFee method
		// https://github.com/bitcoin/bitcoin/blob/67447ba06057b8e83f962c82491d2fe6c5211f50/src/policy/fees.cpp
		addr, err := client.GetNewAddress()
		assert.NoError(t, err)

		err = client2.WalletPassphrase(`passphrase`, 10)
		assert.NoError(t, err)
		_, err = client2.rpcClient.SendToAddress(addr, btcutil.Amount(100000000))
		assert.NoError(t, err)
		client2.WalletLock()
	}
	client2.rpcClient.Generate(6)
}

func f64(f float64, _ bool) float64 {
	return f
}

func findInputs(amount float64, unspents []btcjson.ListUnspentResult) (inputs []btcjson.TransactionInput, total float64) {
	for _, i := range unspents {
		inputs = append(inputs, btcjson.TransactionInput{
			Txid: i.TxID, Vout: i.Vout,
		})
		total += i.Amount
		if total >= amount {
			break
		}
	}
	return
}
