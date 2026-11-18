package anchoring

import (
	"bytes"
	"math/big"
)

// Minimal RLP (Recursive Length Prefix) encoding — just what Ethereum
// transaction serialization needs: byte strings, unsigned integers, lists.

// RLPEncode encodes item, which must be []byte, *big.Int, or []any of those.
func RLPEncode(item any) []byte {
	switch v := item.(type) {
	case []byte:
		return rlpEncodeString(v)
	case *big.Int:
		return rlpEncodeString(bigIntBytes(v))
	case []any:
		var payload bytes.Buffer
		for _, el := range v {
			payload.Write(RLPEncode(el))
		}
		return append(rlpLengthPrefix(uint64(payload.Len()), 0xc0), payload.Bytes()...)
	default:
		panic("rlp: unsupported type")
	}
}

func bigIntBytes(n *big.Int) []byte {
	if n.Sign() == 0 {
		return []byte{}
	}
	return n.Bytes()
}

func rlpEncodeString(b []byte) []byte {
	if len(b) == 1 && b[0] < 0x80 {
		return []byte{b[0]}
	}
	return append(rlpLengthPrefix(uint64(len(b)), 0x80), b...)
}

func rlpLengthPrefix(length uint64, offset byte) []byte {
	if length < 56 {
		return []byte{offset + byte(length)}
	}
	// length-of-length form
	var lb []byte
	for l := length; l > 0; l >>= 8 {
		lb = append([]byte{byte(l)}, lb...)
	}
	out := []byte{offset + 55 + byte(len(lb))}
	return append(out, lb...)
}
