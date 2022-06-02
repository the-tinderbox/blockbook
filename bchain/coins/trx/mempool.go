package trx

import "github.com/trezor/blockbook/bchain"

// Mempool Implements empty mempool, because trx has no support of mempool
type Mempool struct {
}

func NewMempool() *Mempool {
	return &Mempool{}
}

func (m *Mempool) Resync() (int, error) {
	return 0, nil
}
func (m *Mempool) GetTransactions(address string) ([]bchain.Outpoint, error) {
	return make([]bchain.Outpoint, 0), nil
}
func (m *Mempool) GetAddrDescTransactions(addrDesc bchain.AddressDescriptor) ([]bchain.Outpoint, error) {
	return make([]bchain.Outpoint, 0), nil
}
func (m *Mempool) GetAllEntries() bchain.MempoolTxidEntries {
	return make([]bchain.MempoolTxidEntry, 0)
}
func (m *Mempool) GetTransactionTime(txid string) uint32 {
	return 0
}
