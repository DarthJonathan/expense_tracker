package dao

import (
	"encoding/json"
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
	serialized, ok := stored.(string)
	if !ok {
		t.Fatalf("serialized metadata type = %T, want string", stored)
	}
	if !json.Valid([]byte(serialized)) {
		t.Fatalf("serialized metadata is not valid JSON: %q", serialized)
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
	if stored.(string) != "{}" {
		t.Fatalf("serialized nil metadata = %q, want {}", stored)
	}
}
