package anchoring

import (
	"math/big"
	"strings"
	"testing"
)

func TestKeccakVectors(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"},
		{"abc", "4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45"},
		{"The quick brown fox jumps over the lazy dog", "4d741b6f1eb29cb2a9b9911c82f56fa8d73b04959d3d9d222895df6c0b28aa15"},
		// Multi-block input (>136 bytes) to exercise absorption.
		{strings.Repeat("a", 200), "96ea54061def936c4be90b518992fdc6f12f535068a256229aca54267b4d084d"},
	}
	for _, c := range cases {
		got := Keccak256([]byte(c.in))
		if HexEncode(got) != c.want {
			t.Errorf("keccak256(%q) = %s, want %s", c.in, HexEncode(got), c.want)
		}
	}
}

func TestAddressVectors(t *testing.T) {
	// Private key 0x01.
	var k1 PrivateKey
	k1[31] = 1
	if got := k1.Address(); got != "0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf" {
		t.Errorf("address(privkey 1) = %s", got)
	}
	// Well-known development key (Hardhat account #0) — test vector only.
	raw, err := HexDecode("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	var k2 PrivateKey
	copy(k2[:], raw)
	if got := k2.Address(); got != "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266" {
		t.Errorf("address(hardhat0) = %s", got)
	}
}

func TestSignRecoverRoundtrip(t *testing.T) {
	var key PrivateKey
	key[31] = 0x42
	hash := Keccak256([]byte("cuni-anchor test"))
	r, s, recid, err := key.Sign(hash)
	if err != nil {
		t.Fatal(err)
	}
	// Low-s invariant.
	if s.Cmp(new(big.Int).Rsh(secpN, 1)) > 0 {
		t.Error("s is not low-s normalized")
	}
	addr, err := RecoverAddress(hash, r, s, recid)
	if err != nil {
		t.Fatal(err)
	}
	if addr != key.Address() {
		t.Errorf("recovered %s, want %s", addr, key.Address())
	}
	// RFC 6979: signing the same hash twice is deterministic.
	r2, s2, _, err := key.Sign(hash)
	if err != nil {
		t.Fatal(err)
	}
	if r.Cmp(r2) != 0 || s.Cmp(s2) != 0 {
		t.Error("signing not deterministic")
	}
}

func TestRLPVectors(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{[]byte{}, "0x80"},
		{[]byte("dog"), "0x83646f67"},
		{big.NewInt(0), "0x80"},
		{big.NewInt(15), "0x0f"},
		{big.NewInt(1024), "0x820400"},
		{[]any{}, "0xc0"},
		{[]any{[]byte("cat"), []byte("dog")}, "0xc88363617483646f67"},
	}
	for _, c := range cases {
		if got := HexEncodeBytes(RLPEncode(c.in)); got != c.want {
			t.Errorf("rlp(%v) = %s, want %s", c.in, got, c.want)
		}
	}
}
