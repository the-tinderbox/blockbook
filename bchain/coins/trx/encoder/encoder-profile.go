package encoder

var (
	TRONAlphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
)

type AddressType struct {
	EncodeType   string //编码类型
	Alphabet     string //码表
	ChecksumType string //checksum类型(Prefix string when encode type is base32PolyMod)
	HashType     string //地址hash类型，传入数据为公钥时起效
	HashLen      int    //编码前的数据长度
	Prefix       []byte //数据前面的填充
	Suffix       []byte //数据后面的填充
}

//func (at *AddressType) Prefix() []byte {
//	return at.Prefix
//}

var (
	//TRON stuff
	TRON_mainnetAddress = AddressType{"base58", TRONAlphabet, "doubleSHA256", "keccak256_last_twenty", 20, []byte{0x41}, nil}
	TRON_testnetAddress = AddressType{"base58", TRONAlphabet, "doubleSHA256", "keccak256_last_twenty", 20, []byte{0xa0}, nil}
)
