package encoder

import (
	"errors"
)

var (
	ErrorInvalidHashLength = errors.New("Invalid hash length!")
	ErrorInvalidAddress    = errors.New("Invalid address!")
)

// CalcChecksum return calculated checksum
func CalcChecksum(data []byte, chkType string) []byte {
	return calcChecksum(data, chkType)
}

func calcChecksum(data []byte, chkType string) []byte {
	if chkType == "doubleSHA256" {
		return Hash(data, 0, HASH_ALG_DOUBLE_SHA256)[:4]
	}

	return nil
}

// VerifyChecksum return checksum result
func VerifyChecksum(data []byte, chkType string) bool {
	return verifyChecksum(data, chkType)
}

func verifyChecksum(data []byte, chkType string) bool {
	checksum := calcChecksum(data[:len(data)-4], chkType)
	for i := 0; i < 4; i++ {
		if checksum[i] != data[len(data)-4+i] {
			return false
		}
	}
	return true
}

// CatData cat two bytes data
func CatData(data1 []byte, data2 []byte) []byte {
	return catData(data1, data2)
}

func catData(data1 []byte, data2 []byte) []byte {
	if data2 == nil {
		return data1
	}
	return append(data1, data2...)
}

func recoverData(data, prefix, suffix []byte) ([]byte, error) {
	for i := 0; i < len(prefix); i++ {
		if data[i] != prefix[i] {
			return nil, ErrorInvalidAddress
		}
	}
	if suffix != nil {
		for i := 0; i < len(suffix); i++ {
			if data[len(data)-len(suffix)+i] != suffix[i] {
				return nil, ErrorInvalidAddress
			}
		}
	}
	if suffix == nil {
		return data[len(prefix):], nil
	}
	return data[len(prefix) : len(data)-len(suffix)], nil
}

// EncodeData return encoded data
func EncodeData(data []byte, encodeType string, alphabet string) string {
	return encodeData(data, encodeType, alphabet)
}

func encodeData(data []byte, encodeType string, alphabet string) string {
	if encodeType == "base58" {
		return Base58Encode(data, NewBase58Alphabet(alphabet))
	}
	return ""
}

// DecodeData return decoded data
func DecodeData(data, encodeType, alphabet, checkType string, prefix, suffix []byte) ([]byte, error) {
	return decodeData(data, encodeType, alphabet, checkType, prefix, suffix)
}

func decodeData(data, encodeType, alphabet, checkType string, prefix, suffix []byte) ([]byte, error) {
	if encodeType == "base58" {
		ret, err := Base58Decode(data, NewBase58Alphabet(alphabet))
		if err != nil {
			return nil, ErrorInvalidAddress
		}
		if verifyChecksum(ret, checkType) == false {
			return nil, ErrorInvalidAddress
		}
		return recoverData(ret[:len(ret)-4], prefix, suffix)
	}
	return nil, nil
}

func calcHash(data []byte, hashType string) []byte {
	if hashType == "keccak256_last_twenty" {
		return Hash(data, 32, HASH_ALG_KECCAK256)[12:32]
	}

	return nil
}

func AddressEncode(hash []byte, addresstype AddressType) string {

	if len(hash) != addresstype.HashLen {
		hash = calcHash(hash, addresstype.HashType)
	}

	data := catData(catData(addresstype.Prefix, hash), addresstype.Suffix)
	return encodeData(catData(data, calcChecksum(data, addresstype.ChecksumType)), addresstype.EncodeType, addresstype.Alphabet)

}

func AddressDecode(address string, addresstype AddressType) ([]byte, error) {
	data, err := decodeData(address, addresstype.EncodeType, addresstype.Alphabet, addresstype.ChecksumType, addresstype.Prefix, addresstype.Suffix)
	if err != nil {
		return nil, err
	}
	if len(data) != addresstype.HashLen {
		return nil, ErrorInvalidHashLength
	}
	return data, nil
}
