package db

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	vlq "github.com/bsm/go-vlq"
	"github.com/flier/gorocksdb"
	"github.com/golang/glog"
	"github.com/juju/errors"
	"github.com/trezor/blockbook/bchain"
	"github.com/trezor/blockbook/bchain/coins/trx"
)

const (
	TronTypeTrc10Contract = iota
	TronTypeTrc20Contract
)

// TronAddrContract is Contract address with number of transactions done by given address
type TronAddrContract struct {
	Type     uint
	Contract bchain.AddressDescriptor
	Txs      uint
}

// TronAddrContracts contains number of transactions and contracts for an address
type TronAddrContracts struct {
	TotalTxs       uint
	NonContractTxs uint
	Contracts      []TronAddrContract
}

func (d *RocksDB) storeTronAddressContracts(wb *gorocksdb.WriteBatch, acm map[string]*TronAddrContracts) error {
	buf := make([]byte, 64)
	varBuf := make([]byte, vlq.MaxLen64)

	for addrDesc, acs := range acm {
		// address with 0 contracts is removed from db - happens on disconnect
		if acs == nil || (acs.NonContractTxs == 0 && len(acs.Contracts) == 0) {
			wb.DeleteCF(d.cfh[cfAddressContracts], bchain.AddressDescriptor(addrDesc))
		} else {
			buf = buf[:0]

			l := packVaruint(acs.TotalTxs, varBuf)
			buf = append(buf, varBuf[:l]...)

			l = packVaruint(acs.NonContractTxs, varBuf)
			buf = append(buf, varBuf[:l]...)

			for _, ac := range acs.Contracts {
				l = packVaruint(ac.Type, varBuf)
				buf = append(buf, varBuf[:l]...)

				// write contract length first and then contract
				l = packVaruint(uint(binary.Size(ac.Contract)), varBuf)
				buf = append(buf, varBuf[:l]...)

				buf = append(buf, ac.Contract...)

				l = packVaruint(ac.Txs, varBuf)
				buf = append(buf, varBuf[:l]...)
			}
			wb.PutCF(d.cfh[cfAddressContracts], bchain.AddressDescriptor(addrDesc), buf)
		}
	}
	return nil
}

// GetTronAddrDescContracts returns AddrContracts for given addrDesc
func (d *RocksDB) GetTronAddrDescContracts(addrDesc bchain.AddressDescriptor) (*TronAddrContracts, error) {
	val, err := d.db.GetCF(d.ro, d.cfh[cfAddressContracts], addrDesc)
	if err != nil {
		return nil, err
	}
	defer val.Free()
	buf := val.Data()

	if len(buf) == 0 {
		return nil, nil
	}

	tt, l := unpackVaruint(buf)
	buf = buf[l:]

	nct, l := unpackVaruint(buf)
	buf = buf[l:]

	c := make([]TronAddrContract, 0, 4)
	for len(buf) > 0 {
		/*if len(buf) < trx.TronTypeAddressDescriptorLen {
			return nil, errors.New("Invalid data stored in cfAddressContracts for AddrDesc " + string(addrDesc))
		}*/

		t, l := unpackVaruint(buf)
		buf = buf[l:]

		cl, l := unpackVaruint(buf)
		buf = buf[l:]

		contract := append(bchain.AddressDescriptor(nil), buf[:cl]...)
		buf = buf[cl:]

		txs, l := unpackVaruint(buf)
		buf = buf[l:]

		c = append(c, TronAddrContract{
			Type:     t,
			Contract: contract,
			Txs:      txs,
		})
	}

	return &TronAddrContracts{
		TotalTxs:       tt,
		NonContractTxs: nct,
		Contracts:      c,
	}, nil
}

func findTronContractInAddressContracts(contract bchain.AddressDescriptor, contracts []TronAddrContract) (int, bool) {
	for i := range contracts {
		if bytes.Equal(contract, contracts[i].Contract) {
			return i, true
		}
	}
	return 0, false
}

func isBlackholeAddress(addrDesc bchain.AddressDescriptor) bool {
	return "TLsV52sRDL79HXGGm9yzwKibb6BeruhUzy" == string(addrDesc)
}

