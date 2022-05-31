package fantom

import (
	"context"
	"encoding/json"
	"github.com/golang/glog"
	"github.com/juju/errors"
	"github.com/trezor/blockbook/bchain"
	"github.com/trezor/blockbook/bchain/coins/eth"
)

type FantomNet uint32

const (
	MainNet FantomNet = 250

	//Mumbai
	TestNet FantomNet = 4002
)

type Configuration struct {
	CoinName                    string `json:"coin_name"`
	CoinShortcut                string `json:"coin_shortcut"`
	RPCURL                      string `json:"rpc_url"`
	RPCTimeout                  int    `json:"rpc_timeout"`
	BlockAddressesToKeep        int    `json:"block_addresses_to_keep"`
	MempoolTxTimeoutHours       int    `json:"mempoolTxTimeoutHours"`
	QueryBackendOnMempoolResync bool   `json:"queryBackendOnMempoolResync"`
}

type FantomRPC struct {
	*eth.EthereumRPC
}

func NewFantomRPC(config json.RawMessage, pushHandler func(bchain.NotificationType)) (bchain.BlockChain, error) {
	b, err := eth.NewEthereumRPC(config, pushHandler)
	if err != nil {
		return nil, err
	}

	s := &FantomRPC{
		b.(*eth.EthereumRPC),
	}

	return s, nil
}

func (b *FantomRPC) Initialize() error {
	ctx, cancel := context.WithTimeout(context.Background(), b.Timeout)
	defer cancel()

	id, err := b.Client.NetworkID(ctx)
	if err != nil {
		return err
	}

	// parameters for getInfo request
	switch FantomNet(id.Uint64()) {
	case MainNet:
		b.Testnet = false
		b.Network = "livenet"
		break
	case TestNet:
		b.Testnet = true
		b.Network = "testnet"
		break
	default:
		return errors.Errorf("Unknown network id %v", id)
	}
	glog.Info("rpc: block chain ", b.Network)

	return nil
}
