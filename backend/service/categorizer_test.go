package service

import (
	"testing"

	"expense-tracker/backend/dao"
)

func TestNormalizeMerchantKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strips punctuation and lowercases",
			input: "  FairPrice, Xtra!  ",
			want:  "fairprice xtra",
		},
		{
			name:  "removes common stop words",
			input: "NTUC FAIRPRICE PTE LTD SINGAPORE",
			want:  "ntuc fairprice",
		},
		{
			name:  "keeps unicode letters and digits",
			input: "Café 123 SG",
			want:  "café 123",
		},
		{
			name:  "empty stays empty",
			input: "   ",
			want:  "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeMerchantKey(tt.input)
			if got != tt.want {
				t.Fatalf("normalizeMerchantKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatchesRule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		matchKind string
		pattern   string
		target    string
		want      bool
	}{
		{
			name:      "contains matches substring case-insensitive",
			matchKind: "contains",
			pattern:   "fairprice",
			target:    "NTUC FairPrice Xtra",
			want:      true,
		},
		{
			name:      "prefix matches start case-insensitive",
			matchKind: "prefix",
			pattern:   "grab",
			target:    "GrabPay *1234",
			want:      true,
		},
		{
			name:      "equals matches full string case-insensitive",
			matchKind: "equals",
			pattern:   "salary",
			target:    "SALARY",
			want:      true,
		},
		{
			name:      "regex supports patterns",
			matchKind: "regex",
			pattern:   `(?i)^(gojek|grab)`,
			target:    "Gojek ride",
			want:      true,
		},
		{
			name:      "invalid regex safely returns false",
			matchKind: "regex",
			pattern:   `[`,
			target:    "anything",
			want:      false,
		},
		{
			name:      "non-match returns false",
			matchKind: "contains",
			pattern:   "amazon",
			target:    "ntuc fairprice",
			want:      false,
		},
		{
			name:      "empty pattern returns false",
			matchKind: "contains",
			pattern:   "",
			target:    "merchant",
			want:      false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := matchesRule(tt.matchKind, tt.pattern, tt.target)
			if got != tt.want {
				t.Fatalf("matchesRule(%q, %q, %q) = %v, want %v", tt.matchKind, tt.pattern, tt.target, got, tt.want)
			}
		})
	}
}

func TestFallbackCategoryName(t *testing.T) {
	t.Parallel()

	if got := fallbackCategoryName("expense"); got != "Other expense" {
		t.Fatalf("fallbackCategoryName(expense) = %q, want %q", got, "Other expense")
	}
	if got := fallbackCategoryName("income"); got != "Income" {
		t.Fatalf("fallbackCategoryName(income) = %q, want %q", got, "Income")
	}
}

func TestShouldLearnSyncedEntry(t *testing.T) {
	t.Parallel()

	base := dao.ExpenseEntry{
		Merchant:   "Toast Box",
		CategoryID: "category-id",
		Type:       "expense",
	}

	tests := []struct {
		name     string
		metadata map[string]any
		want     bool
	}{
		{name: "legacy manual entry", metadata: nil, want: true},
		{name: "explicit manual entry", metadata: map[string]any{"categorySource": "manual"}, want: true},
		{name: "automatic fallback", metadata: map[string]any{"categorySource": "fallback"}, want: false},
		{name: "automatic exact suggestion", metadata: map[string]any{"categorySource": "exact"}, want: false},
		{name: "automatic fuzzy suggestion", metadata: map[string]any{"categorySource": "fuzzy"}, want: false},
		{name: "automatic rule suggestion", metadata: map[string]any{"categorySource": "rule"}, want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entry := base
			entry.Metadata = tt.metadata
			if got := shouldLearnSyncedEntry(entry); got != tt.want {
				t.Fatalf("shouldLearnSyncedEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}