func (d *RocksDB) addToAddressesAndContractsTronType(addrDesc bchain.AddressDescriptor, btxID []byte, index int32, contract bchain.AddressDescriptor, contractType uint, addresses addressesMap, addressContracts map[string]*TronAddrContracts, addTxCount bool) error {
	var err error
	strAddrDesc := string(addrDesc)

	ac, e := addressContracts[strAddrDesc]
	if !e {
		ac, err = d.GetTronAddrDescContracts(addrDesc)
		if err != nil {
			return err
		}

		if ac == nil {
			ac = &TronAddrContracts{}
		}
		addressContracts[strAddrDesc] = ac
		d.cbs.balancesMiss++
	} else {
		d.cbs.balancesHit++
	}

	if contract == nil {
		if addTxCount {
			ac.NonContractTxs++
		}
	} else {
		// do not store contracts for TLsV52sRDL79HXGGm9yzwKibb6BeruhUzy address
		if !isBlackholeAddress(addrDesc) {
			// locate the contract and set i to the index in the array of contracts
			i, found := findTronContractInAddressContracts(contract, ac.Contracts)
			if !found {
				i = len(ac.Contracts)
				ac.Contracts = append(ac.Contracts, TronAddrContract{Type: contractType, Contract: contract})
			}
			// index 0 is for TRX transfers, contract indexes start with 1
			if index < 0 {
				index = ^int32(i + 1)
			} else {
				index = int32(i + 1)
			}
			if addTxCount {
				ac.Contracts[i].Txs++
			}
		}
	}
	counted := addToAddressesMap(addresses, strAddrDesc, btxID, index)
	if !counted {
		ac.TotalTxs++
	}
	return nil
}

type tronBlockTxContract struct {
	addr, contract bchain.AddressDescriptor
}

type tronBlockTx struct {
	btxID     []byte
	from, to  bchain.AddressDescriptor
	contracts []tronBlockTxContract
}

