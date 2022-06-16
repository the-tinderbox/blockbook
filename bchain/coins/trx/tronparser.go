package trx

import (
	"encoding/json"
	"github.com/juju/errors"
	"github.com/trezor/blockbook/bchain"
	"math/big"
)

const TronTypeTokenDescriptorLen = 30

// TronTypeAddressDescriptorLen - in case of TronType, the AddressDescriptor has fixed length
const TronTypeAddressDescriptorLen = 34

// TronAmountDecimalPoint defines number of decimal points in TRX amounts
const TronAmountDecimalPoint = 6

type TronParser struct {
	*bchain.BaseParser
}

func NewTronParser(b int) *TronParser {
	return &TronParser{&bchain.BaseParser{
		BlockAddressesToKeep: b,
		AmountDecimalPoint:   TronAmountDecimalPoint,
	}}
}

func (p *TronParser) GetChainType() bchain.ChainType {
	return bchain.ChainTronType
}

func (p *TronParser) GetAddrDescFromVout(output *bchain.Vout) (bchain.AddressDescriptor, error) {
	if len(output.ScriptPubKey.Addresses) != 1 {
		return nil, bchain.ErrAddressMissing
	}
	return p.GetAddrDescFromAddress(output.ScriptPubKey.Addresses[0])
}

func (p *TronParser) GetAddrDescFromAddress(address string) (bchain.AddressDescriptor, error) {
	return []byte(address), nil
}

func (p *TronParser) GetAddressesFromAddrDesc(addrDesc bchain.AddressDescriptor) ([]string, bool, error) {
	return []string{string(addrDesc)}, true, nil
}

// GetScriptFromAddrDesc returns output script for given address descriptor
func (p *TronParser) GetScriptFromAddrDesc(addrDesc bchain.AddressDescriptor) ([]byte, error) {
	return addrDesc, nil
}

func (p *TronParser) PackBlockHash(hash string) ([]byte, error) {
	return []byte(hash), nil
}

func (p *TronParser) UnpackBlockHash(buf []byte) (string, error) {
	return string(buf), nil
}

func (p *TronParser) PackedTxidLen() int {
	return 64
}

// PackTxid packs txid to byte array
func (p *TronParser) PackTxid(txid string) ([]byte, error) {
	return []byte(txid), nil
}

// UnpackTxid unpacks byte array to txid
func (p *TronParser) UnpackTxid(buf []byte) (string, error) {
	return string(buf), nil
}

func GetHeightFromTx(tx *bchain.Tx) (uint32, error) {
	csd, ok := tx.CoinSpecificData.(TransactionSpecificData)
	if !ok {
		return 0, errors.New("Missing CoinSpecificData")
	}
	bn := csd.Tx.BlockHeight

	return uint32(bn), nil
}

// TxStatus is status of transaction
type TxStatus int

// TronTxData contains tron specific transaction data
type TronTxData struct {
	Status             TxStatus `json:"status"` // 1 OK, 0 Fail
	Data               string   `json:"data"`
	Type               string   `json:"type"`
	Fee                *big.Int `json:"fee"`
	EnergyUsed         uint64   `json:"energy_used"`
	EnergyBurn         uint64   `json:"energy_burn"`
	EnergyFromContract uint64   `json:"energy_from_contract"`
	BandwidthUsed      uint64   `json:"bandwidth_used"`
	BandwidthBurn      uint64   `json:"bandwidth_burn"`
}

func GetTronTxData(tx *bchain.Tx) *TronTxData {
	return GetTronTxDataFromSpecificData(tx.CoinSpecificData)
}

func GetTronTxDataFromSpecificData(coinSpecificData interface{}) *TronTxData {
	etd := TronTxData{Status: -1}
	csd, ok := coinSpecificData.(TransactionSpecificData)

	if ok {
		// If we have contract execution check contract return state
		if len(csd.Tx.Ret) > 0 {
			if csd.Tx.Ret[0].ContractRet == SUCCESS {
				etd.Status = 1
			} else if csd.Tx.Ret[0].ContractRet == REVERT {
				etd.Status = 0
			}
		} else {
			etd.Status = 1
		}

		etd.Type = csd.Tx.Contract[0].Type
		etd.Fee = csd.Tx.Info.Fee
		etd.EnergyUsed = csd.Tx.Info.Receipt.OriginEnergyUsage.Uint64()
		etd.EnergyBurn = csd.Tx.Info.Receipt.EnergyFee.Uint64()
		etd.EnergyFromContract = 0
		etd.BandwidthUsed = csd.Tx.Info.Receipt.NetUsage.Uint64()
		etd.BandwidthBurn = csd.Tx.Info.Receipt.NetFee.Uint64()
	}

	return &etd
}

