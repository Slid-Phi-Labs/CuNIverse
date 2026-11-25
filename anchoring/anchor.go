package anchoring

import (
	"fmt"
	"math/big"
	"time"
)

// IntrinsicGas computes the intrinsic gas of a calldata-carrying
// transaction to an externally owned account: 21000 + 16 per nonzero
// calldata byte + 4 per zero byte.
func IntrinsicGas(calldata []byte) int64 {
	gas := int64(21_000)
	for _, b := range calldata {
		if b == 0 {
			gas += 4
		} else {
			gas += 16
		}
	}
	return gas
}

// BuiltTx is a signed anchor transaction ready to broadcast.
type BuiltTx struct {
	Raw      []byte
	TxHash   string
	From     string
	ChainID  int64
	Nonce    int64
	GasLimit int64
	MaxFee   *big.Int
	Priority *big.Int
	Calldata []byte
}

// BuildBatchAnchorTx assembles and signs the anchor transaction for a batch
// using live chain data (nonce, base fee, priority fee, gas estimate) —
// without broadcasting it. Used by AnchorBatch and by dry runs.
func BuildBatchAnchorTx(client *RPCClient, key PrivateKey, batch *Batch) (*BuiltTx, error) {
	root, err := HexDecode32(batch.MerkleRoot)
	if err != nil {
		return nil, fmt.Errorf("anchor: bad batch root: %w", err)
	}
	chainID, err := client.ChainID()
	if err != nil {
		return nil, fmt.Errorf("anchor: %w", err)
	}
	from := key.Address()
	nonce, err := client.TransactionCount(from)
	if err != nil {
		return nil, fmt.Errorf("anchor: nonce: %w", err)
	}
	baseFee, err := client.LatestBaseFee()
	if err != nil {
		return nil, fmt.Errorf("anchor: base fee: %w", err)
	}
	priority, err := client.MaxPriorityFeePerGas()
	if err != nil {
		return nil, fmt.Errorf("anchor: priority fee: %w", err)
	}
	maxFee := new(big.Int).Lsh(baseFee, 1)
	maxFee.Add(maxFee, priority)

	calldata := AnchorCalldata(batch.BatchID, root)
	gasLimit := IntrinsicGas(calldata) * 15 / 10 // computed floor + 50%
	est, err := client.EstimateGas(map[string]any{
		"from": from,
		"to":   from, // 0-value self-call; the calldata is the payload
		"data": HexEncodeBytes(calldata),
	})
	if err == nil && est > 0 {
		gasLimit = est * 12 / 10 // estimate + 20%
	}

	rawTx, txHash, err := BuildAnchorTx(key, chainID, nonce, maxFee, priority, gasLimit, calldata)
	if err != nil {
		return nil, err
	}
	return &BuiltTx{
		Raw: rawTx, TxHash: txHash, From: from, ChainID: chainID,
		Nonce: nonce, GasLimit: gasLimit, MaxFee: maxFee, Priority: priority,
		Calldata: calldata,
	}, nil
}

// AnchorBatch sends the batch's anchor transaction and records the result
// (tx hash, block number, block timestamp read back from the chain) into
// the batch record. The signing key is supplied by the caller — the library
// never reads key material from disk, environment, or source.
func AnchorBatch(client *RPCClient, key PrivateKey, batch *Batch) (*AnchorInfo, error) {
	from := key.Address()
	balance, err := client.Balance(from)
	if err != nil {
		return nil, fmt.Errorf("anchor: balance: %w", err)
	}
	if balance.Sign() <= 0 {
		return nil, fmt.Errorf("anchor: signer %s has zero balance — fund it before anchoring", from)
	}
	built, err := BuildBatchAnchorTx(client, key, batch)
	if err != nil {
		return nil, err
	}
	chainID := built.ChainID
	rawTx, txHash := built.Raw, built.TxHash
	sentHash, err := client.SendRawTransaction(rawTx)
	if err != nil {
		return nil, fmt.Errorf("anchor: broadcast: %w", err)
	}
	if sentHash != txHash {
		// Trust the network's hash either way; record what it returned.
		txHash = sentHash
	}

	// Wait for the receipt, then read the block timestamp back from the chain.
	var receipt *rpcReceipt
	for i := 0; i < 40; i++ {
		receipt, err = client.GetTransactionReceipt(txHash)
		if err != nil {
			return nil, fmt.Errorf("anchor: receipt: %w", err)
		}
		if receipt != nil {
			break
		}
		time.Sleep(3 * time.Second)
	}
	if receipt == nil {
		return nil, fmt.Errorf("anchor: transaction %s not mined within timeout", txHash)
	}
	if receipt.Status != "0x1" {
		return nil, fmt.Errorf("anchor: transaction %s failed on-chain (status %s)", txHash, receipt.Status)
	}
	blockNumber, err := parseHexInt64(receipt.BlockNumber)
	if err != nil {
		return nil, err
	}
	gasUsed, err := parseHexInt64(receipt.GasUsed)
	if err != nil {
		return nil, err
	}
	ts, err := client.BlockTimestamp(blockNumber)
	if err != nil {
		return nil, fmt.Errorf("anchor: block timestamp: %w", err)
	}
	info := &AnchorInfo{
		ChainID:        chainID,
		Chain:          ChainName(chainID),
		TxHash:         txHash,
		From:           from,
		BlockNumber:    blockNumber,
		BlockTimestamp: ts,
		GasUsed:        gasUsed,
	}
	batch.Anchor = info
	return info, nil
}

// BuildAnchorTx builds and signs an EIP-1559 (type 2) transaction sending
// calldata to the signer's own address with value 0. Returns the raw
// transaction bytes and its hash (0x-hex).
func BuildAnchorTx(key PrivateKey, chainID, nonce int64, maxFeePerGas, maxPriorityFeePerGas *big.Int, gasLimit int64, calldata []byte) ([]byte, string, error) {
	from := key.Address()
	toBytes, err := HexDecode(from)
	if err != nil {
		return nil, "", err
	}
	unsigned := []any{
		big.NewInt(chainID),
		big.NewInt(nonce),
		maxPriorityFeePerGas,
		maxFeePerGas,
		big.NewInt(gasLimit),
		toBytes,
		big.NewInt(0), // value
		calldata,
		[]any{}, // access list
	}
	payload := append([]byte{0x02}, RLPEncode(unsigned)...)
	sighash := Keccak256(payload)
	r, s, recid, err := key.Sign(sighash)
	if err != nil {
		return nil, "", err
	}
	signed := append(append([]any{}, unsigned...), big.NewInt(int64(recid)), r, s)
	raw := append([]byte{0x02}, RLPEncode(signed)...)
	txHash := Keccak256(raw)
	return raw, HexEncodeBytes(txHash[:]), nil
}

// ApplyAnchor copies anchor info into every proof of a batch.
func ApplyAnchor(batch *Batch, proofs []*Proof) {
	for _, p := range proofs {
		p.Anchor = batch.Anchor
	}
}
