package usage

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReadResponseReturnsJSONRPCError(t *testing.T) {
	dec := json.NewDecoder(strings.NewReader(`{"jsonrpc":"2.0","id":1,"error":{"message":"token revoked"}}` + "\n"))
	_, err := readResponse(dec, 1)
	if err == nil || !strings.Contains(err.Error(), "token revoked") {
		t.Fatalf("readResponse error = %v, want token revoked", err)
	}
}

func TestReadResponseSkipsOtherIDs(t *testing.T) {
	dec := json.NewDecoder(strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"result":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"result":{"ok":true}}` + "\n",
	))
	raw, err := readResponse(dec, 2)
	if err != nil {
		t.Fatalf("readResponse error = %v", err)
	}
	if !strings.Contains(string(raw), `"ok":true`) {
		t.Fatalf("readResponse raw = %s", raw)
	}
}
