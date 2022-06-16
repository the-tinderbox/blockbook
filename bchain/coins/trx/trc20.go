package trx

import (
	"fmt"
	"github.com/shopspring/decimal"
	"github.com/trezor/blockbook/bchain"
	"log"
	"math/big"
	"strings"
	"sync"
)

const (
	TransferDataLength = 68 * 2
)

// ParseTransferEvent Parses TRC-20 transfer logs
func ParseTransferEvent(data string) (string, *big.Int, error) {
	var (
		to     string
		amount = big.NewInt(0)
	)

	if len(data) != TransferDataLength {
		return "", amount, fmt.Errorf("call data is not transfer")
	}
	if data[0:8] != TRC20_TRANSFER_METHOD_ID {
		return "", amount, fmt.Errorf("call method is not transfer")
	}
	to = data[32:72]
	amount, _ = new(big.Int).SetString(data[72:], 16)
	//log.Infof("bigAmount = %s", bigAmount.String())
	return to, amount, nil

	//amount, err := strconv.ParseInt(data[72:], 16, 64)
	//if err != nil {0000000000000000000000000000000000000000000000000001da3cb331b01f
	//	return "", 0, err
	//}
	//return to, amount, nil
}

func ParseTransactionLog(infoLog *TransactionInfoLog) (string, string, string, *big.Int, error) {
	if len(infoLog.Topics) == 3 && infoLog.Topics[0] == TRX_TRANSFER_EVENT_ID {
		amount, _ := new(big.Int).SetString(infoLog.Data, 16)

		return infoLog.Address, infoLog.Topics[1], infoLog.Topics[2], amount, nil
	}

	return "", "", "", nil, fmt.Errorf("log is not a transfer")
}

func StringValueToBigInt(value string, base int) (*big.Int, error) {
	bigvalue := new(big.Int)
	var success bool

	if value == "" {
		value = "0"
	}
	value = strings.TrimPrefix(value, "0x")

	_, success = bigvalue.SetString(value, base)
	if !success {
		return big.NewInt(0), fmt.Errorf("convert value [%v] to bigint failed, check the value and base passed through", value)
	}
	return bigvalue, nil
}

func StringNumToBigIntWithExp(amount string, exp int32) *big.Int {
	vDecimal, _ := decimal.NewFromString(amount)
	vDecimal = vDecimal.Shift(exp)
	bigInt, ok := new(big.Int).SetString(vDecimal.String(), 10)
	if !ok {
		return big.NewInt(0)
	}
	return bigInt
}

var cachedContracts = make(map[string]*bchain.Erc20Contract)
var cachedContractsMux sync.Mutex

func (p *TronRPC) EthereumTypeGetErc20ContractInfo(contractDesc bchain.AddressDescriptor) (*bchain.Erc20Contract, error) {
	cds := string(contractDesc)
	cachedContractsMux.Lock()
	contract, found := cachedContracts[cds]
	cachedContractsMux.Unlock()

	if !found {
		address := string(contractDesc)
		log.Println("CONTRACT!!!" + address)

		//data, err := b.ethCall(erc20NameSignature, address)
		/*if err != nil {
			// ignore the error from the eth_call - since geth v1.9.15 they changed the behavior
			// and returning error "execution reverted" for some non contract addresses
			// https://github.com/ethereum/go-ethereum/issues/21249#issuecomment-648647672
			glog.Warning(errors.Annotatef(err, "erc20NameSignature %v", address))
			return nil, nil
			// return nil, errors.Annotatef(err, "erc20NameSignature %v", address)
		}*/
		//name := parseErc20StringProperty(contractDesc, data)
		/*if name != "" {
			data, err = b.ethCall(erc20SymbolSignature, address)
			if err != nil {
				glog.Warning(errors.Annotatef(err, "erc20SymbolSignature %v", address))
				return nil, nil
				// return nil, errors.Annotatef(err, "erc20SymbolSignature %v", address)
			}
			symbol := parseErc20StringProperty(contractDesc, data)
			data, err = b.ethCall(erc20DecimalsSignature, address)
			if err != nil {
				glog.Warning(errors.Annotatef(err, "erc20DecimalsSignature %v", address))
				// return nil, errors.Annotatef(err, "erc20DecimalsSignature %v", address)
			}
			contract = &bchain.Erc20Contract{
				Contract: address,
				Name:     name,
				Symbol:   symbol,
			}
			d := parseErc20NumericProperty(contractDesc, data)
			if d != nil {
				contract.Decimals = int(uint8(d.Uint64()))
			} else {
				contract.Decimals = EtherAmountDecimalPoint
			}
		} else {
			contract = nil
		}*/
		cachedContractsMux.Lock()
		cachedContracts[cds] = contract
		cachedContractsMux.Unlock()
	}
	return contract, nil
}
