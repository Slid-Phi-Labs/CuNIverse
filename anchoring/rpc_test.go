package anchoring

import "testing"

func TestParseHexQuantities(t *testing.T) {
	// JSON-RPC quantities have no leading zeros: "0x0" is valid and odd-length.
	for s, want := range map[string]int64{"0x0": 0, "0x1": 1, "0x3039": 12345, "0x5b8d80": 6000000} {
		got, err := parseHexInt64(s)
		if err != nil || got != want {
			t.Errorf("parseHexInt64(%q) = %d, %v; want %d", s, got, err, want)
		}
	}
}