func (d *RocksDB) processAddressesTronType(block *bchain.Block, addresses addressesMap, addressContracts map[string]*TronAddrContracts) ([]tronBlockTx, error) {
	blockTxs := make([]tronBlockTx, len(block.Txs))
	for txi, tx := range block.Txs {
		btxID, err := d.chainParser.PackTxid(tx.Txid)
		if err != nil {
			return nil, err
		}
		blockTx := &blockTxs[txi]
		blockTx.btxID = btxID
		var from, to bchain.AddressDescriptor
		// there is only one output address in TronType transaction, store it in format txid 0
		if len(tx.Vout) == 1 && len(tx.Vout[0].ScriptPubKey.Addresses) == 1 {
			to, err = d.chainParser.GetAddrDescFromAddress(tx.Vout[0].ScriptPubKey.Addresses[0])
			if err != nil {
				// do not log ErrAddressMissing, transactions can be without to address (for example tron contracts)
				if err != bchain.ErrAddressMissing {
					glog.Warningf("rocksdb: addrDesc: %v - height %d, tx %v, output", err, block.Height, tx.Txid)
				}
				continue
			}
			if err = d.addToAddressesAndContractsTronType(to, btxID, 0, nil, 0, addresses, addressContracts, true); err != nil {
				return nil, err
			}
			blockTx.to = to
		}
		// there is only one input address in TronType transaction, store it in format txid ^0
		if len(tx.Vin) == 1 && len(tx.Vin[0].Addresses) == 1 {
			from, err = d.chainParser.GetAddrDescFromAddress(tx.Vin[0].Addresses[0])
			if err != nil {
				if err != bchain.ErrAddressMissing {
					glog.Warningf("rocksdb: addrDesc: %v - height %d, tx %v, input", err, block.Height, tx.Txid)
				}
				continue
			}
			if err = d.addToAddressesAndContractsTronType(from, btxID, ^int32(0), nil, 0, addresses, addressContracts, !bytes.Equal(from, to)); err != nil {
				return nil, err
			}
			blockTx.from = from
		}

		// store internal transactions
		internal, err := d.chainParser.TronTypeGetInternalFromTx(&tx)
		if err != nil {
			glog.Warningf("rocksdb: GetInternalFromTx %v - height %d, tx %v", err, block.Height, tx.Txid)
		}

		for _, it := range internal {
			to, err = d.chainParser.GetAddrDescFromAddress(it.To)
			if err != nil {
				return nil, err
			}
			if err = d.addToAddressesAndContractsTronType(to, btxID, 0, nil, 0, addresses, addressContracts, true); err != nil {
				return nil, err
			}

			from, err = d.chainParser.GetAddrDescFromAddress(it.From)
			if err != nil {
				return nil, err
			}
			if err = d.addToAddressesAndContractsTronType(from, btxID, ^int32(0), nil, 0, addresses, addressContracts, !bytes.Equal(from, to)); err != nil {
				return nil, err
			}
		}

		// store trc10 transfers
		trc10, err := d.chainParser.TronTypeGetTrc10FromTx(&tx)
		if err != nil {
			glog.Warningf("rocksdb: GetTrc10FromTx %v - height %d, tx %v", err, block.Height, tx.Txid)
		}

		// store trc20 transfers
		trc20, err := d.chainParser.TronTypeGetTrc20FromTx(&tx)
		if err != nil {
			glog.Warningf("rocksdb: GetTrc20FromTx %v - height %d, tx %v", err, block.Height, tx.Txid)
		}

		blockTx.contracts = make([]tronBlockTxContract, 0, len(trc10)+len(trc20))

		for i, t := range trc10 {
			var contract, from, to bchain.AddressDescriptor
			contract, err = d.chainParser.GetAddrDescFromAddress(t.Contract)
			if err == nil {
				from, err = d.chainParser.GetAddrDescFromAddress(t.From)
				if err == nil {
					to, err = d.chainParser.GetAddrDescFromAddress(t.To)
				}
			}
			if err != nil {
				glog.Warningf("rocksdb: GetTrc10FromTx %v - height %d, tx %v, transfer %v", err, block.Height, tx.Txid, t)
				continue
			}
			if err = d.addToAddressesAndContractsTronType(to, btxID, int32(i), contract, TronTypeTrc10Contract, addresses, addressContracts, true); err != nil {
				return nil, err
			}

			eq := bytes.Equal(from, to)

			blockTx.contracts = append(blockTx.contracts, tronBlockTxContract{
				addr:     from,
				contract: contract,
			})

			if err = d.addToAddressesAndContractsTronType(from, btxID, ^int32(i), contract, TronTypeTrc10Contract, addresses, addressContracts, !eq); err != nil {
				return nil, err
			}

			// add to address to blockTx.contracts only if it is different from from address
			if !eq {
				blockTx.contracts = append(blockTx.contracts, tronBlockTxContract{
					addr:     from,
					contract: contract,
				})
			}
		}

		for i, t := range trc20 {
			var contract, from, to bchain.AddressDescriptor
			contract, err = d.chainParser.GetAddrDescFromAddress(t.Contract)
			if err == nil {
				from, err = d.chainParser.GetAddrDescFromAddress(t.From)
				if err == nil {
					to, err = d.chainParser.GetAddrDescFromAddress(t.To)
				}
			}
			if err != nil {
				glog.Warningf("rocksdb: GetTrc10FromTx %v - height %d, tx %v, transfer %v", err, block.Height, tx.Txid, t)
				continue
			}
			if err = d.addToAddressesAndContractsTronType(to, btxID, int32(i), contract, TronTypeTrc20Contract, addresses, addressContracts, true); err != nil {
				return nil, err
			}

			eq := bytes.Equal(from, to)

			blockTx.contracts = append(blockTx.contracts, tronBlockTxContract{
				addr:     from,
				contract: contract,
			})

			if err = d.addToAddressesAndContractsTronType(from, btxID, ^int32(i), contract, TronTypeTrc20Contract, addresses, addressContracts, !eq); err != nil {
				return nil, err
			}

			// add to address to blockTx.contracts only if it is different from from address
			if !eq {
				blockTx.contracts = append(blockTx.contracts, tronBlockTxContract{
					addr:     from,
					contract: contract,
				})
			}
		}
	}
	return blockTxs, nil
}

