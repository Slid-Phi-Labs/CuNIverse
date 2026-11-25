package anchoring

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sampleReceiptA = `{
  "path": "games/casino-baccarat-settle.exactodds",
  "source_hash": "44bf488f09610fad0e535330dddc4dc066fee4e4ea5ff591c201c187c8f1d004",
  "exact": true,
  "summary": "exactness: PASS (5 langs)",
  "langs": 5,
  "seats": [{"id":"py","seat":"native","emit":true,"run":true}]
}`

const sampleReceiptB = `{"path":"games/dice.exactodds","source_hash":"aa10","exact":true,"summary":"exactness: PASS (5 langs)","langs":5,"seats":[]}`

// fakeChain serves the three JSON-RPC reads the verifier makes.
func fakeChain(t *testing.T, calldata []byte, from string, blockTs int64) *RPCClient {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		var result any
		switch req.Method {
		case "eth_getTransactionByHash":
			result = map[string]any{
				"hash":  "0xdeadbeef",
				"input": HexEncodeBytes(calldata),
				"from":  from,
				"to":    from,
				"value": "0x0",
			}
		case "eth_getTransactionReceipt":
			result = map[string]any{
				"blockNumber": "0x3039", // 12345
				"gasUsed":     "0x5a9d",
				"status":      "0x1",
			}
		case "eth_getBlockByNumber":
			result = map[string]any{"timestamp": fmt.Sprintf("0x%x", blockTs)}
		default:
			t.Errorf("unexpected RPC method %s", req.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return NewRPCClient(srv.URL)
}

func anchoredProof(t *testing.T, root [32]byte, batchID, from string) (*Batch, []*Proof) {
	t.Helper()
	batch, proofs, err := BuildBatch(batchID,
		[]string{"a.receipt.json", "b.receipt.json"},
		[][]byte{[]byte(sampleReceiptA), []byte(sampleReceiptB)})
	if err != nil {
		t.Fatal(err)
	}
	batch.Anchor = &AnchorInfo{
		ChainID:        84532,
		Chain:          "base-sepolia",
		TxHash:         "0xdeadbeef",
		From:           from,
		BlockNumber:    12345,
		BlockTimestamp: 1792700000,
	}
	ApplyAnchor(batch, proofs)
	return batch, proofs
}

func TestCanonicalDigestFormattingIndependent(t *testing.T) {
	// Same receipt, different pretty-printing and key order → same digest.
	reformatted := `{"seats":[{"emit":true,"id":"py","run":true,"seat":"native"}],"langs":5,"summary":"exactness: PASS (5 langs)","exact":true,"source_hash":"44bf488f09610fad0e535330dddc4dc066fee4e4ea5ff591c201c187c8f1d004","path":"games/casino-baccarat-settle.exactodds"}`
	d1, err := CanonicalReceiptDigest([]byte(sampleReceiptA))
	if err != nil {
		t.Fatal(err)
	}
	d2, err := CanonicalReceiptDigest([]byte(reformatted))
	if err != nil {
		t.Fatal(err)
	}
	if d1 != d2 {
		t.Errorf("canonical digest depends on formatting:\n%x\n%x", d1, d2)
	}
}

func TestVerifyHappyPathOfflineOfOperator(t *testing.T) {
	var key PrivateKey
	key[31] = 7
	from := key.Address()
	_, proofs := anchoredProof(t, [32]byte{}, "test-batch-1", from)
	root, _ := HexDecode32(proofs[0].BatchRoot)
	client := fakeChain(t, AnchorCalldata("test-batch-1", root), from, 1792700000)

	res, err := VerifyReceipt(client, []byte(sampleReceiptA), proofs[0])
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if res.BlockTimestamp != 1792700000 {
		t.Errorf("timestamp = %d, want 1792700000 (from the chain, not the proof)", res.BlockTimestamp)
	}
	if res.BlockNumber != 12345 {
		t.Errorf("block = %d, want 12345", res.BlockNumber)
	}
	// Second receipt in the same batch verifies too.
	if _, err := VerifyReceipt(client, []byte(sampleReceiptB), proofs[1]); err != nil {
		t.Fatalf("verify second receipt: %v", err)
	}
}

func TestVerifyTamperedReceiptFails(t *testing.T) {
	var key PrivateKey
	key[31] = 7
	from := key.Address()
	_, proofs := anchoredProof(t, [32]byte{}, "test-batch-1", from)
	root, _ := HexDecode32(proofs[0].BatchRoot)
	client := fakeChain(t, AnchorCalldata("test-batch-1", root), from, 1792700000)

	// Flip the verdict.
	tampered := strings.Replace(sampleReceiptA, `"exact": true`, `"exact": false`, 1)
	if _, err := VerifyReceipt(client, []byte(tampered), proofs[0]); err == nil {
		t.Error("tampered receipt (verdict flipped) verified — must fail")
	} else if !strings.Contains(err.Error(), "digest mismatch") {
		t.Errorf("tampered receipt failed for the wrong reason: %v", err)
	}
	// Edit the summary text.
	tampered2 := strings.Replace(sampleReceiptA, "PASS", "FAKE", 1)
	if _, err := VerifyReceipt(client, []byte(tampered2), proofs[0]); err == nil {
		t.Error("tampered receipt (summary edited) verified — must fail")
	}
	// A receipt from outside the batch presented with this proof.
	outsider := `{"path":"games/other.exactodds","source_hash":"ff","exact":true,"summary":"x","langs":5,"seats":[]}`
	if _, err := VerifyReceipt(client, []byte(outsider), proofs[0]); err == nil {
		t.Error("foreign receipt verified against this proof — must fail")
	}
}

func TestVerifyChainMismatchFails(t *testing.T) {
	var key PrivateKey
	key[31] = 7
	from := key.Address()
	_, proofs := anchoredProof(t, [32]byte{}, "test-batch-1", from)

	// Chain carries a DIFFERENT root for this batch.
	otherRoot := leafOf("some-other-batch")
	client := fakeChain(t, AnchorCalldata("test-batch-1", otherRoot), from, 1792700000)
	if _, err := VerifyReceipt(client, []byte(sampleReceiptA), proofs[0]); err == nil {
		t.Error("verify passed against a tx anchoring a different root — must fail")
	}

	// Chain carries this root but under a DIFFERENT batch id.
	root, _ := HexDecode32(proofs[0].BatchRoot)
	client2 := fakeChain(t, AnchorCalldata("someone-elses-batch", root), from, 1792700000)
	if _, err := VerifyReceipt(client2, []byte(sampleReceiptA), proofs[0]); err == nil {
		t.Error("verify passed against a tx for a different batch — must fail")
	}
}

func TestCalldataLayout(t *testing.T) {
	root := leafOf("root")
	data := AnchorCalldata("batch-xyz", root)
	if len(data) != CalldataBytes {
		t.Fatalf("calldata length = %d, want %d", len(data), CalldataBytes)
	}
	key, gotRoot, err := ParseAnchorCalldata(data)
	if err != nil {
		t.Fatal(err)
	}
	if key != BatchKey("batch-xyz") || gotRoot != root {
		t.Error("calldata roundtrip mismatch")
	}
	if _, _, err := ParseAnchorCalldata([]byte("XXXX" + string(data[4:]))); err == nil {
		t.Error("bad magic accepted")
	}
}
