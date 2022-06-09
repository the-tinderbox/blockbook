package trx

import (
	"context"
	"encoding/json"
	"github.com/golang/glog"
	"github.com/imroc/req"
	"github.com/juju/errors"
	"github.com/trezor/blockbook/bchain"
	"log"
	"math/big"
)

type Configuration struct {
	CoinName             string `json:"coin_name"`
	CoinShortcut         string `json:"coin_shortcut"`
	TronRPC              string `json:"tron_rpc_url"`
	SolidityRPC          string `json:"solidity_rpc_url"`
	RPCTimeout           int    `json:"rpc_timeout"`
	BlockAddressesToKeep int    `json:"block_addresses_to_keep"`
	TestNet              bool   `json:"testnet"`
	BestBlock            uint32 `json:"best_block"`
}

type TronRPC struct {
	*bchain.BaseChain
	rpc         *Client
	ChainConfig *Configuration
	Mempool     *Mempool
	Parser      *TronParser
}

func NewTronRpcFulfilled(config json.RawMessage) (*TronRPC, error) {
	var err error
	var c Configuration
	err = json.Unmarshal(config, &c)
	if err != nil {
		return nil, errors.Annotatef(err, "Invalid configuration file")
	}
	// keep at least 100 mappings block->addresses to allow rollback
	if c.BlockAddressesToKeep < 100 {
		c.BlockAddressesToKeep = 100
	}

	cc := NewConfig()
	cc.TronNodeRPC = c.TronRPC
	cc.SolidityNodeRPC = c.SolidityRPC
	cc.TestNet = c.TestNet

	s := &TronRPC{
		BaseChain:   &bchain.BaseChain{},
		rpc:         NewClient(cc),
		ChainConfig: &c,
		Parser:      NewTronParser(c.BlockAddressesToKeep),
	}

	return s, nil
}

func NewTronRPC(config json.RawMessage, pushHandler func(bchain.NotificationType)) (bchain.BlockChain, error) {
	var err error
	var c Configuration
	err = json.Unmarshal(config, &c)
	if err != nil {
		return nil, errors.Annotatef(err, "Invalid configuration file")
	}
	// keep at least 100 mappings block->addresses to allow rollback
	if c.BlockAddressesToKeep < 100 {
		c.BlockAddressesToKeep = 100
	}

	cc := NewConfig()
	cc.TronNodeRPC = c.TronRPC
	cc.SolidityNodeRPC = c.SolidityRPC
	cc.TestNet = c.TestNet

	s := &TronRPC{
		BaseChain:   &bchain.BaseChain{},
		rpc:         NewClient(cc),
		ChainConfig: &c,
		Parser:      NewTronParser(c.BlockAddressesToKeep),
	}

	return s, nil
}

func (b *TronRPC) Initialize() error {
	b.Testnet = b.ChainConfig.TestNet

	if b.ChainConfig.TestNet {
		b.Network = "livenet"
	} else {
		b.Network = "testnet"
	}

	glog.Info("rpc: block chain ", b.Network)

	return nil
}

func (b *TronRPC) CreateMempool(chain bchain.BlockChain) (bchain.Mempool, error) {
	if b.Mempool == nil {
		b.Mempool = NewMempool()
	}

	return b.Mempool, nil
}

func (b *TronRPC) InitializeMempool(addrDescForOutpoint bchain.AddrDescForOutpointFunc, onNewTxAddr bchain.OnNewTxAddrFunc, onNewTx bchain.OnNewTxFunc) error {
	glog.Info("Tron does not support mempool")
	return nil
}

func (b *TronRPC) Shutdown(ctx context.Context) error {
	glog.Info("rpc: shutdown")
	return nil
}

func (b *TronRPC) GetSubversion() string {
	return ""
}

func (b *TronRPC) GetCoinName() string {
	return b.ChainConfig.CoinName
}

