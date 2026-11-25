package anchoring

import (
	"crypto/sha256"
	"errors"
	"fmt"
)

// Binary SHA-256 Merkle tree over receipt digests.
//
// Conventions (documented for third-party verifiers):
//   - Leaves are the 32-byte receipt digests themselves (no extra leaf
//     hashing — the digest IS the leaf).
//   - Parent = SHA-256(left || right), raw byte concatenation.
//   - A level with an odd node count duplicates its last node.
//   - A single-leaf tree's root is the leaf itself.
//   - Proof paths are ordered leaf-level first; each step names the
//     sibling's side relative to the running node.

// PathStep is one level of a Merkle inclusion proof.
type PathStep struct {
	Side string `json:"side"` // "left" or "right": where the sibling sits
	Hash string `json:"hash"` // hex, 32 bytes
}

// MerkleRoot computes the root of the tree over leaves (hex-decoded digests
// are handled by callers; this takes raw bytes).
func MerkleRoot(leaves [][32]byte) ([32]byte, error) {
	if len(leaves) == 0 {
		return [32]byte{}, errors.New("merkle: empty leaf set")
	}
	level := make([][32]byte, len(leaves))
	copy(level, leaves)
	for len(level) > 1 {
		level = merkleUp(level)
	}
	return level[0], nil
}

func merkleUp(level [][32]byte) [][32]byte {
	next := make([][32]byte, 0, (len(level)+1)/2)
	for i := 0; i < len(level); i += 2 {
		left := level[i]
		right := left // odd node duplicates itself
		if i+1 < len(level) {
			right = level[i+1]
		}
		next = append(next, hashPair(left, right))
	}
	return next
}

func hashPair(left, right [32]byte) [32]byte {
	var buf [64]byte
	copy(buf[:32], left[:])
	copy(buf[32:], right[:])
	return sha256.Sum256(buf[:])
}

// MerkleProof builds the inclusion proof for leaves[index].
func MerkleProof(leaves [][32]byte, index int) ([]PathStep, error) {
	if len(leaves) == 0 {
		return nil, errors.New("merkle: empty leaf set")
	}
	if index < 0 || index >= len(leaves) {
		return nil, fmt.Errorf("merkle: index %d out of range [0,%d)", index, len(leaves))
	}
	var path []PathStep
	level := make([][32]byte, len(leaves))
	copy(level, leaves)
	idx := index
	for len(level) > 1 {
		if idx%2 == 0 {
			sib := level[idx] // self, when this is the duplicated odd node
			if idx+1 < len(level) {
				sib = level[idx+1]
			}
			path = append(path, PathStep{Side: "right", Hash: HexEncode(sib)})
		} else {
			path = append(path, PathStep{Side: "left", Hash: HexEncode(level[idx-1])})
		}
		level = merkleUp(level)
		idx /= 2
	}
	return path, nil
}

// VerifyMerkleProof walks leaf up path and reports whether it lands on root.
func VerifyMerkleProof(leaf [32]byte, path []PathStep, root [32]byte) (bool, error) {
	node := leaf
	for _, step := range path {
		sib, err := HexDecode32(step.Hash)
		if err != nil {
			return false, fmt.Errorf("merkle: bad sibling hash: %w", err)
		}
		switch step.Side {
		case "left":
			node = hashPair(sib, node)
		case "right":
			node = hashPair(node, sib)
		default:
			return false, fmt.Errorf("merkle: bad side %q", step.Side)
		}
	}
	return node == root, nil
}

// HexEncode renders bytes as lowercase 0x-less hex.
func HexEncode(b [32]byte) string {
	const hexd = "0123456789abcdef"
	out := make([]byte, 64)
	for i, v := range b {
		out[2*i] = hexd[v>>4]
		out[2*i+1] = hexd[v&0xf]
	}
	return string(out)
}

// HexDecode32 parses a 64-char hex string (optional 0x prefix) into 32 bytes.
func HexDecode32(s string) ([32]byte, error) {
	var out [32]byte
	b, err := HexDecode(s)
	if err != nil {
		return out, err
	}
	if len(b) != 32 {
		return out, fmt.Errorf("want 32 bytes, got %d", len(b))
	}
	copy(out[:], b)
	return out, nil
}

// HexDecode parses hex with optional 0x prefix.
func HexDecode(s string) ([]byte, error) {
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		s = s[2:]
	}
	if len(s)%2 != 0 {
		return nil, errors.New("hex: odd length")
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		hi, ok1 := hexVal(s[2*i])
		lo, ok2 := hexVal(s[2*i+1])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("hex: invalid character in %q", s)
		}
		out[i] = hi<<4 | lo
	}
	return out, nil
}

func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// HexEncodeBytes renders arbitrary bytes as 0x-prefixed hex.
func HexEncodeBytes(b []byte) string {
	const hexd = "0123456789abcdef"
	out := []byte("0x")
	for _, v := range b {
		out = append(out, hexd[v>>4], hexd[v&0xf])
	}
	return string(out)
}
