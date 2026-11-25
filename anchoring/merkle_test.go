package anchoring

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func leafOf(s string) [32]byte { return sha256.Sum256([]byte(s)) }

func TestMerkleRoots(t *testing.T) {
	a, b, c := leafOf("a"), leafOf("b"), leafOf("c")

	root, err := MerkleRoot([][32]byte{a})
	if err != nil || root != a {
		t.Errorf("single-leaf root = %x, want the leaf itself", root)
	}

	want2 := sha256.Sum256(append(a[:], b[:]...))
	root, err = MerkleRoot([][32]byte{a, b})
	if err != nil || root != want2 {
		t.Errorf("two-leaf root = %x, want %x", root, want2)
	}

	// Odd count: c duplicates.
	cc := sha256.Sum256(append(c[:], c[:]...))
	ab := want2
	want3 := sha256.Sum256(append(ab[:], cc[:]...))
	root, err = MerkleRoot([][32]byte{a, b, c})
	if err != nil || root != want3 {
		t.Errorf("three-leaf root = %x, want %x", root, want3)
	}

	if _, err := MerkleRoot(nil); err == nil {
		t.Error("empty leaf set: want error")
	}
}

func TestMerkleProofRoundtrip(t *testing.T) {
	for n := 1; n <= 20; n++ {
		leaves := make([][32]byte, n)
		for i := range leaves {
			leaves[i] = leafOf(fmt.Sprintf("leaf-%d-%d", n, i))
		}
		root, err := MerkleRoot(leaves)
		if err != nil {
			t.Fatal(err)
		}
		for i := range leaves {
			path, err := MerkleProof(leaves, i)
			if err != nil {
				t.Fatalf("n=%d i=%d: %v", n, i, err)
			}
			ok, err := VerifyMerkleProof(leaves[i], path, root)
			if err != nil || !ok {
				t.Errorf("n=%d i=%d: proof failed (ok=%v, err=%v)", n, i, ok, err)
			}
		}
	}
}

func TestMerkleProofTamper(t *testing.T) {
	leaves := [][32]byte{leafOf("a"), leafOf("b"), leafOf("c"), leafOf("d")}
	root, _ := MerkleRoot(leaves)
	path, _ := MerkleProof(leaves, 1)

	// Wrong leaf.
	if ok, _ := VerifyMerkleProof(leafOf("forged"), path, root); ok {
		t.Error("forged leaf verified")
	}
	// Wrong root.
	if ok, _ := VerifyMerkleProof(leaves[1], path, leafOf("other-root")); ok {
		t.Error("proof verified against wrong root")
	}
	// Corrupted sibling.
	bad := make([]PathStep, len(path))
	copy(bad, path)
	bad[0].Hash = HexEncode(leafOf("evil-sibling"))
	if ok, _ := VerifyMerkleProof(leaves[1], bad, root); ok {
		t.Error("proof with corrupted sibling verified")
	}
	// Flipped side.
	flip := make([]PathStep, len(path))
	copy(flip, path)
	if flip[0].Side == "left" {
		flip[0].Side = "right"
	} else {
		flip[0].Side = "left"
	}
	if ok, _ := VerifyMerkleProof(leaves[1], flip, root); ok {
		t.Error("proof with flipped sibling side verified")
	}
}
