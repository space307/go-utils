package btc_rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/pkg/errors"


)

type RpcClient struct {
	*rpcclient.Client
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

	return &RpcClient{ rpcClient}, nil
}

func (rc *RpcClient) GetNewAddress() (btcutil.Address, error) {
	addr, err := rc.Client.GetNewAddress("")
	if err != nil {
		return nil, err
	}
	return addr, nil
}

func (rc *RpcClient) CreateRawTransaction(address string, amount int64) (string, error) {
	inputs := []byte("[]")
	outputs, err := json.Marshal(map[string]float64{ address: float64(amount)/1e8 })
	if err != nil {
		return "", err
	}
	lockTime := []byte("0")
	replaceable := []byte("true")


	resp, err := rc.Client.RawRequest("createrawtransaction", []json.RawMessage{ inputs, outputs, lockTime, replaceable })
	if err != nil {
		return "", errors.WithStack(err)
	}

	var hexString string

	err = json.Unmarshal(resp, &hexString)

	if err != nil {
		return "", errors.WithStack(err)
	}

	return hexString, nil
}

func (rc *RpcClient) SendRawTransaction(tx *wire.MsgTx) (*chainhash.Hash, error) {
	return rc.Client.SendRawTransaction(tx, false)
}

func (rc *RpcClient) SignRawTransactionWithWallet(tx *wire.MsgTx) (*wire.MsgTx, bool, error) {
	txByteBuffer := bytes.Buffer{}
	err := tx.Serialize(&txByteBuffer)
	if err != nil {
		return nil, false, errors.WithStack(err)
	}

	jsonData, err := json.Marshal(hex.EncodeToString(txByteBuffer.Bytes()))
	if err != nil {
		return nil, false, errors.WithStack(err)
	}

	resultJson, err := rc.Client.RawRequest("signrawtransactionwithwallet", []json.RawMessage{jsonData})
	if err != nil {
		return nil, false, errors.WithStack(err)
	}

	var result SignRawTransactionResult
	err = json.Unmarshal(resultJson, &result)
	if err != nil {
		return nil, false, errors.WithStack(err)
	}

	txBytes, err := hex.DecodeString(result.Hex)
	if err != nil {
		return nil, false, errors.WithStack(err)
	}

	newTx, err := btcutil.NewTxFromBytes(txBytes)
	if err != nil {
		return nil, false, errors.WithStack(err)
	}

	return newTx.MsgTx(), result.Complete, nil
}


/**

KeyPoolRefill() error
WalletLock() error
WalletPassphrase(passphrase string, timeoutSecs int64) error

	ListSinceBlock(blockHash *chainhash.Hash) (*btcjson.ListSinceBlockResult, error)
	GetTransaction(txHash *chainhash.Hash) (*btcjson.GetTransactionResult, error)
	ListUnspentMin(minConf int) ([]btcjson.ListUnspentResult, error)
	WalletPassphrase(passphrase string, timeoutSecs int64) error
	SendManyMinConf(fromAccount string, amounts map[btcutil.Address]btcutil.Amount, minConfirms int) (*chainhash.Hash, error)
	WalletLock() error
	CreateRawTransaction(inputs []btcjson.TransactionInput,
		amounts map[btcutil.Address]btcutil.Amount, lockTime *int64) (*wire.MsgTx, error)
	SignRawTransaction(tx *wire.MsgTx) (*wire.MsgTx, bool, error)
	SendRawTransaction(tx *wire.MsgTx, allowHighFees bool) (*chainhash.Hash, error)
	GetRawChangeAddress(string) (btcutil.Address, error)
 */