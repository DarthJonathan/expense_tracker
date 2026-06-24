package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const defaultFXBaseURL = "https://api.frankfurter.app"

type fxCacheKey struct {
	Date string
	From string
	To   string
}

type fxCacheValue struct {
	Rate      float64
	RateDate  string
	CachedAt  time.Time
}

type FXService struct {
	baseURL string
	client  *http.Client

	mu    sync.RWMutex
	cache map[fxCacheKey]fxCacheValue
}

type frankfurterResponse struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
}

func NewFXService(baseURL string) *FXService {
	normalized := strings.TrimSpace(baseURL)
	if normalized == "" {
		normalized = defaultFXBaseURL
	}
	normalized = strings.TrimRight(normalized, "/")

	return &FXService{
		baseURL: normalized,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		cache: make(map[fxCacheKey]fxCacheValue),
	}
}

func (s *FXService) ResolveRate(
	ctx context.Context,
	from string,
	to string,
	occurredOn string,
) (rate float64, rateDate string, err error) {
	fromCode, err := normalizeCurrencyCode(from)
	if err != nil {
		return 0, "", err
	}
	toCode, err := normalizeCurrencyCode(to)
	if err != nil {
		return 0, "", err
	}

	normalizedDate, err := normalizeDate(occurredOn)
	if err != nil {
		return 0, "", err
	}

	if fromCode == toCode {
		return 1.0, normalizedDate, nil
	}

	key := fxCacheKey{
		Date: normalizedDate,
		From: fromCode,
		To:   toCode,
	}

	s.mu.RLock()
	cached, ok := s.cache[key]
	s.mu.RUnlock()
	if ok {
		return cached.Rate, cached.RateDate, nil
	}

	rate, effectiveDate, err := s.fetchRate(ctx, fromCode, toCode, normalizedDate)
	if err != nil {
		return 0, "", err
	}

	s.mu.Lock()
	s.cache[key] = fxCacheValue{
		Rate:     rate,
		RateDate: effectiveDate,
		CachedAt: time.Now().UTC(),
	}
	s.mu.Unlock()

	return rate, effectiveDate, nil
}

func (s *FXService) fetchRate(ctx context.Context, fromCode string, toCode string, date string) (float64, string, error) {
	params := url.Values{}
	params.Set("from", fromCode)
	params.Set("to", toCode)

	endpoint := fmt.Sprintf("%s/%s?%s", s.baseURL, url.PathEscape(date), params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, "", err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, "", fmt.Errorf("fx rate lookup failed: status %d", resp.StatusCode)
	}

	body := &frankfurterResponse{}
	if err := json.NewDecoder(resp.Body).Decode(body); err != nil {
		return 0, "", fmt.Errorf("decode fx response: %w", err)
	}

	rate, ok := body.Rates[toCode]
	if !ok || rate <= 0 {
		return 0, "", fmt.Errorf("fx rate not found for %s->%s on %s", fromCode, toCode, date)
	}

	effectiveDate := strings.TrimSpace(body.Date)
	if effectiveDate == "" {
		effectiveDate = date
	}

	return rate, effectiveDate, nil
}
