package main

import (
	"encoding/binary"
	"github.com/bsm/go-vlq"
	"github.com/trezor/blockbook/bchain"
	"log"
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

func packVarint(i int, buf []byte) int {
	return vlq.PutInt(buf, int64(i))
}

func packVaruint(i uint, buf []byte) int {
	return vlq.PutUint(buf, uint64(i))
}

func unpackVaruint(buf []byte) (uint, int) {
	i, ofs := vlq.Uint(buf)
	return uint(i), ofs
}

func unpackVarint(buf []byte) (int, int) {
	i, ofs := vlq.Int(buf)
	return int(i), ofs
}

func unpackToken(buf []byte) *bchain.Trc10Token {
	// get contract length first and then contract
	cl, l := unpackVaruint(buf)
	buf = buf[l:]

	c := buf[:cl]
	buf = buf[cl:]

	// get name length first and then contract
	nl, l := unpackVaruint(buf)
	buf = buf[l:]

	n := buf[:nl]
	buf = buf[nl:]

	// get symbol length first and then contract
	sl, l := unpackVaruint(buf)
	buf = buf[l:]

	s := buf[:sl]
	buf = buf[sl:]

	d, l := unpackVarint(buf)

	return &bchain.Trc10Token{
		Contract: string(c),
		Name:     string(n),
		Symbol:   string(s),
		Decimals: d,
	}
}

func packToken(token *bchain.Trc10Token) []byte {
	// Create buffer of maximum size for contract data
	buf := make([]byte, 64)
	varBuf := make([]byte, vlq.MaxLen64)

	buf = buf[:0]

	// write contract length first and then contract
	c := []byte(token.Contract)
	cl := packVaruint(uint(binary.Size(c)), varBuf)
	buf = append(buf, varBuf[:cl]...)
	buf = append(buf, c...)

	// write name length first and then contract
	n := []byte(token.Name)
	nl := packVaruint(uint(binary.Size(n)), varBuf)
	buf = append(buf, varBuf[:nl]...)
	buf = append(buf, n...)

	// write symbol length first and then contract
	s := []byte(token.Symbol)
	sl := packVaruint(uint(binary.Size(s)), varBuf)
	buf = append(buf, varBuf[:sl]...)
	buf = append(buf, s...)

	// write decimals
	dl := packVarint(token.Decimals, varBuf)
	buf = append(buf, varBuf[:dl]...)

	return buf
}

func main() {
	var token *bchain.Trc10Token

	token = &bchain.Trc10Token{
		Contract: "test-contract",
		Name:     "test-contract-name",
		Symbol:   "tst",
		Decimals: 10,
	}

	log.Println(token)

	packed := packToken(token)

	log.Println(packed)

	token = unpackToken(packed)

	log.Println(token)

	pointerBuf := []byte("pt:1000003")
	log.Println(string(pointerBuf[:3]))
	log.Println(string(pointerBuf[3:]))

	//st, _ := hex.DecodeString("4372616674796d653630")
	//log.Println()
	/*a, _ := trx.EncodeAddress("41734c2f23ab41c52308d1206c4eb5fe8e124e6898", false)
	log.Println(a)*/
	/*buf := []byte{53, 49, 102, 98, 102, 51, 57, 97, 100, 55, 49, 102, 100, 102, 102, 97, 50, 51, 48, 100, 102, 101, 52, 49, 49, 98, 54, 97, 97, 97, 52, 98, 51, 50, 97, 99, 51, 49, 57, 50, 102, 102, 97, 51, 52, 54, 48, 98, 99, 101, 49, 100, 56, 98, 98, 50, 100, 54, 101, 50, 97, 50, 49, 53}
	log.Println(len(string(buf)))*/

	/*buf := make([]byte, 64)
	zeroContract := make([]byte, trx.TronTypeTokenDescriptorLen)
	appendContract := func(a bchain.AddressDescriptor) {
		if len(a) != trx.TronTypeTokenDescriptorLen {
			buf = append(buf, zeroContract...)
		} else {
			buf = append(buf, a...)
		}
	}

	appendContract(bchain.AddressDescriptor("test"))
	log.Println(buf)
	os.Exit(0)*/

	//test := "aaaaaaaaaa"
	//log.Println(hex.EncodeToString([]byte(test)))
	//os.Exit(0)

	//str := "000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000054d43532d50000000000000000000000000000000000000000000000000000000"
	//log.Println(str[len(str)-64:])
	//i, _ := hex.DecodeString(str[len(str)-64:])
	//fmt.Printf("%x", bytes.Trim(i, "\x00"))
	//log.Println(string(bytes.Trim(i, "\x00")))

	//os.Exit(0)
	//buf := []byte{21, 21, 0, 84, 82, 88, 84, 101, 115, 116, 67, 111, 105, 110, 2}
	//buf := []byte{21, 21, 0, 84, 82, 88, 84, 101, 115, 116, 67, 111, 105, 110, 2}
	/*log.Printf("^int32(0): %d", ^int32(0))
	log.Printf("^int32(i + 1): %d", ^int32(^int32(0)+1))

	acs := &TronAddrContracts{
		TotalTxs:       1123,
		NonContractTxs: 927,
		Contracts: []TronAddrContract{
			TronAddrContract{
				Type:     TronTypeTrc10Contract,
				Contract: []byte("TAHJP2Mb9kAXe5UmiYbUhoW1CGHuKCRNpw"),
				Txs:      13,
			},
			TronAddrContract{
				Type:     TronTypeTrc20Contract,
				Contract: []byte("TXGAjm87CGe4DWNjqXGmbDiDFsADPLT5wM"),
				Txs:      881,
			},
		},
	}
	buf := make([]byte, 64)
	varBuf := make([]byte, vlq.MaxLen64)

	buf = buf[:0]

	l := packVaruint(acs.TotalTxs, varBuf)
	buf = append(buf, varBuf[:l]...)

	log.Println(buf)
	log.Println(varBuf)
	log.Println("------------")

	l = packVaruint(acs.NonContractTxs, varBuf)
	buf = append(buf, varBuf[:l]...)

	log.Println(buf)
	log.Println(varBuf)
	log.Println("------------")

	for _, ac := range acs.Contracts {
		log.Println("Contract...")

		l = packVaruint(ac.Type, varBuf)
		buf = append(buf, varBuf[:l]...)

		log.Println(buf)
		log.Println(varBuf)
		log.Println("------------")

		buf = append(buf, ac.Contract...)

		log.Println(buf)
		log.Println(varBuf)
		log.Println("------------")

		l = packVaruint(ac.Txs, varBuf)
		buf = append(buf, varBuf[:l]...)

		log.Println(buf)
		log.Println(varBuf)
		log.Println("------------")
		log.Println("End contract")
	}*/

	/*log.Println("Buffer")
	log.Println(string(buf))

	tt, l := unpackVaruint(buf)
	buf = buf[l:]

	nct, l := unpackVaruint(buf)
	buf = buf[l:]

	log.Printf("TotalTxs: %d\n", tt)
	log.Printf("NonContractTxs: %d\n", nct)
	log.Println("Contracts: ")

	for len(buf) > 0 {
		t, l := unpackVaruint(buf)
		buf = buf[l:]

		log.Printf("Type: %d\n", t)

		var cl int

		if t == TronTypeTrc10Contract {
			cl = trx.TronTypeTokenDescriptorLen
		} else {
			cl = trx.TronTypeAddressDescriptorLen
		}

		contract := append(bchain.AddressDescriptor(nil), buf[:cl]...)
		buf = buf[cl:]

		txs, l := unpackVaruint(buf)
		buf = buf[l:]

		log.Printf("Type: %d\n", t)
		log.Printf("Contract: %s\n", string(contract))
		log.Printf("Txs: %d\n", txs)
		log.Println("-------")
	}*/

	/*log.Println("Running test")

	c := trx.NewConfig()
	c.TronNodeRPC = "http://192.168.110.34:8090"
	c.SolidityNodeRPC = "http://192.168.110.34:8091"
	c.TestNet = false

	//rc := trx.NewClient(c)

	config, err := ioutil.ReadFile("./bin/config.json")
	if err != nil {
		log.Println(err)
	}

	rpc, err := trx.NewTronRpcFulfilled(config)

	if err != nil {
		log.Println(err)

		os.Exit(0)
	}

	tx, err := rpc.GetTransaction("asfasfasfasfasfasf")

	if err != nil {
		log.Println(err)
		os.Exit(0)
	}

	log.Println(tx.Txid)*/

	/*log.Println(hex.EncodeToString([]byte("TRXTwitter")))

	ai, err := rc.GetAssetInfoByName("54525854776974746572")

	if err != nil {
		log.Println(err)
	}

	log.Println(ai)*/

	/*_, err := rc.GetBlockByNum(0)

	if err != nil {
		log.Println(err)
	} else {
		//trx.PrintBlock(block)
	}*/

	/*config, err := ioutil.ReadFile("./bin/config.json")
	if err != nil {
		log.Println(err)
	}

	rpc, err := trx.NewTronRpcFulfilled(config)

	_, err = rpc.GetBlock("00000000004c2b48830b65a048373351a4d6bcea65ad116f57e7c9c0573c1e04", 4991816)

	if err != nil {
		log.Println(err)
	}*/

	/*var a string

	a, _ = trx.EncodeAddress("411022ce81420283645968e8929d3a3412647a6422", false)
	log.Println("Tx owner address: " + a)

	a, _ = trx.EncodeAddress("41a5ae9c370df734211d9059fdfa3f2044f9054c91", false)
	log.Println("Tx contract address: " + a)

	a, _ = trx.EncodeAddress("574138562ca8bd1d35068c5b21279610c1f583c4", false)
	log.Println("L1 address: " + a)

	a, _ = trx.EncodeAddress("a5ae9c370df734211d9059fdfa3f2044f9054c91", false)
	log.Println("L1 T1: " + a)

	a, _ = trx.EncodeAddress("1022ce81420283645968e8929d3a3412647a6422", false)
	log.Println("L1 T2: " + a)

	d, _ := new(big.Int).SetString("0000000000000000000000000000000000000000000000000000000002faf080", 16)
	log.Printf("L1 DATA: %v\n", d)

	a, _ = trx.EncodeAddress("a5ae9c370df734211d9059fdfa3f2044f9054c91", false)
	log.Println("L2 address: " + a)*/

	//log.Println(bl)

	//_, err = rpc.GetBlockInfo("00000000004c2b48830b65a048373351a4d6bcea65ad116f57e7c9c0573c1e04")

	/*if err != nil {
		log.Println(err)
	}*/

	//log.Println(bi)

	/*a, exists, err := rc.GetTRXAccount("TN93Yq91SpCtiHaLGNXq8HCJ4WsvRK5dup")

	if err != nil {
		log.Println(err)
	}

	if exists {
		log.Println("Exists!")
	} else {
		log.Println("Not exists!")
	}

	var ai *trx.AssetInfo
	assets := make(map[string]string, 0)

	for id, _ := range a.AssetV2 {
		ai, _ = rc.GetAssetInfo(id)

		assets[ai.ID] = ai.Name + "(" + ai.Abr + ")"
	}

	trx.PrintAccount(a, assets, c.TestNet)*/
}
