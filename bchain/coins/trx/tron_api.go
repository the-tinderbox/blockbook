package trx

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/imroc/req"
	"github.com/juju/errors"
	"github.com/tidwall/gjson"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
)

var (
	ErrEmptyResponse    = errors.New("Empty response from server")
	ErrContractNotFound = errors.New("Contract not found")
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

	/*pt := reflect.TypeOf(param)

	if pt != nil {
		cParam := param.(req.Param)
		paramValues := URL.Values{}

		for k, v := range cParam {
			paramValues.Set(k, fmt.Sprintf("%v", v))
		}

		log.Println("API CALL: " + url + "?" + paramValues.Encode())
	} else {
		log.Println("API CALL: " + url)
	}*/

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
	res, err := c.call(url, param)

	if err != nil {
		return nil, err
	}

	resErr := gjson.Get(res.Raw, "Error").String()

	if len(resErr) > 0 {
		return nil, errors.Annotatef(errors.New(resErr), "Invalid response from server")
	}

	if strings.TrimSpace(res.String()) == "{}" {
		return nil, ErrEmptyResponse
	}

	return res, nil
}

func (c *Client) SolidityCall(path string, param interface{}) (*gjson.Result, error) {
	url := c.config.SolidityNodeRPC + path
	res, err := c.call(url, param)

	if err != nil {
		return nil, err
	}

	resErr := gjson.Get(res.Raw, "Error").String()

	if len(resErr) > 0 {
		return nil, errors.Annotatef(errors.New(resErr), "Invalid response from server")
	}

	if strings.TrimSpace(res.String()) == "{}" {
		return nil, ErrEmptyResponse
	}

	return res, nil
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

func (c *Client) GetBlockByNum(num uint32) (block *Block, error error) {
	r, err := c.TronCall("/wallet/getblockbynum", req.Param{"num": num})

	if err != nil {
		return nil, err
	}
	block, err = NewBlock(r, c.config.TestNet)

	if err != nil {
		return nil, err
	}

	if block.GetBlockHashID() == "" || block.GetHeight() < 0 {
		return nil, errors.New("GetBlockByNum [" + strconv.FormatUint(uint64(num), 10) + "] failed: No found <block>")
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

func (c *Client) GetTransactionByID(txID string) (*Transaction, error) {
	//log.Println("Get transaction by id [" + txID + "]")

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

	tx, err := NewTransaction(r, bl.Hash, bl.Height, bl.Time, c.config.TestNet)

	if err != nil {
		return nil, err
	}

	tx.Info = i

	return tx, err
}

func (c *Client) GetTransactionInfoById(txID string) (txInfo *TransactionInfo, err error) {
	//log.Println("Loading txInfo [" + txID + "]")

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

func (c *Client) GetTransactionInfoByBlockNum(num uint64) (map[string]*TransactionInfo, error) {
	r, err := c.TronCall("/wallet/gettransactioninfobyblocknum", req.Param{"num": num})
	if err != nil {
		return nil, err
	}

	txsi := make(map[string]*TransactionInfo)

	if r.IsArray() {
		for _, ti := range r.Array() {
			txId := gjson.Get(ti.Raw, "id").String()

			txInfo, err := NewTransactionInfo(&ti, c.config.TestNet)
			if err != nil {
				return nil, err
			}

			txsi[txId] = txInfo
		}
	}

	return txsi, nil
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
	log.Println("Getting info about contract [" + string(contractAddress) + " (" + value + ")]")

	if err != nil {
		return nil, err
	}
	params := req.Param{
		"value": value,
	}
	r, err := c.TronCall("/wallet/getcontract", params)

	if err == ErrEmptyResponse {
		return nil, ErrContractNotFound
	} else if err != nil {
		return nil, err
	}

	ci, err := NewContractInfo(r, c.config.TestNet)
	if err != nil {
		return nil, err
	}

	// get precision of token
	r, err = c.TronCall("/wallet/triggerconstantcontract", req.Param{
		"contract_address":  value,
		"function_selector": "decimals()",
		"owner_address":     ci.OriginAddressHex,
	})

	if err != nil {
		return nil, err
	}

	te := NewTransactionExtention(r)

	if len(te.ConstantResult) > 0 {
		d, _ := strconv.ParseInt(te.ConstantResult[0], 16, 64)
		ci.Decimals = int(d)
	}

	// get token symbol
	r, err = c.TronCall("/wallet/triggerconstantcontract", req.Param{
		"contract_address":  value,
		"function_selector": "symbol()",
		"owner_address":     ci.OriginAddressHex,
	})

	if err != nil {
		return nil, err
	}

	te = NewTransactionExtention(r)

	if len(te.ConstantResult) > 0 && len(te.ConstantResult[0]) > 0 {
		s, err := hex.DecodeString(te.ConstantResult[0][len(te.ConstantResult[0])-64:])

		if err != nil {
			return nil, err
		}

		ci.Symbol = string(bytes.Trim(s, "\x00"))
	}

	// if no name specified get it from constant
	if len(ci.Name) == 0 {
		r, err = c.TronCall("/wallet/triggerconstantcontract", req.Param{
			"contract_address":  value,
			"function_selector": "name()",
			"owner_address":     ci.OriginAddressHex,
		})
		if err != nil {
			return nil, err
		}

		te = NewTransactionExtention(r)

		if len(te.ConstantResult) > 0 && len(te.ConstantResult[0]) > 0 {
			s, err := hex.DecodeString(te.ConstantResult[0][len(te.ConstantResult[0])-64:])

			if err != nil {
				return nil, err
			}

			ci.Name = string(bytes.Trim(s, "\x00"))
		}
	}

	return ci, nil
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

	// tokenID may be id or name
	if v2balance, ok := a.AssetV2[tokenID]; ok {
		return v2balance, nil
	} else if balance, ok := a.Asset[tokenID]; ok {
		return balance, nil
	} else {
		return big.NewInt(0), err
	}
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
	account, err = NewAccount(r, c.config.TestNet)
	if err != nil {
		return nil, false, err
	}

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
