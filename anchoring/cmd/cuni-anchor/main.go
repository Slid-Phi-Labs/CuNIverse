// Command cuni-anchor batches CuNi receipts into a Merkle tree, anchors the
// root on Base with a single transaction, and verifies individual receipts
// against the chain — no Slid Phi Labs server involved at verify time.
//
// Subcommands:
//
//	keygen  -out <file>                 generate an ephemeral signer key (0600)
//	digest  <receipt.json>              print a receipt's canonical digest
//	batch   -id <id> -out <dir> <receipt.json>...
//	                                    build batch.json + per-receipt proofs
//	anchor  -batch <dir|batch.json>     sign + broadcast the anchor tx
//	verify  -receipt <f> -proof <f>     full third-party verification
//
// The anchor signer key is read ONLY from the environment variable
// CUNI_ANCHOR_PRIVATE_KEY (hex, optional 0x prefix). It is never taken from
// a flag, a file argument, or source.
package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	anchoring "cuni-anchoring"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = cmdKeygen(os.Args[2:])
	case "digest":
		err = cmdDigest(os.Args[2:])
	case "batch":
		err = cmdBatch(os.Args[2:])
	case "anchor":
		err = cmdAnchor(os.Args[2:])
	case "verify":
		err = cmdVerify(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: cuni-anchor <keygen|digest|batch|anchor|verify> [flags]")
}

func cmdKeygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	out := fs.String("out", "", "file to write the hex private key to (mode 0600)")
	fs.Parse(args)
	if *out == "" {
		return fmt.Errorf("keygen: -out is required")
	}
	var key anchoring.PrivateKey
	if _, err := rand.Read(key[:]); err != nil {
		return err
	}
	hexKey := anchoring.HexEncodeBytes(key[:])
	if err := os.WriteFile(*out, []byte(hexKey+"\n"), 0o600); err != nil {
		return err
	}
	fmt.Printf("address: %s\n", key.Address())
	fmt.Printf("key written to %s (mode 0600)\n", *out)
	fmt.Println("WARNING: treat this file as a secret. For production anchoring use a")
	fmt.Println("dedicated wallet whose key you control; never commit key files.")
	return nil
}

func cmdDigest(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("digest: want exactly one receipt file")
	}
	raw, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	d, err := anchoring.CanonicalReceiptDigest(raw)
	if err != nil {
		return err
	}
	fmt.Printf("receipt_digest: %s\n", anchoring.HexEncode(d))
	if sh, err := anchoring.ReceiptSourceHash(raw); err == nil && sh != "" {
		fmt.Printf("source_hash:    %s\n", sh)
	}
	return nil
}

func cmdBatch(args []string) error {
	fs := flag.NewFlagSet("batch", flag.ExitOnError)
	id := fs.String("id", "", "batch id (human label; the on-chain key derives from it)")
	out := fs.String("out", "", "output directory for batch.json and proofs/")
	fs.Parse(args)
	if *id == "" || *out == "" {
		return fmt.Errorf("batch: -id and -out are required")
	}
	files := fs.Args()
	if len(files) == 0 {
		return fmt.Errorf("batch: no receipt files given")
	}
	names := make([]string, len(files))
	raws := make([][]byte, len(files))
	for i, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		names[i] = filepath.Base(f)
		raws[i] = raw
	}
	batch, proofs, err := anchoring.BuildBatch(*id, names, raws)
	if err != nil {
		return err
	}
	if err := writeBatch(*out, batch, proofs); err != nil {
		return err
	}
	fmt.Printf("batch_id:   %s\n", batch.BatchID)
	fmt.Printf("leaf_count: %d\n", batch.LeafCount)
	fmt.Printf("merkle_root: %s\n", batch.MerkleRoot)
	fmt.Printf("wrote %s and %d proofs\n", filepath.Join(*out, "batch.json"), len(proofs))
	return nil
}

func cmdAnchor(args []string) error {
	fs := flag.NewFlagSet("anchor", flag.ExitOnError)
	batchPath := fs.String("batch", "", "batch.json file (or its directory)")
	rpcURL := fs.String("rpc", "", "RPC URL (default: public Base Sepolia)")
	dryRun := fs.Bool("dry-run", false, "build and sign the anchor tx but do not broadcast it")
	outTx := fs.String("out-tx", "", "with -dry-run: write the signed raw transaction (hex) to this file")
	fs.Parse(args)
	if *batchPath == "" {
		return fmt.Errorf("anchor: -batch is required")
	}
	keyHex := os.Getenv("CUNI_ANCHOR_PRIVATE_KEY")
	if keyHex == "" {
		return fmt.Errorf("anchor: CUNI_ANCHOR_PRIVATE_KEY is not set")
	}
	keyBytes, err := anchoring.HexDecode(strings.TrimSpace(keyHex))
	if err != nil || len(keyBytes) != 32 {
		return fmt.Errorf("anchor: CUNI_ANCHOR_PRIVATE_KEY must be 32 bytes of hex")
	}
	var key anchoring.PrivateKey
	copy(key[:], keyBytes)

	dir, batch, proofs, err := readBatch(*batchPath)
	if err != nil {
		return err
	}
	url := *rpcURL
	if url == "" {
		url = "https://sepolia.base.org"
	}
	client := anchoring.NewRPCClient(url)
	if *dryRun {
		built, err := anchoring.BuildBatchAnchorTx(client, key, batch)
		if err != nil {
			return err
		}
		fmt.Printf("DRY RUN — transaction built and signed, NOT broadcast\n")
		fmt.Printf("from:       %s\n", built.From)
		fmt.Printf("chain_id:   %d\n", built.ChainID)
		fmt.Printf("nonce:      %d\n", built.Nonce)
		fmt.Printf("gas_limit:  %d\n", built.GasLimit)
		fmt.Printf("max_fee:    %s wei\n", built.MaxFee.String())
		fmt.Printf("priority:   %s wei\n", built.Priority.String())
		fmt.Printf("calldata:   %s\n", anchoring.HexEncodeBytes(built.Calldata))
		fmt.Printf("tx_hash:    %s\n", built.TxHash)
		if *outTx != "" {
			raw := anchoring.HexEncodeBytes(built.Raw)
			if err := os.WriteFile(*outTx, []byte(raw+"\n"), 0o644); err != nil {
				return err
			}
			fmt.Printf("raw tx written to %s — broadcast later with any Ethereum tool\n", *outTx)
		}
		return nil
	}
	info, err := anchoring.AnchorBatch(client, key, batch)
	if err != nil {
		return err
	}
	anchoring.ApplyAnchor(batch, proofs)
	if err := writeBatch(dir, batch, proofs); err != nil {
		return err
	}
	fmt.Printf("anchored batch %s\n", batch.BatchID)
	fmt.Printf("chain:        %s (%d)\n", info.Chain, info.ChainID)
	fmt.Printf("tx_hash:      %s\n", info.TxHash)
	fmt.Printf("block:        %d\n", info.BlockNumber)
	fmt.Printf("block_time:   %d\n", info.BlockTimestamp)
	fmt.Printf("gas_used:     %d\n", info.GasUsed)
	return nil
}

func cmdVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	receiptPath := fs.String("receipt", "", "receipt JSON file")
	proofPath := fs.String("proof", "", "proof JSON file")
	rpcURL := fs.String("rpc", "", "RPC URL (default: public RPC for the proof's chain)")
	fs.Parse(args)
	if *receiptPath == "" || *proofPath == "" {
		return fmt.Errorf("verify: -receipt and -proof are required")
	}
	raw, err := os.ReadFile(*receiptPath)
	if err != nil {
		return err
	}
	praw, err := os.ReadFile(*proofPath)
	if err != nil {
		return err
	}
	var proof anchoring.Proof
	if err := json.Unmarshal(praw, &proof); err != nil {
		return fmt.Errorf("parse proof: %w", err)
	}
	url := *rpcURL
	if url == "" {
		if proof.Anchor == nil {
			return fmt.Errorf("verify: proof has no anchor; batch not anchored yet")
		}
		def, ok := anchoring.DefaultRPCURL(proof.Anchor.ChainID)
		if !ok {
			return fmt.Errorf("verify: no default RPC for chain %d; pass -rpc", proof.Anchor.ChainID)
		}
		url = def
	}
	client := anchoring.NewRPCClient(url)
	res, err := anchoring.VerifyReceipt(client, raw, &proof)
	if err != nil {
		fmt.Println("FAIL:", err)
		return fmt.Errorf("verification failed")
	}
	fmt.Println("PASS")
	fmt.Printf("receipt_digest: %s\n", res.ReceiptDigest)
	fmt.Printf("batch_id:       %s\n", res.BatchID)
	fmt.Printf("batch_root:     %s\n", res.Root)
	fmt.Printf("anchor_tx:      %s\n", res.TxHash)
	fmt.Printf("anchor_from:    %s\n", res.AnchorFrom)
	fmt.Printf("block:          %d\n", res.BlockNumber)
	fmt.Printf("block_time:     %d (unix)\n", res.BlockTimestamp)
	fmt.Println("This receipt existed, byte-identical, no later than the block time above.")
	fmt.Println("That proves existence + integrity, not that the game was fair or the result correct.")
	return nil
}

// writeBatch writes batch.json and proofs/<name>.proof.json under dir.
func writeBatch(dir string, batch *anchoring.Batch, proofs []*anchoring.Proof) error {
	if err := os.MkdirAll(filepath.Join(dir, "proofs"), 0o755); err != nil {
		return err
	}
	bj, err := json.MarshalIndent(batch, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "batch.json"), append(bj, '\n'), 0o644); err != nil {
		return err
	}
	for _, p := range proofs {
		pj, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(p.ReceiptName, ".json") + ".proof.json"
		if err := os.WriteFile(filepath.Join(dir, "proofs", name), append(pj, '\n'), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// readBatch loads batch.json (path may be the file or its directory) and all
// proofs under <dir>/proofs.
func readBatch(path string) (string, *anchoring.Batch, []*anchoring.Proof, error) {
	dir := path
	batchFile := path
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		batchFile = filepath.Join(path, "batch.json")
	} else {
		dir = filepath.Dir(path)
	}
	raw, err := os.ReadFile(batchFile)
	if err != nil {
		return "", nil, nil, err
	}
	var batch anchoring.Batch
	if err := json.Unmarshal(raw, &batch); err != nil {
		return "", nil, nil, fmt.Errorf("parse batch: %w", err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "proofs"))
	if err != nil {
		return "", nil, nil, err
	}
	var proofs []*anchoring.Proof
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".proof.json") {
			continue
		}
		praw, err := os.ReadFile(filepath.Join(dir, "proofs", e.Name()))
		if err != nil {
			return "", nil, nil, err
		}
		var p anchoring.Proof
		if err := json.Unmarshal(praw, &p); err != nil {
			return "", nil, nil, fmt.Errorf("parse proof %s: %w", e.Name(), err)
		}
		proofs = append(proofs, &p)
	}
	return dir, &batch, proofs, nil
}