func (d *RocksDB) storeAndCleanupBlockTxsTronType(wb *gorocksdb.WriteBatch, block *bchain.Block, blockTxs []tronBlockTx) error {
	pl := d.chainParser.PackedTxidLen()
	buf := make([]byte, 0, (pl*trx.TronTypeAddressDescriptorLen)*len(blockTxs))
	varBuf := make([]byte, vlq.MaxLen64)
	zeroAddress := make([]byte, trx.TronTypeAddressDescriptorLen)
	appendAddress := func(a bchain.AddressDescriptor) {
		if len(a) != trx.TronTypeAddressDescriptorLen {
			buf = append(buf, zeroAddress...)
		} else {
			buf = append(buf, a...)
		}
	}

	for i := range blockTxs {
		blockTx := &blockTxs[i]
		buf = append(buf, blockTx.btxID...)

		appendAddress(blockTx.from)
		appendAddress(blockTx.to)
		l := packVaruint(uint(len(blockTx.contracts)), varBuf)
		buf = append(buf, varBuf[:l]...)
		for j := range blockTx.contracts {
			c := &blockTx.contracts[j]
			appendAddress(c.addr)
			appendAddress(c.contract)
		}
	}
	key := packUint(block.Height)
	wb.PutCF(d.cfh[cfBlockTxs], key, buf)
	return d.cleanupBlockTxs(wb, block)
}

func (d *RocksDB) getBlockTxsTronType(height uint32) ([]tronBlockTx, error) {
	pl := d.chainParser.PackedTxidLen()
	val, err := d.db.GetCF(d.ro, d.cfh[cfBlockTxs], packUint(height))
	if err != nil {
		return nil, err
	}
	defer val.Free()
	buf := val.Data()
	// nil data means the key was not found in DB
	if buf == nil {
		return nil, nil
	}
	// buf can be empty slice, this means the block did not contain any transactions
	bt := make([]tronBlockTx, 0, 8)
	getAddress := func(i int) (bchain.AddressDescriptor, int, error) {
		if len(buf)-i < trx.TronTypeAddressDescriptorLen {
			glog.Error("rocksdb: Inconsistent data in blockTxs ", hex.EncodeToString(buf))
			return nil, 0, errors.New("Inconsistent data in blockTxs")
		}
		a := append(bchain.AddressDescriptor(nil), buf[i:i+trx.TronTypeAddressDescriptorLen]...)
		// return null addresses as nil
		for _, b := range a {
			if b != 0 {
				return a, i + trx.TronTypeAddressDescriptorLen, nil
			}
		}
		return nil, i + trx.TronTypeAddressDescriptorLen, nil
	}
	var from, to bchain.AddressDescriptor
	for i := 0; i < len(buf); {
		if len(buf)-i < pl {
			glog.Error("rocksdb: Inconsistent data in blockTxs ", hex.EncodeToString(buf))
			return nil, errors.New("Inconsistent data in blockTxs")
		}
		txid := append([]byte(nil), buf[i:i+pl]...)
		i += pl
		from, i, err = getAddress(i)
		if err != nil {
			return nil, err
		}
		to, i, err = getAddress(i)
		if err != nil {
			return nil, err
		}
		cc, l := unpackVaruint(buf[i:])
		i += l
		contracts := make([]tronBlockTxContract, cc)
		for j := range contracts {
			contracts[j].addr, i, err = getAddress(i)
			if err != nil {
				return nil, err
			}
			contracts[j].contract, i, err = getAddress(i)
			if err != nil {
				return nil, err
			}
		}
		bt = append(bt, tronBlockTx{
			btxID:     txid,
			from:      from,
			to:        to,
			contracts: contracts,
		})
	}
	return bt, nil
}

