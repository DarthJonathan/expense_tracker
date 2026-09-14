package dao

import (
	"reflect"
	"testing"
)

func TestJSONMapDatabaseRoundTrip(t *testing.T) {
	want := JSONMap{
		"categoryConfidence": 0.2,
		"categorySource":     "fallback",
		"fxMarkupPercent":    3.5,
	}

	stored, err := want.Value()
	if err != nil {
		t.Fatalf("serialize metadata: %v", err)
	}

	var got JSONMap
	if err := got.Scan(stored); err != nil {
		t.Fatalf("deserialize metadata: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("metadata round-trip = %#v, want %#v", got, want)
	}
}

func TestJSONMapNilUsesEmptyObject(t *testing.T) {
	var value JSONMap
	stored, err := value.Value()
	if err != nil {
		t.Fatalf("serialize nil metadata: %v", err)
	}
	if string(stored.([]byte)) != "{}" {
		t.Fatalf("serialized nil metadata = %q, want {}", stored)
	}
}
