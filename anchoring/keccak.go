package anchoring

import (
	"encoding/binary"
	"math/bits"
)

// Keccak-256 (original Keccak padding, domain byte 0x01 — NOT FIPS SHA3-256,
// which uses 0x06). Implemented from the Keccak reference: Keccak-f[1600]
// permutation, rate 1088 bits (136 bytes), capacity 512 bits, 32-byte output.
// Ethereum transaction hashes, addresses, and calldata conventions all use
// this variant.

var keccakRoundConstants = [24]uint64{
	0x0000000000000001, 0x0000000000008082, 0x800000000000808a, 0x8000000080008000,
	0x000000000000808b, 0x0000000080000001, 0x8000000080008081, 0x8000000000008009,
	0x000000000000008a, 0x0000000000000088, 0x0000000080008009, 0x000000008000000a,
	0x000000008000808b, 0x800000000000008b, 0x8000000000008089, 0x8000000000008003,
	0x8000000000008002, 0x8000000000000080, 0x000000000000800a, 0x800000008000000a,
	0x8000000080008081, 0x8000000000008080, 0x0000000080000001, 0x8000000080008008,
}

var keccakRotation = [25]int{
	0, 1, 62, 28, 27,
	36, 44, 6, 55, 20,
	3, 10, 43, 25, 39,
	41, 45, 15, 21, 8,
	18, 2, 61, 56, 14,
}

// keccakPi is the direct substitution table for the Pi step with lane
// index i = x + 5*y: lane (x, y) moves to (y, (2x+3y) mod 5), i.e. target
// index y + 5*((2x+3y) mod 5).
var keccakPi = [25]int{
	0, 10, 20, 5, 15,
	16, 1, 11, 21, 6,
	7, 17, 2, 12, 22,
	23, 8, 18, 3, 13,
	14, 24, 9, 19, 4,
}

func keccakF1600(a *[25]uint64) {
	for round := 0; round < 24; round++ {
		// Theta
		var c [5]uint64
		for x := 0; x < 5; x++ {
			c[x] = a[x] ^ a[x+5] ^ a[x+10] ^ a[x+15] ^ a[x+20]
		}
		for x := 0; x < 5; x++ {
			d := c[(x+4)%5] ^ bits.RotateLeft64(c[(x+1)%5], 1)
			for y := 0; y < 25; y += 5 {
				a[y+x] ^= d
			}
		}
		// Rho + Pi
		var b [25]uint64
		for i := 0; i < 25; i++ {
			b[keccakPi[i]] = bits.RotateLeft64(a[i], keccakRotation[i])
		}
		// Chi
		for y := 0; y < 25; y += 5 {
			for x := 0; x < 5; x++ {
				a[y+x] = b[y+x] ^ (^b[y+(x+1)%5] & b[y+(x+2)%5])
			}
		}
		// Iota
		a[0] ^= keccakRoundConstants[round]
	}
}

// Keccak256 returns the Keccak-256 digest of data.
func Keccak256(data []byte) [32]byte {
	const rate = 136 // bytes
	var state [25]uint64

	// Absorb full blocks.
	for len(data) >= rate {
		for i := 0; i < rate/8; i++ {
			state[i] ^= binary.LittleEndian.Uint64(data[i*8:])
		}
		keccakF1600(&state)
		data = data[rate:]
	}
	// Final block with pad10*1, Keccak domain byte 0x01, last-bit set.
	var block [136]byte
	copy(block[:], data)
	block[len(data)] = 0x01
	block[rate-1] |= 0x80
	for i := 0; i < rate/8; i++ {
		state[i] ^= binary.LittleEndian.Uint64(block[i*8:])
	}
	keccakF1600(&state)

	var out [32]byte
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint64(out[i*8:], state[i])
	}
	return out
}
