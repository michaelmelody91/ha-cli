package cmd

import (
	"testing"
	"time"
)

func TestParseRelativeDuration(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"1h", time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"45s", 45 * time.Second, false},
		{"1h30m", 90 * time.Minute, false},
		{"2d", 2 * 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"invalid", 0, true},
		{"", 0, true},
		{"d", 0, true},
		{"w", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseRelativeDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseRelativeDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseRelativeDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	t.Run("RFC3339 UTC", func(t *testing.T) {
		got, err := parseTimestamp("2024-01-15T10:30:00Z")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC).UnixMicro()
		if got != expected {
			t.Errorf("got %d, want %d", got, expected)
		}
	})

	t.Run("RFC3339 with offset", func(t *testing.T) {
		got, err := parseTimestamp("2024-01-15T12:30:00+02:00")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// +02:00 means 12:30 local = 10:30 UTC
		expected := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC).UnixMicro()
		if got != expected {
			t.Errorf("got %d, want %d", got, expected)
		}
	})

	t.Run("datetime without timezone", func(t *testing.T) {
		got, err := parseTimestamp("2024-01-15T10:30:00")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC).UnixMicro()
		if got != expected {
			t.Errorf("got %d, want %d", got, expected)
		}
	})

	t.Run("date only", func(t *testing.T) {
		got, err := parseTimestamp("2024-01-15")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC).UnixMicro()
		if got != expected {
			t.Errorf("got %d, want %d", got, expected)
		}
	})

	t.Run("relative 1h", func(t *testing.T) {
		before := time.Now()
		got, err := parseTimestamp("1h")
		after := time.Now()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lo := before.Add(-time.Hour).UnixMicro()
		hi := after.Add(-time.Hour).UnixMicro()
		if got < lo || got > hi {
			t.Errorf("got %d, want in [%d, %d]", got, lo, hi)
		}
	})

	t.Run("relative with leading dash -1h", func(t *testing.T) {
		before := time.Now()
		got, err := parseTimestamp("-1h")
		after := time.Now()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lo := before.Add(-time.Hour).UnixMicro()
		hi := after.Add(-time.Hour).UnixMicro()
		if got < lo || got > hi {
			t.Errorf("got %d, want in [%d, %d]", got, lo, hi)
		}
	})

	t.Run("relative 30m", func(t *testing.T) {
		before := time.Now()
		got, err := parseTimestamp("30m")
		after := time.Now()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lo := before.Add(-30 * time.Minute).UnixMicro()
		hi := after.Add(-30 * time.Minute).UnixMicro()
		if got < lo || got > hi {
			t.Errorf("got %d, want in [%d, %d]", got, lo, hi)
		}
	})

	t.Run("relative 2d", func(t *testing.T) {
		before := time.Now()
		got, err := parseTimestamp("2d")
		after := time.Now()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lo := before.Add(-2 * 24 * time.Hour).UnixMicro()
		hi := after.Add(-2 * 24 * time.Hour).UnixMicro()
		if got < lo || got > hi {
			t.Errorf("got %d, want in [%d, %d]", got, lo, hi)
		}
	})

	t.Run("relative 1w", func(t *testing.T) {
		before := time.Now()
		got, err := parseTimestamp("1w")
		after := time.Now()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lo := before.Add(-7 * 24 * time.Hour).UnixMicro()
		hi := after.Add(-7 * 24 * time.Hour).UnixMicro()
		if got < lo || got > hi {
			t.Errorf("got %d, want in [%d, %d]", got, lo, hi)
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		_, err := parseTimestamp("invalid")
		if err == nil {
			t.Error("expected error but got nil")
		}
	})

	t.Run("empty string", func(t *testing.T) {
		_, err := parseTimestamp("")
		if err == nil {
			t.Error("expected error but got nil")
		}
	})

	t.Run("bare dash", func(t *testing.T) {
		_, err := parseTimestamp("-")
		if err == nil {
			t.Error("expected error but got nil")
		}
	})
}
