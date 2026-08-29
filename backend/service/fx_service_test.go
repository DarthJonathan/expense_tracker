package service

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"expense-tracker/backend/dao"
)

type stubFXRateResolver struct {
	rate     float64
	rateDate string
	err      error
	calls    int
}

func (s *stubFXRateResolver) ResolveRate(
	_ context.Context,
	_ string,
	_ string,
	_ string,
) (float64, string, error) {
	s.calls++
	return s.rate, s.rateDate, s.err
}

func TestConvertToBaseAmountUsesHistoricalFXRate(t *testing.T) {
	resolver := &stubFXRateResolver{rate: 1.35, rateDate: "2026-08-08"}
	service := &ExpenseService{FX: resolver}

	baseAmount, rate, rateDate, err := service.convertToBaseAmount(
		context.Background(),
		12345,
		"USD",
		"SGD",
		"2026-08-09",
	)
	if err != nil {
		t.Fatalf("convert amount: %v", err)
	}
	if want := int(math.Round(12345 * 1.35)); baseAmount != want {
		t.Fatalf("base amount: got %d, want %d", baseAmount, want)
	}
	if rate != 1.35 {
		t.Fatalf("rate: got %f, want 1.35", rate)
	}
	if rateDate != "2026-08-08" {
		t.Fatalf("rate date: got %q, want %q", rateDate, "2026-08-08")
	}
	if resolver.calls != 1 {
		t.Fatalf("resolver calls: got %d, want 1", resolver.calls)
	}
}

func TestConvertToBaseAmountDoesNotSilentlyFallbackToOneToOne(t *testing.T) {
	resolver := &stubFXRateResolver{err: errors.New("provider unavailable")}
	service := &ExpenseService{FX: resolver}

	baseAmount, rate, _, err := service.convertToBaseAmount(
		context.Background(),
		10000,
		"USD",
		"SGD",
		"2026-08-09",
	)
	if err == nil {
		t.Fatal("expected conversion error")
	}
	if baseAmount != 0 || rate != 0 {
		t.Fatalf("failed conversion must not return a 1:1 amount: amount=%d rate=%f", baseAmount, rate)
	}
}

func TestNormalizeCurrencyCodeDefaultsToSGD(t *testing.T) {
	currency, err := normalizeCurrencyCode("")
	if err != nil {
		t.Fatalf("normalize empty currency: %v", err)
	}
	if currency != "SGD" {
		t.Fatalf("currency: got %q, want SGD", currency)
	}
}

func TestPrepareSyncedEntryFXConvertsPendingForeignEntry(t *testing.T) {
	resolver := &stubFXRateResolver{rate: 1.25, rateDate: "2026-08-08"}
	service := &ExpenseService{FX: resolver}
	entry := dao.ExpenseEntry{
		Amount:       1000,
		Currency:     "USD",
		BaseAmount:   0,
		BaseCurrency: "SGD",
		FxRate:       0,
		FxRateDate:   "2026-08-09",
		OccurredOn:   "2026-08-09",
	}

	converted, err := service.prepareSyncedEntryFX(context.Background(), entry, "SGD")
	if err != nil {
		t.Fatalf("prepare synced fx: %v", err)
	}
	if converted.BaseAmount != 1250 || converted.FxRate != 1.25 {
		t.Fatalf("unexpected conversion: amount=%d rate=%f", converted.BaseAmount, converted.FxRate)
	}
	if converted.BaseCurrency != "SGD" || converted.FxRateDate != "2026-08-08" {
		t.Fatalf("unexpected conversion metadata: base=%s date=%s", converted.BaseCurrency, converted.FxRateDate)
	}
}

func TestPrepareSyncedEntryFXSkipsDeletedEntry(t *testing.T) {
	deletedAt := time.Now().UTC()
	resolver := &stubFXRateResolver{err: errors.New("provider unavailable")}
	service := &ExpenseService{FX: resolver}
	entry := dao.ExpenseEntry{
		Amount:       1000,
		Currency:     "IDR",
		BaseAmount:   0,
		BaseCurrency: "SGD",
		FxRate:       0,
		FxRateDate:   "2026-08-09",
		OccurredOn:   "2026-08-09",
		DeletedAt:    &deletedAt,
	}

	deleted, err := service.prepareSyncedEntryFX(context.Background(), entry, "SGD")
	if err != nil {
		t.Fatalf("prepare deleted entry: %v", err)
	}
	if deleted.DeletedAt == nil {
		t.Fatal("deleted marker was lost")
	}
	if resolver.calls != 0 {
		t.Fatalf("deleted entry should not resolve FX; calls=%d", resolver.calls)
	}
}
