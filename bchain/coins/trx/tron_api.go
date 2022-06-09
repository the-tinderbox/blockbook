package trx

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/imroc/req"
	"github.com/tidwall/gjson"
	"log"
	"math/big"
	"net/http"
	"strconv"
)

// A Client is a Tron RPC client. It performs RPCs over HTTP using JSON
// request and responses. A Client must be configured with a secret token
// to authenticate with other Cores on the network.
type Client struct {
	config *ClientConfig
	client *req.Req
}

// NewClient create new client to connect
func NewClient(config *ClientConfig) *Client {
	return &Client{
		config: config,
		client: req.New(),
	}
}

type ClientConfig struct {
	TronNodeRPC     string
	SolidityNodeRPC string
	TestNet         bool
}

func NewConfig() *ClientConfig {
	return &ClientConfig{}
}

func (c *Client) call(url string, param interface{}) (*gjson.Result, error) {
	if c == nil || c.client == nil {
		return nil, errors.New("API url is not setup. ")
	}

	authHeader := req.Header{"Accept": "application/json"}

	r, err := req.Post(url, req.BodyJSON(&param), authHeader)
	if err != nil {
		return nil, err
	}

	if r.Response().StatusCode != http.StatusOK {
		message := gjson.ParseBytes(r.Bytes()).String()
		message = fmt.Sprintf("[%s]%s", r.Response().Status, message)

		return nil, errors.New(message)
	}

	res := gjson.ParseBytes(r.Bytes())
	return &res, nil
}

func (c *Client) TronCall(path string, param interface{}) (*gjson.Result, error) {
	url := c.config.TronNodeRPC + path

	return c.call(url, param)
}

func (c *Client) SolidityCall(path string, param interface{}) (*gjson.Result, error) {
	url := c.config.SolidityNodeRPC + path

	return c.call(url, param)
}

func (c *Client) GetNodeInfo() (*NodeInfo, error) {
	r, err := c.TronCall("/wallet/getnodeinfo", nil)
	if err != nil {
		return nil, err
	}

	return NewNodeInfo(r), nil
}

func (c *Client) GetNowBlock() (block *Block, err error) {
	r, err := c.TronCall("/wallet/getnowblock", nil)
	if err != nil {
		return nil, err
	}

	block, err = NewBlock(r, c.config.TestNet)

	if err != nil {
		return nil, err
	}
	if block.GetBlockHashID() == "" || block.GetHeight() < 0 {
		return nil, errors.New("GetNowBlock failed: No found <block>")
	}

	return block, nil
}

func (c *Client) GetBlockByNum(num uint64) (block *Block, error error) {
	r, err := c.TronCall("/wallet/getblockbynum", req.Param{"num": num})

	if err != nil {
		return nil, err
	}
	block, err = NewBlock(r, c.config.TestNet)

	if err != nil {
		return nil, err
	}

	if block.GetBlockHashID() == "" || block.GetHeight() < 0 {
		return nil, errors.New("GetBlockByNum [" + strconv.FormatUint(num, 10) + "] failed: No found <block>")
	}

	return block, nil
}

func (c *Client) GetBlockByID(blockID string) (block *Block, err error) {
	r, err := c.TronCall("/wallet/getblockbyid", req.Param{"value": blockID})
	if err != nil {
		return nil, err
	}

	block, err = NewBlock(r, c.config.TestNet)

	if err != nil {
		return nil, err
	}

	if block.GetBlockHashID() == "" || block.GetHeight() < 0 {
		return nil, errors.New("GetBlockByID [" + blockID + "] failed: No found <block>")
	}

	return block, nil
}

func (c *Client) GetTransactionByID(txID string) (tx *Transaction, err error) {
	log.Println("Get transaction by id [" + txID + "]")

	r, err := c.TronCall("/wallet/gettransactionbyid", req.Param{"value": txID})
	if err != nil {
		return nil, err
	}

	i, err := c.GetTransactionInfoById(txID)
	if err != nil {
		return nil, err
	}

	bl, err := c.GetBlockByNum(i.BlockNumber)
	if err != nil {
		return nil, err
	}

	tx, err = NewTransaction(r, bl.Hash, bl.Height, bl.Time, c.config.TestNet)

	if err != nil {
		return nil, err
	}

	tx.Info = i

	return tx, err
}

