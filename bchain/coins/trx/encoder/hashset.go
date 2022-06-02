package encoder

import (
	"crypto/sha256"
)

const (
	HASH_ALG_DOUBLE_SHA256 = uint32(0xA000000A)
	HASH_ALG_KECCAK256     = uint32(0xA000000E)
)

func Hash(data []byte, digestLen uint16, typeChoose uint32) []byte {
	switch typeChoose {
	case HASH_ALG_DOUBLE_SHA256:
		h := sha256.New()
		_, err := h.Write(data)
		if err != nil {
			return nil
		}
		tmp := h.Sum(nil)
		h = sha256.New()
		_, err = h.Write(tmp)
		if err != nil {
			return nil
		}
		return h.Sum(nil)
		break

	case HASH_ALG_KECCAK256:
		h := NewKeccak256()
		_, err := h.Write(data)
		if err != nil {
			return nil
		}
		return h.Sum(nil)
		break
	default:
		return nil
	}

	return nil
}
