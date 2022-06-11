package trx

import (
	"encoding/hex"
	"fmt"
	"github.com/blocktree/openwallet/v2/common"
	"github.com/tidwall/gjson"
	"github.com/trezor/blockbook/bchain/coins/trx/encoder"
	"log"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

const (
	TRC10 = "trc10"
	TRC20 = "trc20"

	TRC20_BALANCE_OF_METHOD  = "balanceOf(address)"
	TRC20_TRANSFER_METHOD_ID = "a9059cbb"
	TRX_TRANSFER_EVENT_ID    = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

	SOLIDITY_TYPE_ADDRESS = "address"
	SOLIDITY_TYPE_UINT256 = "uint256"
	SOLIDITY_TYPE_UINT160 = "uint160"

	SUCCESS = "SUCCESS"
	REVERT  = "REVERT"
)

// Contract Types
const (
	AccountCreateContract         = "AccountCreateContract"
	TransferContract              = "TransferContract"
	TransferAssetContract         = "TransferAssetContract"
	VoteWitnessContract           = "VoteWitnessContract"
	WitnessCreateContract         = "WitnessCreateContract"
	AssetIssueContract            = "AssetIssueContract"
	WitnessUpdateContract         = "WitnessUpdateContract"
	ParticipateAssetIssueContract = "ParticipateAssetIssueContract"
	AccountUpdateContract         = "AccountUpdateContract"
	FreezeBalanceContract         = "FreezeBalanceContract"
	UnfreezeBalanceContract       = "UnfreezeBalanceContract"
	WithdrawBalanceContract       = "WithdrawBalanceContract"
	UnfreezeAssetContract         = "UnfreezeAssetContract"
	UpdateAssetContract           = "UpdateAssetContract"
	ProposalCreateContract        = "ProposalCreateContract"
	ProposalApproveContract       = "ProposalApproveContract"
	ProposalDeleteContract        = "ProposalDeleteContract"
	SetAccountIdContract          = "SetAccountIdContract"
	CreateSmartContract           = "CreateSmartContract"
	TriggerSmartContract          = "TriggerSmartContract"
	UpdateSettingContract         = "UpdateSettingContract"
	ExchangeCreateContract        = "ExchangeCreateContract"
	ExchangeInjectContract        = "ExchangeInjectContract"
	ExchangeWithdrawContract      = "ExchangeWithdrawContract"
	ExchangeTransactionContract   = "ExchangeTransactionContract"
	ShieldedTransferContract      = "ShieldedTransferContract"
	ClearABIContract              = "ClearABIContract"
	UpdateBrokerageContract       = "UpdateBrokerageContract"
	UpdateEnergyLimitContract     = "UpdateEnergyLimitContract"

	Trc20Transfer = "trc20_transfer"
	UnknownCall   = "unknown"
)

// SolidityParam Type for requests into solidity node
type SolidityParam struct {
	ParamType  string
	ParamValue interface{}
}

func makeRepeatString(c string, count uint) string {
	cs := make([]string, 0)
	for i := 0; i < int(count); i++ {
		cs = append(cs, c)
	}
	return strings.Join(cs, "")
}

func makeTransactionParameter(methodId string, params []SolidityParam) (string, error) {

	data := methodId
	for i, _ := range params {
		var param string
		if params[i].ParamType == SOLIDITY_TYPE_ADDRESS {
			param = strings.ToLower(params[i].ParamValue.(string))
			param = strings.TrimPrefix(param, "0x")
			if len(param) != 42 {
				return "", fmt.Errorf("length of address error.")
			}
			param = makeRepeatString("0", 22) + param
		} else if params[i].ParamType == SOLIDITY_TYPE_UINT256 {
			intParam := params[i].ParamValue.(*big.Int)
			param = intParam.Text(16)
			l := len(param)
			if l > 64 {
				return "", fmt.Errorf("integer overflow.")
			}
			param = makeRepeatString("0", uint(64-l)) + param
			//fmt.Println("makeTransactionData intParam:", intParam.String(), " param:", param)
		} else {
			return "", fmt.Errorf("not support solidity type")
		}

		data += param
	}
	return data, nil
}

type AdditionalChainInfo struct {
	ActiveConnections uint32
	TotalMemory       int64
	FreeMemory        int64
}

// NodeInfo Returns node info
type NodeInfo struct {
	BestBlockNumber uint64
	BestBlockHash   string
	Version         string
	ProtocolVersion string
	Additional      *AdditionalChainInfo
}

// NewNodeInfo Returns new NodeInfo
func NewNodeInfo(json *gjson.Result) *NodeInfo {
	r, _ := regexp.Compile(`Num:(\d+),ID:(.*)`)

	matches := r.FindStringSubmatch(gjson.Get(json.Raw, "block").String())

	bbn, _ := strconv.ParseUint(matches[1], 0, 64)

	return &NodeInfo{
		BestBlockNumber: bbn,
		BestBlockHash:   matches[2],
		Version:         gjson.Get(json.Raw, "configNodeInfo").Get("codeVersion").String(),
		ProtocolVersion: gjson.Get(json.Raw, "configNodeInfo").Get("p2pVersion").String(),
		Additional: &AdditionalChainInfo{
			ActiveConnections: uint32(gjson.Get(json.Raw, "activeConnectCount").Uint()),
			TotalMemory:       gjson.Get(json.Raw, "machineInfo").Get("jvmTotalMemory").Int(),
			FreeMemory:        gjson.Get(json.Raw, "machineInfo").Get("jvmFreeMemory").Int(),
		},
	}
}

// AccountIdentifier Type for balance request
type AccountIdentifier struct {
	Address string `json:"address"`
}

// BlockIdentifier Type for balance request
type BlockIdentifier struct {
	Hash   string `json:"hash"`
	Number uint64 `json:"number"`
}

// AccountBalanceParams Type for balance request
type AccountBalanceParams struct {
	AccountIdentifier *AccountIdentifier `json:"account_identifier"`
	BlockIdentifier   *BlockIdentifier   `json:"block_identifier"`
	Visible           bool               `json:"visible"`
}

// Block Describes block with transactions
type Block struct {
	Hash       string         `json:"hash"`
	Tx         []*Transaction `json:"tx"`
	PrevHash   string         `json:"prev_hash"`
	Height     uint64         `json:"height"`
	Version    uint64         `json:"version"`
	Time       int64          `json:"time"`
	Fork       bool           `json:"fork"`
	TxHash     []string       `json:"tx_hash"`
	MerkleRoot string         `json:"merkle_root"'`
	//Confirmations     uint64
}

func (block *Block) Print() {
	log.Println("Block: " + block.Hash)
	log.Println("PrevHash: " + block.PrevHash)
	log.Printf("Height: %v\n", block.Height)
	log.Printf("Time: %v\n", block.Time)
	log.Println("Transactions:")

	for _, t := range block.GetTransactions() {
		t.Print()
	}
}

// GetHeight Returns block height
func (block *Block) GetHeight() uint64 {
	return block.Height
}

// GetBlockHashID Returns block hash
func (block *Block) GetBlockHashID() string {
	return block.Hash
}

// GetTransactions Returns block transactions
func (block *Block) GetTransactions() []*Transaction {
	return block.Tx
}

// GetTransactionsCount Returns count of transactions in block
func (block *Block) GetTransactionsCount() int {
	return len(block.Tx)
}

// NewBlock Returns new Block
func NewBlock(json *gjson.Result, isTestnet bool) (*Block, error) {
	header := gjson.Get(json.Raw, "block_header").Get("raw_data")

	b := &Block{}
	b.Hash = gjson.Get(json.Raw, "blockID").String()
	b.PrevHash = header.Get("parentHash").String()
	b.Height = header.Get("number").Uint()
	b.Version = header.Get("version").Uint()
	b.Time = header.Get("timestamp").Int() / 1000
	b.MerkleRoot = header.Get("txTrieRoot").String()

	txs := make([]*Transaction, 0)
	for _, x := range gjson.Get(json.Raw, "transactions").Array() {
		ti, err := NewTransaction(&x, b.Hash, b.Height, b.Time, isTestnet)

		if err != nil {
			return nil, err
		}

		txs = append(txs, ti)
	}

	b.Tx = txs

	return b, nil
}

type TransactionSpecificData struct {
	Tx *Transaction `json:"tx"`
}

// TransactionInfo
type TransactionInfo struct {
	TxID                 string                            `json:"id"`
	Fee                  *big.Int                          `json:"fee"`
	BlockNumber          uint64                            `json:"blockNumber"`
	BlockTimeStamp       int64                             `json:"blockTimeStamp"`
	ContractAddress      string                            `json:"contract_address"`
	Receipt              *TransactionInfoReceipt           `json:"receipt"`
	Log                  []*TransactionInfoLog             `json:"log"`
	InternalTransactions []*TransactionInternalTransaction `json:"internal_transactions"`
	AssetIssueId         string                            `json:"assetIssueID"`
}

type TransactionInternalTransaction struct {
	Hash              string                                     `json:"hash"`
	CallerAddress     string                                     `json:"caller_address"`
	TransferToAddress string                                     `json:"transferTo_address"`
	Note              string                                     `json:"note"`
	CallValueInfo     []*TransactionInternalTransactionCallValue `json:"call_value_info"`
}

type TransactionInternalTransactionCallValue struct {
	Value *big.Int
}

type TransactionInfoLog struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
	Data    string   `json:"data"`
}