func (c *Client) GetTransactionInfoById(txID string) (txInfo *TransactionInfo, err error) {
	log.Println("Loading txInfo [" + txID + "]")

	r, err := c.TronCall("/wallet/gettransactioninfobyid", req.Param{"value": txID})
	if err != nil {
		return nil, err
	}

	txInfo, err = NewTransactionInfo(r, c.config.TestNet)
	if err != nil {
		return nil, err
	}

	return txInfo, err
}

func (c *Client) GetAccountBalance(address string, block *Block) (int64, error) {
	params := &AccountBalanceParams{
		AccountIdentifier: &AccountIdentifier{
			Address: address,
		},
		BlockIdentifier: &BlockIdentifier{
			Hash:   block.Hash,
			Number: block.Height,
		},
	}

	r, err := c.TronCall("/wallet/getaccountbalance", params)
	if err != nil {
		return 0, err
	}

	return gjson.Get(r.Raw, "balance").Int(), nil
}

func (c *Client) GetContractInfo(contractAddress string) (*ContractInfo, error) {
	value, _, err := DecodeAddress(contractAddress, c.config.TestNet)

	if err != nil {
		return nil, err
	}
	params := req.Param{
		"value": value,
	}
	r, err := c.TronCall("/wallet/getcontract", params)
	if err != nil {
		return nil, err
	}
	return NewContractInfo(r), nil
}

// TriggerSmartContract triggers smart contract method
func (c *Client) TriggerSmartContract(
	contractAddress string,
	function string,
	parameter string,
	feeLimit uint64,
	callValue uint64,
	ownerAddress string) (*TransactionExtention, error) {
	params := req.Param{
		"contract_address":  contractAddress,
		"function_selector": function,
		"parameter":         parameter,
		"fee_limit":         feeLimit,
		"call_value":        callValue,
		"owner_address":     ownerAddress,
	}
	r, err := c.TronCall("/wallet/triggersmartcontract", params)
	if err != nil {
		return nil, err
	}
	return NewTransactionExtention(r), nil
}

func (c *Client) GetTRC20Balance(address string, contractAddress string) (*big.Int, error) {
	from, _, err := DecodeAddress(address, c.config.TestNet)
	if err != nil {
		return big.NewInt(0), err
	}

	caddr, _, err := DecodeAddress(contractAddress, c.config.TestNet)
	if err != nil {
		return big.NewInt(0), err
	}
	param, err := makeTransactionParameter("", []SolidityParam{
		SolidityParam{
			SOLIDITY_TYPE_ADDRESS,
			from,
		},
	})
	if err != nil {
		return big.NewInt(0), err
	}

	tx, err := c.TriggerSmartContract(
		caddr,
		TRC20_BALANCE_OF_METHOD,
		param,
		0,
		0,
		from)
	if err != nil {
		return big.NewInt(0), err
	}

	if len(tx.ConstantResult) > 0 {
		balance, err := StringValueToBigInt(tx.ConstantResult[0], 16)
		//balance, err := strconv.ParseInt(tx.ConstantResult[0], 16, 64)
		if err != nil {
			return big.NewInt(0), err
		}
		return balance, nil
	} else {
		nameBytes, _ := hex.DecodeString(tx.Result.Message)
		return big.NewInt(0), fmt.Errorf(string(nameBytes))
	}

	return big.NewInt(0), nil
}

func (c *Client) GetTRC10Balance(address string, tokenID string) (*big.Int, error) {

	a, _, err := c.GetTRXAccount(address)
	if err != nil {
		return big.NewInt(0), err
	}

	return a.AssetV2[tokenID], nil
}

func (c *Client) GetTRXAccount(address string) (account *Account, exist bool, err error) {
	ca, err := ConvertAddrToHex(address)

	if err != nil {
		return nil, false, err
	}

	params := req.Param{"address": ca}
	r, err := c.TronCall("/wallet/getaccount", params)
	if err != nil {
		return nil, false, err
	}
	account = NewAccount(r)

	if len(account.AddressHex) == 0 {
		return account, false, nil
	}

	return account, true, nil
}

func (c *Client) GetAssetInfoById(id string) (*AssetInfo, error) {
	params := req.Param{"value": id}
	r, err := c.SolidityCall("/walletsolidity/getassetissuebyid", params)
	if err != nil {
		return nil, err
	}

	return NewAssetInfo(r, c.config.TestNet), nil
}

func (c *Client) GetAssetInfoByName(name string) (*AssetInfo, error) {
	params := req.Param{"value": hex.EncodeToString([]byte(name))}
	r, err := c.SolidityCall("/walletsolidity/getassetissuebyname", params)
	if err != nil {
		return nil, err
	}

	return NewAssetInfo(r, c.config.TestNet), nil
}
