package trx

import (
	"fmt"
	"time"
)

// ----------------------------------- Functions -----------------------------------------------
func PrintBlock(block *Block) {
	if block == nil {
		fmt.Println("Block == nil")
	}

	fmt.Println("\nvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv")
	fmt.Println("Transactions:")
	for i, tx := range block.GetTransactions() {
		fmt.Println("\n\tvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv")
		fmt.Printf("TX: %s\n", tx.TxID)

		for ri, r := range tx.Ret {
			fmt.Printf("\tRet %v: %s\n", ri, r.Ret)
			fmt.Printf("\tFee %v: %v\n", ri, r.Fee)
			fmt.Printf("\tContractRet %v: %v\n\n", ri, r.ContractRet)
		}

		for ci, c := range tx.Contract {
			fmt.Printf("\tType %v: %s\n", ci, c.Type)
			fmt.Printf("\tContractName %v: %s\n", ci, string(c.ContractName))
			fmt.Printf("\tContractAddress %v: %s\n", ci, c.ContractAddress)
			fmt.Printf("\tProvider %v: %s\n", ci, c.Provider)
			fmt.Printf("\tFrom %v: %s\n", ci, c.From)
			fmt.Printf("\tTo %v: %s\n", ci, c.To)
			fmt.Printf("\tAmount %v: %s\n", ci, c.Amount)
		}

		fmt.Printf("\t tx%2d: IsSuccess=%v \t<BlockBytes=%v, BlockNum=%v, BlockHash=%+v, Contact=%v, Signature_len=%v>\n", i+1, true, "", tx.BlockHeight, tx.BlockHash, "", "")
		fmt.Println("\t^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^")
	}

	fmt.Println("Block Header:")
	if block != nil {
		fmt.Printf("\tRawData: <Number=%v, timestamp=%v, ParentHash=%v> \n", block.GetHeight(), time.Unix(int64(block.Time)/1000, 0), block.PrevHash)
	}

	fmt.Println("^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^")
}

func PrintAccount(account *Account, assets map[string]string, isTestnet bool) {
	fmt.Println("\nvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv")
	address, _ := account.GetAddress(isTestnet)

	fmt.Printf("Address: %s\n", address)
	fmt.Printf("Balance: %v\n", account.Balance)
	fmt.Println("Assets:")

	for asset, amount := range account.AssetV2 {
		fmt.Printf("\t%s: %v\n", assets[asset], amount)
	}

	fmt.Println("^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^")
}