type TransactionInfoReceipt struct {
	EnergyFee         *big.Int `json:"energy_fee"`
	OriginEnergyUsage *big.Int `json:"origin_energy_usage"`
	EnergyUsageTotal  *big.Int `json:"energy_usage_total"`
	NetFee            *big.Int `json:"net_fee"`
	NetUsage          *big.Int `json:"net_usage"`
	Result            string   `json:"result"`
}

func NewTransactionInfoLog(json *gjson.Result, isTestnet bool) (*TransactionInfoLog, error) {
	le := &TransactionInfoLog{}

	a, err := EncodeAddress(gjson.Get(json.Raw, "address").String(), isTestnet)

	if err != nil {
		return nil, err
	}

	le.Address = a
	log.Println("Address: " + a)

	t := make([]string, 0)

	for _, x := range gjson.Get(json.Raw, "topics").Array() {
		t = append(t, x.String())
	}

	le.Topics = t

	return le, nil
}

func NewTransactionInternalTransaction(json *gjson.Result, isTestnet bool) (*TransactionInternalTransaction, error) {
	it := &TransactionInternalTransaction{
		Hash: gjson.Get(json.Raw, "hash").String(),
		Note: gjson.Get(json.Raw, "note").String(),
	}

	ca, err := EncodeAddress(gjson.Get(json.Raw, "caller_address").String(), isTestnet)

	if err != nil {
		return nil, err
	}

	it.CallerAddress = ca

	ta, err := EncodeAddress(gjson.Get(json.Raw, "transferTo_address").String(), isTestnet)

	if err != nil {
		return nil, err
	}

	it.TransferToAddress = ta

	cvi := make([]*TransactionInternalTransactionCallValue, 0)
	for _, cv := range gjson.Get(json.Raw, "callValueInfo").Array() {
		cvi = append(cvi, &TransactionInternalTransactionCallValue{
			Value: big.NewInt(cv.Get("callValue").Int()),
		})
	}

	it.CallValueInfo = cvi

	return it, nil
}

