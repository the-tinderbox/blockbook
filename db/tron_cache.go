package db

import (
	"github.com/golang/glog"
	"github.com/trezor/blockbook/bchain"
	"github.com/trezor/blockbook/common"
)

// TronTokenCache is handle to TronTokenCacheServer
type TronTokenCache struct {
	db      *RocksDB
	chain   bchain.BlockChain
	metrics *common.Metrics
	is      *common.InternalState
	enabled bool
}

// NewTronTokenCache creates new TronTokenCache interface and returns its handle
func NewTronTokenCache(db *RocksDB, chain bchain.BlockChain, metrics *common.Metrics, is *common.InternalState, enabled bool) (*TronTokenCache, error) {
	if !enabled {
		glog.Info("tron_token_cache: disabled")
	}
	return &TronTokenCache{
		db:      db,
		chain:   chain,
		metrics: metrics,
		is:      is,
		enabled: enabled,
	}, nil
}

// GetToken returns token either from RocksDB or if not present from blockchain
func (c *TronTokenCache) GetToken(token string) (*bchain.Trc10Token, error) {
	var t *bchain.Trc10Token
	var err error
	if c.enabled {
		t, err = c.db.GetTronToken(token)
		if err != nil {
			return nil, err
		}
		if t != nil {
			c.metrics.TronTokenCacheEfficiency.With(common.Labels{"status": "hit"}).Inc()
			return t, nil
		}
	}

	t, err = c.chain.TronTypeGetTrc10ContractInfo(bchain.AddressDescriptor(token))
	if err != nil {
		return nil, err
	}

	c.metrics.TronTokenCacheEfficiency.With(common.Labels{"status": "miss"}).Inc()

	if c.enabled {
		err = c.db.PutTronToken(t)

		if err != nil {
			glog.Warning("PutTronToken ", t.Contract, ",error ", err)
		}
	}
	return t, nil
}