func (d *RocksDB) disconnectBlockTxsTronType(wb *gorocksdb.WriteBatch, height uint32, blockTxs []tronBlockTx, contracts map[string]*TronAddrContracts) error {
	glog.Info("Disconnecting block ", height, " containing ", len(blockTxs), " transactions")
	addresses := make(map[string]map[string]struct{})
	disconnectAddress := func(btxID []byte, addrDesc, contract bchain.AddressDescriptor) error {
		var err error
		// do not process empty address
		if len(addrDesc) == 0 {
			return nil
		}
		s := string(addrDesc)
		txid := string(btxID)
		// find if tx for this address was already encountered
		mtx, ftx := addresses[s]
		if !ftx {
			mtx = make(map[string]struct{})
			mtx[txid] = struct{}{}
			addresses[s] = mtx
		} else {
			_, ftx = mtx[txid]
			if !ftx {
				mtx[txid] = struct{}{}
			}
		}
		c, fc := contracts[s]
		if !fc {
			c, err = d.GetTronAddrDescContracts(addrDesc)
			if err != nil {
				return err
			}
			contracts[s] = c
		}
		if c != nil {
			if !ftx {
				c.TotalTxs--
			}
			if contract == nil {
				if c.NonContractTxs > 0 {
					c.NonContractTxs--
				} else {
					glog.Warning("AddressContracts ", addrDesc, ", tronTxs would be negative, tx ", hex.EncodeToString(btxID))
				}
			} else {
				i, found := findTronContractInAddressContracts(contract, c.Contracts)
				if found {
					if c.Contracts[i].Txs > 0 {
						c.Contracts[i].Txs--
						if c.Contracts[i].Txs == 0 {
							c.Contracts = append(c.Contracts[:i], c.Contracts[i+1:]...)
						}
					} else {
						glog.Warning("AddressContracts ", addrDesc, ", contract ", i, " Txs would be negative, tx ", hex.EncodeToString(btxID))
					}
				} else {
					glog.Warning("AddressContracts ", addrDesc, ", contract ", contract, " not found, tx ", hex.EncodeToString(btxID))
				}
			}
		} else {
			glog.Warning("AddressContracts ", addrDesc, " not found, tx ", hex.EncodeToString(btxID))
		}
		return nil
	}
	for i := range blockTxs {
		blockTx := &blockTxs[i]
		if err := disconnectAddress(blockTx.btxID, blockTx.from, nil); err != nil {
			return err
		}
		// if from==to, tx is counted only once and does not have to be disconnected again
		if !bytes.Equal(blockTx.from, blockTx.to) {
			if err := disconnectAddress(blockTx.btxID, blockTx.to, nil); err != nil {
				return err
			}
		}
		for _, c := range blockTx.contracts {
			if err := disconnectAddress(blockTx.btxID, c.addr, c.contract); err != nil {
				return err
			}
		}
		wb.DeleteCF(d.cfh[cfTransactions], blockTx.btxID)
	}
	for a := range addresses {
		key := packAddressKey([]byte(a), height)
		wb.DeleteCF(d.cfh[cfAddresses], key)
	}
	return nil
}

// DisconnectBlockRangeTronType removes all data belonging to blocks in range lower-higher
// it is able to disconnect only blocks for which there are data in the blockTxs column
func (d *RocksDB) DisconnectBlockRangeTronType(lower uint32, higher uint32) error {
	blocks := make([][]tronBlockTx, higher-lower+1)
	for height := lower; height <= higher; height++ {
		blockTxs, err := d.getBlockTxsTronType(height)
		if err != nil {
			return err
		}
		// nil blockTxs means blockTxs were not found in db
		if blockTxs == nil {
			return errors.Errorf("Cannot disconnect blocks with height %v and lower. It is necessary to rebuild index.", height)
		}
		blocks[height-lower] = blockTxs
	}
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	contracts := make(map[string]*TronAddrContracts)
	for height := higher; height >= lower; height-- {
		if err := d.disconnectBlockTxsTronType(wb, height, blocks[height-lower], contracts); err != nil {
			return err
		}
		key := packUint(height)
		wb.DeleteCF(d.cfh[cfBlockTxs], key)
		wb.DeleteCF(d.cfh[cfHeight], key)
	}
	d.storeTronAddressContracts(wb, contracts)
	err := d.db.Write(d.wo, wb)
	if err == nil {
		d.is.RemoveLastBlockTimes(int(higher-lower) + 1)
		glog.Infof("rocksdb: blocks %d-%d disconnected", lower, higher)
	}
	return err
}