func NewTransactionInfo(json *gjson.Result, isTestnet bool) (*TransactionInfo, error) {
	l := make([]*TransactionInfoLog, 0)

	for _, x := range gjson.Get(json.Raw, "log").Array() {
		le, err := NewTransactionInfoLog(&x, isTestnet)

		if err != nil {
			return nil, err
		}

		l = append(l, le)
	}

	it := make([]*TransactionInternalTransaction, 0)

	for _, x := range gjson.Get(json.Raw, "internal_transactions").Array() {
		ite, err := NewTransactionInternalTransaction(&x, isTestnet)

		if err != nil {
			return nil, err
		}

		it = append(it, ite)
	}

	return &TransactionInfo{
		TxID:           gjson.Get(json.Raw, "id").String(),
		Fee:            big.NewInt(gjson.Get(json.Raw, "fee").Int()),
		BlockNumber:    gjson.Get(json.Raw, "blockNumber").Uint(),
		BlockTimeStamp: gjson.Get(json.Raw, "blockTimeStamp").Int() / 1000,
		Receipt: &TransactionInfoReceipt{
			EnergyFee:         big.NewInt(gjson.Get(json.Raw, "receipt.energy_fee").Int()),
			OriginEnergyUsage: big.NewInt(gjson.Get(json.Raw, "receipt.energy_usage").Int()),
			EnergyUsageTotal:  big.NewInt(gjson.Get(json.Raw, "receipt.energy_usage_total").Int()),
			NetFee:            big.NewInt(gjson.Get(json.Raw, "receipt.net_fee").Int()),
			NetUsage:          big.NewInt(gjson.Get(json.Raw, "receipt.net_usage").Int()),
			Result:            gjson.Get(json.Raw, "result").String(),
		},
		Log:                  l,
		InternalTransactions: it,
	}, nil
}

