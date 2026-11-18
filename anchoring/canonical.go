package anchoring

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// CanonicalReceiptDigest computes the digest a receipt is anchored under:
// SHA-256 over the receipt's canonical JSON form.
//
// Canonical form (must be reproducible by any third-party verifier):
//   - UTF-8 JSON, object keys sorted bytewise, no insignificant whitespace.
//   - Numbers keep their exact source token (no float re-formatting).
//   - Strings escaped minimally: `"` and `\` are backslash-escaped, control
//     characters below U+0020 are written as \u00xx (lowercase hex); all other
//     characters are emitted as raw UTF-8.
//
// Why not the receipt's own `source_hash` field as the leaf? In CuNi
// receipts, source_hash commits to the *game/law source* — two different
// verdict receipts about the same game share it — so it cannot detect a
// swapped or edited receipt. The leaf digest must commit to the whole
// receipt. The receipt's source_hash (when present) is still carried in
// the proof and cross-checked by the verifier; see Proof.SourceHash.
func CanonicalReceiptDigest(receiptJSON []byte) ([32]byte, error) {
	canon, err := CanonicalJSON(receiptJSON)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(canon), nil
}

// CanonicalJSON re-serializes arbitrary JSON into the canonical form above.
func CanonicalJSON(in []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(in))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("parse receipt JSON: %w", err)
	}
	var buf bytes.Buffer
	if err := writeCanonical(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeCanonical(buf *bytes.Buffer, v any) error {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeCanonicalString(buf, k)
			buf.WriteByte(':')
			if err := writeCanonical(buf, t[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	case []any:
		buf.WriteByte('[')
		for i, el := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, el); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case string:
		writeCanonicalString(buf, t)
	case json.Number:
		buf.WriteString(t.String())
	case bool:
		if t {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case nil:
		buf.WriteString("null")
	default:
		return fmt.Errorf("unsupported JSON value type %T", v)
	}
	return nil
}

func writeCanonicalString(buf *bytes.Buffer, s string) {
	const hexd = "0123456789abcdef"
	buf.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			buf.WriteString(`\"`)
		case r == '\\':
			buf.WriteString(`\\`)
		case r < 0x20:
			buf.WriteString(`\u00`)
			buf.WriteByte(hexd[(r>>4)&0xf])
			buf.WriteByte(hexd[r&0xf])
		default:
			buf.WriteRune(r)
		}
	}
	buf.WriteByte('"')
}

// ReceiptSourceHash extracts the receipt's own digest field (CuNi receipts
// carry `source_hash`) if present. Returns "" when absent.
func ReceiptSourceHash(receiptJSON []byte) (string, error) {
	var m map[string]any
	dec := json.NewDecoder(strings.NewReader(string(receiptJSON)))
	if err := dec.Decode(&m); err != nil {
		return "", fmt.Errorf("parse receipt JSON: %w", err)
	}
	if v, ok := m["source_hash"]; ok {
		if s, ok := v.(string); ok {
			return s, nil
		}
	}
	return "", nil
}
