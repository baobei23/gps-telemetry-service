package telemetry

import (
	"testing"
	"time"
)

func TestEncodeDecodeCursor(t *testing.T) {
	origTime := time.Date(2026, 10, 5, 12, 34, 56, 123456000, time.UTC)

	token := EncodeCursor(origTime)
	if token == "" {
		t.Fatal("expected non-empty cursor token")
	}

	decodedTime, err := DecodeCursor(token)
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}

	if !decodedTime.Equal(origTime) {
		t.Errorf("expected time %v, got %v", origTime, decodedTime)
	}
}

func TestDecodeCursor_Errors(t *testing.T) {
	tests := []string{
		"invalid-base64!@#",
		"YWJjX2RlZg==", // "abc_def" non-numeric
		"bm90LW51bWJlcg==", // "not-number"
	}

	for _, token := range tests {
		_, err := DecodeCursor(token)
		if err == nil {
			t.Errorf("expected error for token %q, got nil", token)
		}
	}
}
