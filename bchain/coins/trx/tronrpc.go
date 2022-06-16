package trx

import (
	"context"
	"encoding/json"
	"github.com/golang/glog"
	"github.com/imroc/req"
	"github.com/juju/errors"
	"github.com/trezor/blockbook/bchain"
	"io/ioutil"
	"log"
	"math/big"
	"strconv"
	"sync"
	"time"
)

var (
	// ErrBlockNotFound is returned when block is not found
	ErrBlockNotFound = errors.New("Block not found")
	// ErrAddressMissing is returned if address is not specified
	ErrAddressMissing = errors.New("Address missing")
	// ErrTxidMissing is returned if txid is not specified
	ErrTxidMissing = errors.New("Txid missing")
	// ErrTxNotFound is returned if transaction was not found
	ErrTxNotFound = errors.New("Tx not found")
	// ErrTrc10TokenNotFound is returned if trc-10 token was not found
	ErrTrc10TokenNotFound = errors.New("TRC 10 token not found")
	// ErrTrc20TokenNotFound is returned if trc-20 token was not found
	ErrTrc20TokenNotFound = errors.New("TRC 20 token not found")
)

type Configuration struct {
	CoinName             string `json:"coin_name"`
	CoinShortcut         string `json:"coin_shortcut"`
	TronRPC              string `json:"tron_rpc_url"`
	SolidityRPC          string `json:"solidity_rpc_url"`
	RPCTimeout           int    `json:"rpc_timeout"`
	BlockAddressesToKeep int    `json:"block_addresses_to_keep"`
	TestNet              bool   `json:"testnet"`
	StopAtBlock          uint32 `json:"stop_at_block"`
}

type TronRPC struct {
	*bchain.BaseChain
	rpc         *Client
	ChainConfig *Configuration
	Mempool     *Mempool
	Parser      *TronParser

	chanNewBlock  chan uint32
	bestBlockLock sync.Mutex
	bestBlock     uint32
	bestBlockTime int64
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
	c, err := NewRPCConfig(config)
	if err != nil {
		return nil, err
	}

	cc := NewConfig()
	cc.TronNodeRPC = c.TronRPC
	cc.SolidityNodeRPC = c.SolidityRPC
	cc.TestNet = c.TestNet

	s := &TronRPC{
		BaseChain:   &bchain.BaseChain{},
		rpc:         NewClient(cc),
		ChainConfig: c,
		Parser:      NewTronParser(c.BlockAddressesToKeep),
	}

	// New blocks notifier
	s.chanNewBlock = make(chan uint32)
	go func() {
		for {
			h, ok := <-s.chanNewBlock
			if !ok {
				break
			}
			glog.V(2).Info("rpc: new block header ", h)

			s.bestBlockLock.Lock()
			s.bestBlock = h
			s.bestBlockTime = time.Now().Unix()
			s.bestBlockLock.Unlock()

			// notify blockbook
			pushHandler(bchain.NotificationNewBlock)
		}
	}()

	return s, nil
}

func NewRPCConfig(config json.RawMessage) (c *Configuration, err error) {
	//var err error
	//var c Configuration
	err = json.Unmarshal(config, &c)
	if err != nil {
		return nil, errors.Annotatef(err, "Invalid configuration file")
	}
	// keep at least 100 mappings block->addresses to allow rollback
	if c.BlockAddressesToKeep < 100 {
		c.BlockAddressesToKeep = 100
	}

	return c, nil
}

func (b *TronRPC) Initialize() error {
	b.Testnet = b.ChainConfig.TestNet

	if b.ChainConfig.TestNet {
		b.Network = "testnet"
	} else {
		b.Network = "livenet"
	}

	glog.Info("rpc: block chain ", b.Network)

	b.subscribeForNewBlocks()

	return nil
}

func (b *TronRPC) subscribeForNewBlocks() {
	ticker := time.NewTicker(1 * time.Second)
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				data, err := ioutil.ReadFile("/blockbook/config.json")
				if err != nil {
					log.Println("Error reading configfile")
				}
				var config json.RawMessage
				err = json.Unmarshal(data, &config)
				if err != nil {
					log.Println("Error parsing configfile")
				}

				c, err := NewRPCConfig(config)

				if err != nil {
					log.Println("Error creating config")
				}

				if b.ChainConfig.StopAtBlock != c.StopAtBlock {
					if c.StopAtBlock > b.ChainConfig.StopAtBlock {
						b.chanNewBlock <- c.StopAtBlock

						log.Printf("Stop trigger at %d\n", c.StopAtBlock)
					}

					b.ChainConfig = c
				}
			}
		}
	}()
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
		Blocks:          int(ni.BestBlockNumber),
		Bestblockhash:   ni.BestBlockHash,
		Difficulty:      "0",
		Version:         ni.Version,
		ProtocolVersion: ni.ProtocolVersion,
		Chain:           b.Network,
		Additional:      ni.Additional,
	}

	return rv, nil
}

func (b *TronRPC) GetBestBlockHash() (string, error) {
	bbh, err := b.GetBestBlockHeight()
	if err != nil {
		return "", err
	}

	bh, err := b.GetBlockHash(bbh)
	if err != nil {
		return "", err
	}

	return bh, nil
}