func (b *TronRPC) GetChainInfo() (*bchain.ChainInfo, error) {
	ni, err := b.rpc.GetNodeInfo()

	if err != nil {
		return nil, err
	}

	rv := &bchain.ChainInfo{
		Blocks:        int(ni.BestBlockNumber),
		Bestblockhash: ni.BestBlockHash,
		Difficulty:    "0",
		Version:       ni.Version,
		Chain:         b.Network,
	}

	return rv, nil
}

func (b *TronRPC) GetBestBlockHash() (string, error) {
	/*ni, err := b.rpc.GetNodeInfo()
	if err != nil {
		return "", err
	}

	return ni.BestBlockHash, nil*/
	bh, err := b.GetBestBlockHeight()
	if err != nil {
		return "", err
	}

	bl, err := b.rpc.GetBlockByNum(uint64(bh))
	if err != nil {
		return "", err
	}

	return bl.Hash, nil
}

func (b *TronRPC) GetBestBlockHeight() (uint32, error) {
	_, err := b.rpc.GetNodeInfo()
	if err != nil {
		return 0, err
	}

	//return uint32(ni.BestBlockNumber), nil
	return b.ChainConfig.BestBlock, nil // Временно парсим только 10 000 блоков
}

func (b *TronRPC) GetBlockHash(height uint32) (string, error) {
	bl, err := b.rpc.GetBlockByNum(uint64(height))

	if err != nil {
		return "", err
	}

	return bl.Hash, nil
}

func (b *TronRPC) GetBlockHeader(hash string) (*bchain.BlockHeader, error) {
	bl, err := b.rpc.GetBlockByID(hash)
	if err != nil {
		return nil, err
	}

	return b.tronHeaderToBlockHeader(bl)
}

func (b *TronRPC) tronHeaderToBlockHeader(block *Block) (*bchain.BlockHeader, error) {
	c, err := b.computeConfirmations(block.Height)
	if err != nil {
		return nil, err
	}

	return &bchain.BlockHeader{
		Hash:          block.Hash,
		Prev:          block.PrevHash,
		Height:        uint32(block.Height),
		Confirmations: int(c),
		Time:          block.Time,
		Size:          0, // Not supported
	}, nil
}

func (b *TronRPC) computeConfirmations(n uint64) (uint32, error) {
	bb, err := b.GetBestBlockHeight()
	if err != nil {
		return 0, err
	}

	// transaction in the best block has 1 confirmation
	return bb - uint32(n) + 1, nil
}

func (b *TronRPC) GetBlock(hash string, height uint32) (*bchain.Block, error) {
	log.Printf("Block ["+hash+"] %v\n", height)

	bl, err := b.rpc.GetBlockByID(hash)
	if err != nil {
		return nil, err
	}

	bbh, err := b.tronHeaderToBlockHeader(bl)
	if err != nil {
		return nil, errors.Annotatef(err, "hash %v, height %v", hash, height)
	}

	btxs := make([]bchain.Tx, bl.GetTransactionsCount())

	for i, tx := range bl.GetTransactions() {
		btx, err := tronTxToTx(tx, bbh.Time, uint32(bbh.Confirmations))
		if err != nil {
			return nil, errors.Annotatef(err, "hash %v, height %v, txid %v", hash, height, tx.TxID)
		}

		btxs[i] = *btx
	}

	bbk := bchain.Block{
		BlockHeader: *bbh,
		Txs:         btxs,
	}

	return &bbk, nil
}

func (b *TronRPC) GetBlockInfo(hash string) (*bchain.BlockInfo, error) {
	bl, err := b.rpc.GetBlockByID(hash)
	if err != nil {
		return nil, err
	}

	bch, err := b.tronHeaderToBlockHeader(bl)
	if err != nil {
		return nil, err
	}

	txs := make([]string, bl.GetTransactionsCount())

	for i, tx := range bl.GetTransactions() {
		txs[i] = tx.TxID
	}

	return &bchain.BlockInfo{
		BlockHeader: *bch,
		Difficulty:  "0",
		Nonce:       "0",
		Txids:       txs,
		MerkleRoot:  bl.MerkleRoot,
	}, nil
}

