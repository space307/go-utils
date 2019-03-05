package btc_rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

const addressType = "p2sh-segwit"

type RpcClient struct {
	rpcClient *rpcclient.Client
}

type SignRawTransactionResult struct {
	Hex      string        `json:"hex"`
	Complete bool          `json:"complete"`
	Errors   []interface{} `json:"errors"`
}

func NewRpcClient(cfg *rpcclient.ConnConfig) (*RpcClient, error) {
	rpcClient, err := rpcclient.New(cfg, nil)
	if err != nil {
		return nil, err
	}

	return &RpcClient{rpcClient: rpcClient}, nil
}

func (rc *RpcClient) Shutdown() {
	rc.rpcClient.Shutdown()
}

func (rc *RpcClient) GetNewAddress() (btcutil.Address, error) {
	return rc.rpcClient.GetNewAddress("")
}

func (rc *RpcClient) ListUnspent() ([]btcjson.ListUnspentResult, error) {
	return rc.rpcClient.ListUnspent()
}

// from https://github.com/RHavar/bustapay/blob/master/rpc-client/rpc-client.go
func (rc *RpcClient) SignRawTransactionWithWallet(tx *wire.MsgTx) (*wire.MsgTx, bool, error) {
	txByteBuffer := bytes.Buffer{}
	err := tx.Serialize(&txByteBuffer)
	if err != nil {
		return nil, false, err
	}

	jsonData, err := json.Marshal(hex.EncodeToString(txByteBuffer.Bytes()))
	if err != nil {
		return nil, false, err
	}

	resultJson, err := rc.rpcClient.RawRequest("signrawtransactionwithwallet", []json.RawMessage{jsonData})
	if err != nil {
		return nil, false, err
	}

	var result SignRawTransactionResult
	err = json.Unmarshal(resultJson, &result)
	if err != nil {
		return nil, false, err
	}

	txBytes, err := hex.DecodeString(result.Hex)
	if err != nil {
		return nil, false, err
	}

	newTx, err := btcutil.NewTxFromBytes(txBytes)
	if err != nil {
		return nil, false, err
	}

	return newTx.MsgTx(), result.Complete, nil
}

func (rc *RpcClient) KeyPoolRefill() error {
	return rc.rpcClient.KeyPoolRefill()
}

func (rc *RpcClient) WalletLock() error {
	return rc.rpcClient.WalletLock()
}

func (rc *RpcClient) WalletPassphrase(passphrase string, timeoutSecs int64) error {
	return rc.rpcClient.WalletPassphrase(passphrase, timeoutSecs)
}

func (rc *RpcClient) ListSinceBlock(blockHash *chainhash.Hash) (*btcjson.ListSinceBlockResult, error) {
	return rc.rpcClient.ListSinceBlock(blockHash)
}

func (rc *RpcClient) GetClient() *rpcclient.Client {
	return rc.rpcClient
}

func (rc *RpcClient) GetTransaction(txHash *chainhash.Hash) (*btcjson.GetTransactionResult, error) {
	return rc.rpcClient.GetTransaction(txHash)
}

func (rc *RpcClient) SendManyMinConf(fromAccount string, amounts map[btcutil.Address]btcutil.Amount, minConfirms int) (*chainhash.Hash, error) {
	return rc.rpcClient.SendManyMinConf(fromAccount, amounts, minConfirms)
}

func (rc *RpcClient) CreateRawTransaction(inputs []btcjson.TransactionInput, amounts map[btcutil.Address]btcutil.Amount, lockTime *int64) (*wire.MsgTx, error) {
	return rc.rpcClient.CreateRawTransaction(inputs, amounts, lockTime)
}

func (rc *RpcClient) SignRawTransaction(tx *wire.MsgTx) (*wire.MsgTx, bool, error) {
	return rc.rpcClient.SignRawTransaction(tx)
}

func (rc *RpcClient) SendRawTransaction(tx *wire.MsgTx, allowHighFees bool) (*chainhash.Hash, error) {
	return rc.rpcClient.SendRawTransaction(tx, allowHighFees)
}

func (rc *RpcClient) GetRawChangeAddress() (btcutil.Address, error) {
	return rc.rpcClient.GetRawChangeAddress(addressType)
}
