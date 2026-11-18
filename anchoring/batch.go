package anchoring

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// On-chain anchor format (v1): a 0-value transaction whose calldata is
//
//	"CUNI" (4 bytes) || batchKey (32 bytes) || merkleRoot (32 bytes)
//
// where batchKey = SHA-256("cuni-anchor/v1:" || batch_id). Fixed 68-byte
// layout, so any verifier can parse it without an ABI or a contract.
const (
	AnchorMagic   = "CUNI"
	anchorDomain  = "cuni-anchor/v1:"
	CalldataBytes = 68
)

// BatchKey derives the on-chain batch identifier from the human batch id.
func BatchKey(batchID string) [32]byte {
	return sha256.Sum256([]byte(anchorDomain + batchID))
}

// AnchorCalldata builds the 68-byte calldata for a batch.
func AnchorCalldata(batchID string, root [32]byte) []byte {
	key := BatchKey(batchID)
	out := make([]byte, 0, CalldataBytes)
	out = append(out, AnchorMagic...)
	out = append(out, key[:]...)
	out = append(out, root[:]...)
	return out
}

// ParseAnchorCalldata extracts (batchKey, root) from calldata, checking the
// magic prefix and length.
func ParseAnchorCalldata(data []byte) (key, root [32]byte, err error) {
	if len(data) != CalldataBytes {
		return key, root, fmt.Errorf("calldata: want %d bytes, got %d", CalldataBytes, len(data))
	}
	if string(data[:4]) != AnchorMagic {
		return key, root, fmt.Errorf("calldata: bad magic %q", string(data[:4]))
	}
	copy(key[:], data[4:36])
	copy(root[:], data[36:68])
	return key, root, nil
}

// AnchorInfo records where a batch root landed on-chain.
type AnchorInfo struct {
	ChainID        int64  `json:"chain_id"`
	Chain          string `json:"chain"`
	TxHash         string `json:"tx_hash"`
	From           string `json:"from"`
	BlockNumber    int64  `json:"block_number"`
	BlockTimestamp int64  `json:"block_timestamp"` // unix seconds, read back from the chain
	GasUsed        int64  `json:"gas_used,omitempty"`
}

// LeafEntry is one receipt's slot in a batch.
type LeafEntry struct {
	Index         int    `json:"index"`
	ReceiptName   string `json:"receipt_name"`
	ReceiptDigest string `json:"receipt_digest"` // canonical SHA-256, hex
	SourceHash    string `json:"source_hash,omitempty"`
}

// Batch is the full batch record written next to the proofs.
type Batch struct {
	BatchID    string      `json:"batch_id"`
	CreatedAt  string      `json:"created_at"` // RFC 3339, informational only
	LeafCount  int         `json:"leaf_count"`
	MerkleRoot string      `json:"merkle_root"`
	Leaves     []LeafEntry `json:"leaves"`
	Anchor     *AnchorInfo `json:"anchor,omitempty"`
}

// Proof is everything a third party needs to verify one receipt against the
// chain, without any Slid Phi Labs server.
type Proof struct {
	Version       int         `json:"version"`
	ReceiptName   string      `json:"receipt_name"`
	ReceiptDigest string      `json:"receipt_digest"`
	SourceHash    string      `json:"source_hash,omitempty"`
	LeafIndex     int         `json:"leaf_index"`
	LeafCount     int         `json:"leaf_count"`
	MerklePath    []PathStep  `json:"merkle_path"`
	BatchID       string      `json:"batch_id"`
	BatchRoot     string      `json:"batch_root"`
	Anchor        *AnchorInfo `json:"anchor,omitempty"`
}

// BuildBatch digests receipts and builds the tree + proofs. receipts are raw
// JSON documents; names are labels (usually file names).
func BuildBatch(batchID string, names []string, receipts [][]byte) (*Batch, []*Proof, error) {
	if len(receipts) == 0 {
		return nil, nil, fmt.Errorf("batch: no receipts")
	}
	if len(names) != len(receipts) {
		return nil, nil, fmt.Errorf("batch: %d names for %d receipts", len(names), len(receipts))
	}
	digests := make([][32]byte, len(receipts))
	batch := &Batch{
		BatchID:   batchID,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		LeafCount: len(receipts),
	}
	for i, raw := range receipts {
		d, err := CanonicalReceiptDigest(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("batch: receipt %q: %w", names[i], err)
		}
		digests[i] = d
		sh, err := ReceiptSourceHash(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("batch: receipt %q: %w", names[i], err)
		}
		batch.Leaves = append(batch.Leaves, LeafEntry{
			Index:         i,
			ReceiptName:   names[i],
			ReceiptDigest: HexEncode(d),
			SourceHash:    sh,
		})
	}
	root, err := MerkleRoot(digests)
	if err != nil {
		return nil, nil, err
	}
	batch.MerkleRoot = HexEncode(root)

	proofs := make([]*Proof, len(receipts))
	for i := range receipts {
		path, err := MerkleProof(digests, i)
		if err != nil {
			return nil, nil, err
		}
		proofs[i] = &Proof{
			Version:       1,
			ReceiptName:   names[i],
			ReceiptDigest: batch.Leaves[i].ReceiptDigest,
			SourceHash:    batch.Leaves[i].SourceHash,
			LeafIndex:     i,
			LeafCount:     len(receipts),
			MerklePath:    path,
			BatchID:       batchID,
			BatchRoot:     batch.MerkleRoot,
		}
	}
	return batch, proofs, nil
}