// Transaction Describes transaction
type Transaction struct {
	TxID        string           `json:"id"`
	BlockHash   string           `json:"block_hash"`
	BlockHeight uint64           `json:"block_height"`
	BlockTime   int64            `json:"block_time"`
	IsCoinBase  bool             `json:"coinbase"`
	Ret         []*Result        `json:"ret"`
	Contract    []*Contract      `json:"contract"`
	Info        *TransactionInfo `json:"info"`
}

func (t *Transaction) Print() {
	log.Println("\tTX ID: " + t.TxID)
	log.Println("\tContracts:")

	for _, c := range t.Contract {
		c.Print()
	}
}

// NewTransaction Returns new Transaction
func NewTransaction(json *gjson.Result, blockHash string, blockHeight uint64, blocktime int64, isTestnet bool) (*Transaction, error) {
	rawData := gjson.Get(json.Raw, "raw_data")

	b := &Transaction{}
	b.TxID = gjson.Get(json.Raw, "txID").String()

	//log.Println("Parsing tx [" + b.TxID + "]")

	b.BlockHash = blockHash
	b.BlockHeight = blockHeight
	b.BlockTime = blocktime

	b.Ret = make([]*Result, 0)
	if rets := gjson.Get(json.Raw, "ret"); rets.IsArray() {
		for _, r := range rets.Array() {
			ret := NewResult(r)
			b.Ret = append(b.Ret, ret)
		}
	}

	b.Contract = make([]*Contract, 0)
	if contracts := rawData.Get("contract"); contracts.IsArray() {
		for i, c := range contracts.Array() {
			ci, err := NewContract(c, isTestnet)

			if err != nil {
				return nil, err
			}

			contract := ci
			contract.TxID = b.TxID
			contract.BlockHash = blockHash
			contract.BlockHeight = blockHeight
			contract.BlockTime = blocktime
			if len(b.Ret) > i {
				contract.ContractRet = b.Ret[i].ContractRet
			}
			b.Contract = append(b.Contract, contract)
		}
	}

	return b, nil
}

// Result Represents transaction execution result
type Result struct {
	Ret         string `json:"ret"`
	Fee         int64  `json:"fee"`
	ContractRet string `json:"contract_ret"`
}

// NewResult Returns new Result
func NewResult(json gjson.Result) *Result {
	b := &Result{}
	b.Ret = gjson.Get(json.Raw, "ret").String()
	b.Fee = gjson.Get(json.Raw, "fee").Int()
	b.ContractRet = gjson.Get(json.Raw, "contractRet").String()
	return b
}

// Contract Describes contract log
type Contract struct {
	TxID             string       `json:"tx_id"`
	BlockHash        string       `json:"block_hash"`
	BlockHeight      uint64       `json:"block_height"`
	BlockTime        int64        `json:"block_time"`
	Type             string       `json:"type"`
	ContractCallType string       `json:"contract_call_type"`
	Parameter        gjson.Result `json:"parameter"`
	Provider         []byte       `json:"provider"`
	ContractName     []byte       `json:"contract_name"`
	From             string       `json:"from"`
	To               string       `json:"to"`
	Amount           *big.Int     `json:"amount"`
	ContractAddress  string       `json:"contract_address"`
	SourceKey        string       `json:"source_key"`
	ContractRet      string       `json:"contract_ret"`
	Protocol         string       `json:"protocol"`
}

