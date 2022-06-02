package main

import (
	"encoding/hex"
	"github.com/trezor/blockbook/bchain/coins/trx"
	"log"
)

func main() {
	log.Println("Running test")

	c := trx.NewConfig()
	c.TronNodeRPC = "http://192.168.110.34:8090"
	c.SolidityNodeRPC = "http://192.168.110.34:8091"
	c.TestNet = false

	rc := trx.NewClient(c)

	log.Println(hex.EncodeToString([]byte("TRXTwitter")))
	
	ai, err := rc.GetAssetInfoByName("54525854776974746572")

	if err != nil {
		log.Println(err)
	}

	log.Println(ai)

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
