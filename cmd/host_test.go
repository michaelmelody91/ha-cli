package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

// ---- parseRelativeDuration -------------------------------------------------

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

// ---- parseTimestamp --------------------------------------------------------

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

// ---- parseVerboseTimestamp -------------------------------------------------

func TestParseVerboseTimestamp(t *testing.T) {
	t.Run("valid verbose line", func(t *testing.T) {
		line := "2024-01-15 10:30:00.123 homeassistant supervisor[1]: hello"
		ts, ok := parseVerboseTimestamp(line)
		if !ok {
			t.Fatal("expected ok=true")
		}
		want := time.Date(2024, 1, 15, 10, 30, 0, 123_000_000, time.UTC)
		if !ts.Equal(want) {
			t.Errorf("got %v, want %v", ts, want)
		}
	})

	t.Run("line too short", func(t *testing.T) {
		_, ok := parseVerboseTimestamp("2024-01-15")
		if ok {
			t.Error("expected ok=false for short line")
		}
	})

	t.Run("empty string", func(t *testing.T) {
		_, ok := parseVerboseTimestamp("")
		if ok {
			t.Error("expected ok=false for empty string")
		}
	})

	t.Run("non-timestamp prefix", func(t *testing.T) {
		_, ok := parseVerboseTimestamp("this is not a timestamp at all, way too long")
		if ok {
			t.Error("expected ok=false for non-timestamp prefix")
		}
	})
}

// ---- stripVerbosePrefix ----------------------------------------------------

func TestStripVerbosePrefix(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normal verbose line with PID",
			input: "2024-01-15 10:30:00.123 homeassistant supervisor[1234]: the message",
			want:  "the message",
		},
		{
			name:  "verbose line without PID",
			input: "2024-01-15 10:30:00.123 homeassistant supervisor: the message",
			want:  "the message",
		},
		{
			name:  "message contains colon-space",
			input: "2024-01-15 10:30:00.123 host syslog[99]: key: value",
			want:  "key: value",
		},
		{
			name:  "too short to strip",
			input: "short",
			want:  "short",
		},
		{
			name:  "exactly timestamp length",
			input: "2024-01-15 10:30:00.123",
			want:  "2024-01-15 10:30:00.123",
		},
		{
			name:  "no colon-space separator after prefix",
			input: "2024-01-15 10:30:00.123 noseparatorhere",
			want:  "2024-01-15 10:30:00.123 noseparatorhere",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripVerbosePrefix(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// ---- filterLines (unit-level helper to test filtering logic) ---------------

// filterLines is a pure function that applies the same filtering logic as
// streamLogsFromResponse but operates on an in-memory reader, making it
// straightforward to unit test.
func filterLines(input io.Reader, from, to *time.Time, keepVerbose bool) (string, error) {
	var out strings.Builder
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()

		ts, ok := parseVerboseTimestamp(line)
		if ok {
			if from != nil && ts.Before(*from) {
				continue
			}
			if to != nil && ts.After(*to) {
				continue
			}
		}

		outputLine := line
		if !keepVerbose {
			outputLine = stripVerbosePrefix(line)
		}
		fmt.Fprintln(&out, outputLine)
	}
	return out.String(), scanner.Err()
}

func TestFilterLines(t *testing.T) {
	lines := []string{
		"2024-01-15 09:00:00.000 host svc[1]: before window",
		"2024-01-15 10:00:00.000 host svc[1]: start of window",
		"2024-01-15 10:30:00.000 host svc[1]: middle of window",
		"2024-01-15 11:00:00.000 host svc[1]: end of window",
		"2024-01-15 12:00:00.000 host svc[1]: after window",
	}
	input := strings.Join(lines, "\n") + "\n"

	from := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)

	t.Run("from and to set, plain output", func(t *testing.T) {
		got, err := filterLines(strings.NewReader(input), &from, &to, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantLines := []string{
			"start of window",
			"middle of window",
			"end of window",
		}
		want := strings.Join(wantLines, "\n") + "\n"
		if got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("from and to set, verbose output", func(t *testing.T) {
		got, err := filterLines(strings.NewReader(input), &from, &to, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantLines := []string{
			"2024-01-15 10:00:00.000 host svc[1]: start of window",
			"2024-01-15 10:30:00.000 host svc[1]: middle of window",
			"2024-01-15 11:00:00.000 host svc[1]: end of window",
		}
		want := strings.Join(wantLines, "\n") + "\n"
		if got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("from only", func(t *testing.T) {
		got, err := filterLines(strings.NewReader(input), &from, nil, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantLines := []string{
			"start of window",
			"middle of window",
			"end of window",
			"after window",
		}
		want := strings.Join(wantLines, "\n") + "\n"
		if got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("to only", func(t *testing.T) {
		got, err := filterLines(strings.NewReader(input), nil, &to, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantLines := []string{
			"before window",
			"start of window",
			"middle of window",
			"end of window",
		}
		want := strings.Join(wantLines, "\n") + "\n"
		if got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("lines without parseable timestamps pass through", func(t *testing.T) {
		mixed := "2024-01-15 10:30:00.000 host svc[1]: in window\nnot-a-timestamp line\n"
		got, err := filterLines(strings.NewReader(mixed), &from, &to, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, "in window") {
			t.Error("expected 'in window' in output")
		}
		if !strings.Contains(got, "not-a-timestamp line") {
			t.Error("expected non-timestamp line to be passed through")
		}
	})

	t.Run("empty input produces empty output", func(t *testing.T) {
		got, err := filterLines(bytes.NewReader(nil), &from, &to, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("expected empty output, got %q", got)
		}
	})
}