func (c *Contract) Print() {
	log.Println("\t\tType: " + c.Type)
	log.Println("\t\tContractAddress: " + c.ContractAddress)
	log.Println("\t\tFrom: " + c.From)
	log.Println("\t\tTo: " + c.To)
}

// NewContract Returns new Contract
func NewContract(json gjson.Result, isTestnet bool) (*Contract, error) {
	var err error

	b := &Contract{}
	b.Type = gjson.Get(json.Raw, "type").String()
	b.Parameter = gjson.Get(json.Raw, "parameter")
	b.From, err = EncodeAddress(b.Parameter.Get("value.owner_address").String(), isTestnet)
	b.Amount = common.StringNumToBigIntWithExp("0", 0)
	b.ContractCallType = UnknownCall

	if err != nil {
		return nil, err
	}

	switch b.Type {

	case WitnessCreateContract: // SR creator
		b.Amount = common.StringNumToBigIntWithExp("9999000000", 0)

	case AccountUpdateContract: // Contract update
		b.Amount = common.StringNumToBigIntWithExp("0", 0)

	case FreezeBalanceContract: // Freeze
		b.To = b.From
		b.Amount = common.StringNumToBigIntWithExp(b.Parameter.Get("value.frozen_balance").String(), 0)

	case AssetIssueContract: // Contract creation
		b.Amount = common.StringNumToBigIntWithExp("0", 0)

	case VoteWitnessContract: // Vote
		b.Amount = common.StringNumToBigIntWithExp("0", 0)

	case TransferContract: //TRX
		b.To, err = EncodeAddress(b.Parameter.Get("value.to_address").String(), isTestnet)
		if err != nil {
			return nil, err
		}
		b.Amount = common.StringNumToBigIntWithExp(b.Parameter.Get("value.amount").String(), 0)
		//b.Amount = b.Parameter.Get("value.amount").Int()
		b.Protocol = ""

	case TransferAssetContract: //TRC10
		b.To, err = EncodeAddress(b.Parameter.Get("value.to_address").String(), isTestnet)
		if err != nil {
			return nil, err
		}
		b.Amount = common.StringNumToBigIntWithExp(b.Parameter.Get("value.amount").String(), 0)
		//b.Amount = b.Parameter.Get("value.amount").Int()
		assetsByte, err := hex.DecodeString(b.Parameter.Get("value.asset_name").String())

		if err != nil {
			return nil, err
		}
		b.ContractAddress = string(assetsByte)
		b.Protocol = TRC10

	case TriggerSmartContract: //TRC20
		b.ContractAddress, err = EncodeAddress(b.Parameter.Get("value.contract_address").String(), isTestnet)
		if err != nil {
			return nil, err
		}

		b.To = b.ContractAddress

		data := b.Parameter.Get("value.data").String()

		// Trying to parse transfer
		to, amount, err := ParseTransferEvent(data)
		if err != nil {
			b.Amount = common.StringNumToBigIntWithExp(b.Parameter.Get("value.call_value").String(), 0)
		} else {
			b.ContractCallType = Trc20Transfer

			b.To, err = EncodeAddress(to, isTestnet)
			if err != nil {
				return nil, err
			}

			b.Amount = amount
		}

		b.Protocol = TRC20
	}

	return b, nil
}

// ContractInfo Describes contract
type ContractInfo struct {
	Bytecode                   string
	Name                       string
	ConsumeUserResourcePercent uint64
	ContractAddress            string
	ABI                        string
}

// NewContractInfo Returns new ContractInfo
func NewContractInfo(json *gjson.Result) *ContractInfo {
	obj := &ContractInfo{}
	obj.Bytecode = json.Get("bytecode").String()
	obj.Name = json.Get("name").String()
	obj.ConsumeUserResourcePercent = json.Get("consume_user_resource_percent").Uint()
	obj.ContractAddress = json.Get("contract_address").String()
	obj.ABI = json.Get("abi.entrys").Raw
	return obj
}

// TransactionExtension Detailed info for transaction
type TransactionExtention struct {
	Transaction    gjson.Result `json:"transaction" `
	Txid           string       `json:"txid"`
	ConstantResult []string     `json:"constant_result"`
	Result         *Return      `json:"result"`
}

