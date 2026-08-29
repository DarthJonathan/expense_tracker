package request

import (
	"encoding/json"
	"testing"
)

func TestCreateAutomationEntryRequestAcceptsCurrency(t *testing.T) {
	payload := []byte(`{
		"createdAt":"2026-08-09T12:30:00+08:00",
		"accountType":"card",
		"merchant":"Example merchant",
		"amount":"12.34",
		"currency":"usd",
		"device":"iPhone"
	}`)

	var request CreateAutomationEntryRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("decode automation request: %v", err)
	}
	if request.Currency != "usd" {
		t.Fatalf("currency: got %q, want usd", request.Currency)
	}
}