func tronTxToTx(tx *Transaction, blockTime int64, confirmations uint32) (*bchain.Tx, error) {
	csd := TransactionSpecificData{
		Tx: tx,
	}

	valueSat := big.NewInt(0)
	to := tx.Contract[0].To

	if tx.Contract[0].Type != TransferAssetContract {
		valueSat.Set(tx.Contract[0].Amount)
	}

	if tx.Contract[0].Type == TriggerSmartContract && tx.Contract[0].ContractCallType == Trc20Transfer {
		to = tx.Contract[0].ContractAddress
	}

	vout := make([]bchain.Vout, 0)

	if tx.Contract[0].To != NO_ADDRESS {
		vout = []bchain.Vout{
			{
				N:        0, // there is always up to one To address
				ValueSat: *valueSat,
				ScriptPubKey: bchain.ScriptPubKey{
					// Hex
					Addresses: []string{to},
				},
			},
		}
	}

	return &bchain.Tx{
		Blocktime:     blockTime,
		Confirmations: confirmations,
		// Hex
		// LockTime
		Time: blockTime,
		Txid: tx.TxID,
		Vin: []bchain.Vin{
			{
				Addresses: []string{tx.Contract[0].From},
				// Coinbase
				// ScriptSig
				// Sequence
				// Txid
				// Vout
			},
		},
		Vout:             vout,
		CoinSpecificData: csd,
	}, nil
}

// Temp
type PackedTx struct {
	Tx string `json:"tx"`
}

func (p *TronParser) PackTx(tx *bchain.Tx, height uint32, blockTime int64) ([]byte, error) {
	r, ok := tx.CoinSpecificData.(TransactionSpecificData)
	if !ok {
		return nil, errors.New("Missing CoinSpecificData")
	}

	ptx, _ := json.Marshal(r.Tx)

	return ptx, nil
}

func (p *TronParser) UnpackTx(buf []byte) (*bchain.Tx, uint32, error) {
	var utx Transaction
	json.Unmarshal(buf, &utx)

	tx, err := tronTxToTx(&utx, utx.BlockTime, 0)
	if err != nil {
		return nil, 0, err
	}

	return tx, uint32(utx.BlockHeight), nil
}

func (p *TronParser) TronTypeGetTrc10FromTx(tx *bchain.Tx) ([]bchain.Trc10Transfer, error) {
	var r []bchain.Trc10Transfer

	csd, ok := tx.CoinSpecificData.(TransactionSpecificData)
	if ok {
		// TRC 10
		if csd.Tx.Contract[0].Type == TransferAssetContract {
			//log.Printf("TRC 10: CA: %s, F: %s, T: %s, A: %d", csd.Tx.Contract[0].ContractAddress, csd.Tx.Contract[0].From, csd.Tx.Contract[0].To, csd.Tx.Contract[0].Amount)

			r = append(r, bchain.Trc10Transfer{
				Contract: csd.Tx.Contract[0].ContractAddress,
				From:     csd.Tx.Contract[0].From,
				To:       csd.Tx.Contract[0].To,
				Tokens:   *csd.Tx.Contract[0].Amount,
			})
		}
	}

	return r, nil
}

func (p *TronParser) TronTypeGetTrc20FromTx(tx *bchain.Tx) ([]bchain.Trc20Transfer, error) {
	var r []bchain.Trc20Transfer

	csd, ok := tx.CoinSpecificData.(TransactionSpecificData)
	if ok {
		// TRC 20
		if csd.Tx.Contract[0].Type == TriggerSmartContract {
			// Get token transfers from tx
			if csd.Tx.Contract[0].ContractCallType == Trc20Transfer {
				r = append(r, bchain.Trc20Transfer{
					Contract: csd.Tx.Contract[0].ContractAddress,
					From:     csd.Tx.Contract[0].From,
					To:       csd.Tx.Contract[0].To,
					Tokens:   *csd.Tx.Contract[0].Amount,
				})
			}

			// Get token transfers from logs
			for _, l := range csd.Tx.Info.Log {
				contract, from, to, amount, err := ParseTransactionLog(l)

				if err == nil {
					r = append(r, bchain.Trc20Transfer{
						Contract: contract,
						From:     from,
						To:       to,
						Tokens:   *amount,
					})
				}
			}
		}
	}

	return r, nil
}

func (p *TronParser) TronTypeGetInternalFromTx(tx *bchain.Tx) ([]bchain.InternalTransfer, error) {
	var t []bchain.InternalTransfer

	csd, ok := tx.CoinSpecificData.(TransactionSpecificData)
	if ok {
		// TRC 20
		if csd.Tx.Contract[0].Type == TriggerSmartContract {
			for _, it := range csd.Tx.Info.InternalTransactions {
				t = append(t, bchain.InternalTransfer{
					From:  it.CallerAddress,
					To:    it.TransferToAddress,
					Value: *it.CallValueInfo[0].Value,
				})
			}
		}
	}

	return t, nil
}