func (b *TronRPC) getBlockRaw(hash string, height uint32, fullTxs bool) (json.RawMessage, error) {
	r, err := b.rpc.TronCall("/wallet/getblockbyid", req.Param{"value": hash})
	if err != nil {
		return nil, err
	}

	return []byte(r.String()), nil
}

func (b *TronRPC) GetMempoolTransactions() ([]string, error) {
	return make([]string, 0), nil
}

func (b *TronRPC) GetTransaction(txid string) (*bchain.Tx, error) {
	log.Println("Loading transaction [" + txid + "] info")

	tx, err := b.rpc.GetTransactionByID(txid)
	if err != nil {
		return nil, err
	}

	c, err := b.computeConfirmations(tx.BlockHeight)
	if err != nil {
		return nil, err
	}

	return tronTxToTx(tx, tx.BlockTime, c)
}

func (b *TronRPC) GetTransactionForMempool(txid string) (*bchain.Tx, error) {
	return b.GetTransaction(txid)
}

func (b *TronRPC) GetTransactionSpecific(tx *bchain.Tx) (json.RawMessage, error) {
	log.Println("Get transaction specific for [" + tx.Txid + "]")

	csd, ok := tx.CoinSpecificData.(TransactionSpecificData)
	if !ok {
		ntx, err := b.GetTransaction(tx.Txid)
		if err != nil {
			return nil, err
		}
		csd, ok = ntx.CoinSpecificData.(TransactionSpecificData)
		if !ok {
			return nil, errors.New("Cannot get CoinSpecificData")
		}
	}
	m, err := json.Marshal(&csd)

	return json.RawMessage(m), err
}

func (b *TronRPC) EstimateFee(blocks int) (big.Int, error) {
	return *big.NewInt(0), nil
}

func (b *TronRPC) EstimateSmartFee(blocks int, conservative bool) (big.Int, error) {
	return b.EstimateFee(blocks)
}

func (b *TronRPC) SendRawTransaction(hex string) (string, error) {
	return "", errors.New("Send transactions is not supported")
}

func (b *TronRPC) GetChainParser() bchain.BlockChainParser {
	return b.Parser
}

func (b *TronRPC) EthereumTypeGetBalance(addrDesc bchain.AddressDescriptor) (*big.Int, error) {
	bl, err := b.rpc.GetNowBlock()

	if err != nil {
		return nil, err
	}

	bal, err := b.rpc.GetAccountBalance(addrDesc.String(), bl)

	if err != nil {
		return nil, err
	}

	return big.NewInt(bal), nil
}

func (b *TronRPC) EthereumTypeGetNonce(addrDesc bchain.AddressDescriptor) (uint64, error) {
	return 0, nil
}

func (b *TronRPC) EthereumTypeEstimateGas(params map[string]interface{}) (uint64, error) {
	return 0, nil
}

func (b *TronRPC) TronTypeGetTrc10ContractInfo(contractDesc bchain.AddressDescriptor) (*bchain.Trc10Contract, error) {
	ai, err := b.rpc.GetAssetInfoByName(string(contractDesc))

	if err != nil {
		return nil, err
	}

	return &bchain.Trc10Contract{
		Contract: ai.ID,
		Name:     ai.Name,
		Symbol:   ai.Abr,
		Decimals: 6,
	}, nil
}

func (b *TronRPC) TronTypeGetTrc20ContractInfo(contractDesc bchain.AddressDescriptor) (*bchain.Trc20Contract, error) {
	ai, err := b.rpc.GetContractInfo(string(contractDesc))

	if err != nil {
		return nil, err
	}

	return &bchain.Trc20Contract{
		Contract: ai.ContractAddress,
		Name:     ai.Name,
		Symbol:   ai.Name,
		Decimals: 6,
	}, nil
}
