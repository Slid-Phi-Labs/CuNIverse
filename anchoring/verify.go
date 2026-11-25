package anchoring

import (
	"fmt"
	"strings"
)

// VerifyResult is the outcome of a full third-party verification.
type VerifyResult struct {
	ReceiptDigest string
	Root          string
	BatchID       string
	ChainID       int64
	TxHash        string
	BlockNumber   int64
	// BlockTimestamp is read from the chain during verification (not taken
	// from the proof), so it is the independently confirmed timestamp.
	BlockTimestamp int64
	AnchorFrom     string
}

// VerifyReceipt performs the complete offline-of-the-operator check:
//
//  1. Recompute the receipt's canonical digest from the receipt itself.
//  2. Cross-check the receipt's own source_hash against the proof (if any).
//  3. Walk the Merkle path to the batch root.
//  4. Fetch the anchor transaction from a public chain RPC and confirm its
//     calldata carries this batch's key and the same root.
//  5. Read the anchor block's timestamp from the chain.
//
// It talks to a public chain RPC only — never to a Slid Phi Labs server.
// A nil error means every check passed; the error names the failing check.
func VerifyReceipt(client *RPCClient, receiptJSON []byte, proof *Proof) (*VerifyResult, error) {
	// 1. Receipt digest, recomputed from the receipt itself.
	digest, err := CanonicalReceiptDigest(receiptJSON)
	if err != nil {
		return nil, fmt.Errorf("receipt does not parse: %w", err)
	}
	digestHex := HexEncode(digest)
	if digestHex != proof.ReceiptDigest {
		return nil, fmt.Errorf("receipt digest mismatch: receipt hashes to %s, proof claims %s — the receipt was altered or the proof belongs to another receipt", digestHex, proof.ReceiptDigest)
	}

	// 2. source_hash cross-check (CuNi receipts carry one).
	if proof.SourceHash != "" {
		sh, err := ReceiptSourceHash(receiptJSON)
		if err != nil {
			return nil, err
		}
		if sh != proof.SourceHash {
			return nil, fmt.Errorf("source_hash mismatch: receipt carries %s, proof claims %s", sh, proof.SourceHash)
		}
	}

	// 3. Merkle path.
	root, err := HexDecode32(proof.BatchRoot)
	if err != nil {
		return nil, fmt.Errorf("proof batch_root: %w", err)
	}
	ok, err := VerifyMerkleProof(digest, proof.MerklePath, root)
	if err != nil {
		return nil, fmt.Errorf("merkle path: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("merkle path does not lead to the batch root — proof is wrong or for another batch")
	}

	// 4. On-chain anchor.
	if proof.Anchor == nil || proof.Anchor.TxHash == "" {
		return nil, fmt.Errorf("proof carries no anchor — batch not yet anchored")
	}
	tx, err := client.GetTransaction(proof.Anchor.TxHash)
	if err != nil {
		return nil, fmt.Errorf("fetch anchor tx: %w", err)
	}
	calldata, err := HexDecode(tx.Input)
	if err != nil {
		return nil, fmt.Errorf("anchor tx calldata: %w", err)
	}
	key, anchoredRoot, err := ParseAnchorCalldata(calldata)
	if err != nil {
		return nil, fmt.Errorf("anchor tx calldata: %w", err)
	}
	if key != BatchKey(proof.BatchID) {
		return nil, fmt.Errorf("anchor tx is for a different batch (batch key mismatch)")
	}
	if anchoredRoot != root {
		return nil, fmt.Errorf("anchor tx root %s does not match proof root %s", HexEncode(anchoredRoot), proof.BatchRoot)
	}
	if !strings.EqualFold(tx.From, proof.Anchor.From) {
		return nil, fmt.Errorf("anchor tx sender %s differs from proof's recorded sender %s", tx.From, proof.Anchor.From)
	}

	// 5. Block timestamp from the chain itself.
	receipt, err := client.GetTransactionReceipt(proof.Anchor.TxHash)
	if err != nil {
		return nil, fmt.Errorf("fetch anchor receipt: %w", err)
	}
	if receipt == nil {
		return nil, fmt.Errorf("anchor tx not mined yet")
	}
	blockNumber, err := parseHexInt64(receipt.BlockNumber)
	if err != nil {
		return nil, err
	}
	ts, err := client.BlockTimestamp(blockNumber)
	if err != nil {
		return nil, fmt.Errorf("fetch anchor block: %w", err)
	}
	return &VerifyResult{
		ReceiptDigest:  digestHex,
		Root:           proof.BatchRoot,
		BatchID:        proof.BatchID,
		ChainID:        proof.Anchor.ChainID,
		TxHash:         proof.Anchor.TxHash,
		BlockNumber:    blockNumber,
		BlockTimestamp: ts,
		AnchorFrom:     tx.From,
	}, nil
}