// NewTransactionExtention Returns new TransactionExtension
func NewTransactionExtention(json *gjson.Result) *TransactionExtention {
	b := &TransactionExtention{}
	b.Transaction = json.Get("transaction")
	result := json.Get("result")
	b.Result = NewReturn(&result)
	b.Txid = json.Get("txid").String()

	b.ConstantResult = make([]string, 0)
	if constant_result := json.Get("constant_result"); constant_result.IsArray() {
		for _, c := range constant_result.Array() {
			b.ConstantResult = append(b.ConstantResult, c.String())
		}
	}

	return b
}

// Return TransactionExtension execution result
type Return struct {
	Result  bool   `json:"result"`
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

// NewReturn Returns new Return
func NewReturn(json *gjson.Result) *Return {
	b := &Return{}
	b.Result = json.Get("result").Bool()
	b.Code = json.Get("code").Int()
	msg, _ := hex.DecodeString(json.Get("message").String())
	b.Message = string(msg)
	return b
}

// Account Describes account info
type Account struct {
	AddressHex          string
	Balance             int64
	FreeNetUsage        int64
	AssetV2             map[string]*big.Int
	FreeAssetNetUsageV2 map[string]int64
}

// NewAccount Returns new Account
func NewAccount(json *gjson.Result) *Account {
	obj := &Account{}
	obj.AddressHex = json.Get("address").String()
	obj.Balance = json.Get("balance").Int()
	obj.FreeNetUsage = json.Get("free_net_usage").Int()

	obj.AssetV2 = make(map[string]*big.Int, 0)
	assetV2 := json.Get("assetV2")
	if assetV2.IsArray() {
		for _, as := range assetV2.Array() {
			obj.AssetV2[as.Get("key").String()] = StringNumToBigIntWithExp(as.Get("value").String(), 0)
		}
	}

	obj.FreeAssetNetUsageV2 = make(map[string]int64, 0)
	freeAssetNetUsageV2 := json.Get("free_asset_net_usageV2")
	if freeAssetNetUsageV2.IsArray() {
		for _, as := range freeAssetNetUsageV2.Array() {
			obj.FreeAssetNetUsageV2[as.Get("key").String()] = as.Get("value").Int()
		}
	}
	return obj
}

func (a *Account) GetAddress(isTestnet bool) (string, error) {
	return EncodeAddress(a.AddressHex, isTestnet)
}

// AssetInfo TRC-10 asset info
type AssetInfo struct {
	OwnerAddress string
	ID           string
	Name         string
	Abr          string
}

// NewAssetInfo Returns new AssetInfo
func NewAssetInfo(json *gjson.Result, isTestnet bool) *AssetInfo {
	oa, _ := EncodeAddress(gjson.Get(json.Raw, "owner_address").String(), isTestnet)
	n, _ := hex.DecodeString(gjson.Get(json.Raw, "name").String())
	a, _ := hex.DecodeString(gjson.Get(json.Raw, "abbr").String())

	return &AssetInfo{
		OwnerAddress: oa,
		Name:         string(n),
		Abr:          string(a),
		ID:           gjson.Get(json.Raw, "id").String(),
	}
}

func ConvertAddrToHex(address string) (string, error) {
	toAddressBytes, err := encoder.AddressDecode(address, encoder.TRON_mainnetAddress)
	if err != nil {
		return "", err
	}
	toAddressBytes = append([]byte{0x41}, toAddressBytes...)
	return hex.EncodeToString(toAddressBytes), nil
}

func DecodeAddress(addr string, isTestnet bool) (string, []byte, error) {
	codeType := encoder.TRON_mainnetAddress
	if isTestnet {
		codeType = encoder.TRON_testnetAddress
	}

	toAddressBytes, err := encoder.AddressDecode(addr, codeType)
	if err != nil {
		return "", nil, err
	}
	toAddressBytes = append(codeType.Prefix, toAddressBytes...)
	return hex.EncodeToString(toAddressBytes), toAddressBytes, nil
}

func EncodeAddress(hexStr string, isTestnet bool) (string, error) {
	codeType := encoder.TRON_mainnetAddress
	if isTestnet {
		codeType = encoder.TRON_testnetAddress
	}

	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", err
	}
	if len(b) > 20 {
		b = b[1:]
	}

	addr := encoder.AddressEncode(b, codeType)
	return addr, nil
}
