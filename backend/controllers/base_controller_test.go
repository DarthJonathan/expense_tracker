package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeStrictJSONRejectsRawStatementFields(t *testing.T) {
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"accountId":"a","pdf":"data:application/pdf;base64,secret"}`))
	var payload struct {
		AccountID string `json:"accountId"`
	}
	controller := &BaseController{}
	if err := controller.decodeStrictJSON(&payload, request); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown PDF field to be rejected, got %v", err)
	}
}

func TestDecodeStrictJSONAcceptsOneKnownObject(t *testing.T) {
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"accountId":"a"}`))
	var payload struct {
		AccountID string `json:"accountId"`
	}
	controller := &BaseController{}
	if err := controller.decodeStrictJSON(&payload, request); err != nil {
		t.Fatalf("decode known object: %v", err)
	}
	if payload.AccountID != "a" {
		t.Fatalf("account id = %q", payload.AccountID)
	}
}

func TestStatementValidationErrorsUseClientStatuses(t *testing.T) {
	tests := []struct {
		message string
		want    int
	}{
		{message: "at most 5000 statement rows are allowed", want: http.StatusBadRequest},
		{message: "sourceRowKey is duplicated", want: http.StatusBadRequest},
		{message: "category type mismatch", want: http.StatusBadRequest},
		{message: "matched transaction is already assigned to another row", want: http.StatusConflict},
	}
	for _, test := range tests {
		if got := errorStatus(fmt.Errorf("%s", test.message)); got != test.want {
			t.Errorf("errorStatus(%q) = %d, want %d", test.message, got, test.want)
		}
	}
}