func (b *TronRPC) GetBestBlockHeight() (uint32, error) {
	if b.ChainConfig.StopAtBlock > 0 {
		return b.ChainConfig.StopAtBlock, nil
	}

	ni, err := b.rpc.GetNodeInfo()
	if err != nil {
		return 0, err
	}

	return uint32(ni.BestBlockNumber), nil
}

func (b *TronRPC) GetBlockHash(height uint32) (string, error) {
	bl, err := b.rpc.GetBlockByNum(height)

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

func (b *TronRPC) GetBlock(hash string, height uint32) (bbk *bchain.Block, err error) {
	/*if hash == "" {
		hash, err = b.GetBlockHash(height)

		if err != nil {
			return nil, err
		}
	}*/

	/*
	 * If height >= stop trigger - return block not found error
	 */
	bh, err := b.GetBestBlockHeight()
	if err != nil {
		return nil, err
	}

	if height > bh {
		return nil, bchain.ErrBlockNotFound
	}

	bl, err := b.rpc.GetBlockByNum(height)
	if err != nil {
		return nil, err
	}

	hash = bl.Hash

	//log.Printf("Block ["+hash+"] %d | TXS: %d\n", height, len(bl.Tx))

	bbh, err := b.tronHeaderToBlockHeader(bl)
	if err != nil {
		return nil, errors.Annotatef(err, "hash %v, height %v", hash, height)
	}

	btxs := make([]bchain.Tx, bl.GetTransactionsCount())

	// Load batch of transactions info
	txsInfo := make(map[string]*TransactionInfo)

	if bl.GetTransactionsCount() > 0 {
		txsInfo, err = b.rpc.GetTransactionInfoByBlockNum(bl.Height)
		if err != nil {
			return nil, err
		}
	}

	for i, tx := range bl.GetTransactions() {
		txInfo, ok := txsInfo[tx.TxID]
		if ok {
			tx.Info = txInfo
		}

		btx, err := tronTxToTx(tx, bbh.Time, uint32(bbh.Confirmations))
		if err != nil {
			return nil, errors.Annotatef(err, "hash %v, height %v, txid %v", hash, height, tx.TxID)
		}

		btxs[i] = *btx
	}

	bbk = &bchain.Block{
		BlockHeader: *bbh,
		Txs:         btxs,
	}

	return bbk, nil
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
		return nil, bchain.ErrTxNotFound
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

func (b *TronRPC) EthereumTypeGetNonce(addrDesc bchain.AddressDescriptor) (uint64, error) {
	return 0, nil
}

func (b *TronRPC) EthereumTypeEstimateGas(params map[string]interface{}) (uint64, error) {
	return 0, nil
}

func isNumeric(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func (b *TronRPC) TronTypeGetTrc10ContractInfo(contractDesc bchain.AddressDescriptor) (*bchain.Trc10Token, error) {
	var (
		ai  *AssetInfo
		err error
	)

	if isNumeric(string(contractDesc)) {
		ai, err = b.rpc.GetAssetInfoById(string(contractDesc))
	} else {
		ai, err = b.rpc.GetAssetInfoByName(string(contractDesc))
	}

	if err != nil {
		return nil, err
	}

	return &bchain.Trc10Token{
		Contract: ai.ID,
		Name:     ai.Name,
		Symbol:   ai.Abr,
		Decimals: ai.Decimals,
	}, nil
}

func (b *TronRPC) TronTypeGetTrc10ContractBalance(addrDesc, contractDesc bchain.AddressDescriptor) (*big.Int, error) {
	return b.rpc.GetTRC10Balance(string(addrDesc), string(contractDesc))
}

func (b *TronRPC) TronTypeGetTrc20ContractInfo(contractDesc bchain.AddressDescriptor) (*bchain.Trc20Contract, error) {
	ai, err := b.rpc.GetContractInfo(string(contractDesc))

	if err != nil {
		return nil, err
	}

	return &bchain.Trc20Contract{
		Contract: ai.ContractAddress,
		Name:     ai.Name,
		Symbol:   ai.Symbol,
		Decimals: ai.Decimals,
	}, nil
}

func (b *TronRPC) TronTypeGetAccount(addrDesc bchain.AddressDescriptor) (*bchain.TronAccount, error) {
	a, exist, err := b.rpc.GetTRXAccount(string(addrDesc))

	if err != nil {
		return nil, err
	}

	if !exist {
		return nil, ErrAddressMissing
	} else {
		taa, err := a.GetAddress(b.ChainConfig.TestNet)

		if err != nil {
			return nil, err
		}

		v := make([]*bchain.TronAccountVote, 0)
		for _, av := range a.Votes {
			v = append(v, &bchain.TronAccountVote{
				Address: av.Address,
				Count:   av.Count,
			})
		}

		f := make([]*bchain.TronAccountFrozenBalance, 0)
		for _, af := range a.Frozen {
			f = append(f, &bchain.TronAccountFrozenBalance{
				Balance:    af.Balance,
				ExpireTime: af.ExpireTime,
			})
		}

		ta := &bchain.TronAccount{
			Name:    a.Name,
			Address: taa,
			Balance: a.Balance,
			Votes:   v,
			Frozen:  f,
			Asset:   a.Asset,
			AssetV2: a.AssetV2,
		}

		return ta, nil
	}
}
