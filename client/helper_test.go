package client

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/spf13/viper"
)

var urlTests = []struct {
	base    string
	section string
	command string
	out     string
}{
	{"supervisor", "section", "command", "http://supervisor/section/command"},
	{"supervisor:80", "section", "command", "http://supervisor:80/section/command"},
	{"supervisor.example.org:8080", "section", "command", "http://supervisor.example.org:8080/section/command"},
	{"https://supervisor", "section", "command", "https://supervisor/section/command"},
	{"https://supervisor:8080", "section", "command", "https://supervisor:8080/section/command"},
	{"https://supervisor", "section", "command", "https://supervisor/section/command"},
	{"https://supervisor:8080", "section", "command", "https://supervisor:8080/section/command"},
	{"supervisor", "section", "", "http://supervisor/section"},
	{"supervisor", "section/../othersection", "", "http://supervisor/othersection"},
	{"supervisor/api/", "section", "command", "http://supervisor/api/section/command"},
	{"supervisor/api/", "section", "{slug}/command", "http://supervisor/api/section/{slug}/command"},
}

func TestURLHelper(t *testing.T) {
	for _, tt := range urlTests {
		t.Run(fmt.Sprintf("[%s][%s][%s]", tt.base, tt.section, tt.command), func(t *testing.T) {
			viper.SetDefault("endpoint", tt.base)
			s, _ := URLHelper(tt.section, tt.command)
			if s != tt.out {
				t.Errorf("got %q, want %q", s, tt.out)
			}
		})
	}
}

var parseHALogLevelTests = []struct {
	line       string
	wantLevel  slog.Level
	wantParsed bool
}{
	// Valid HA log lines
	{"2026-05-18 16:33:16.267 DEBUG (MainThread) [module] msg", slog.LevelDebug, true},
	{"2026-05-18 16:33:16.267 INFO (MainThread) [module] msg", slog.LevelInfo, true},
	{"2026-05-18 16:33:16.267 WARNING (MainThread) [module] msg", slog.LevelWarn, true},
	{"2026-05-18 16:33:16.267 ERROR (MainThread) [module] msg", slog.LevelError, true},
	{"2026-05-18 16:33:16.267 CRITICAL (MainThread) [module] msg", slog.LevelError + 4, true},
	// Continuation / stack-trace lines
	{"  File \"/usr/lib/python3.13/asyncio/events.py\", line 88", 0, false},
	{"Traceback (most recent call last):", 0, false},
	{"", 0, false},
}

func TestParseHALogLevel(t *testing.T) {
	for _, tt := range parseHALogLevelTests {
		t.Run(tt.line, func(t *testing.T) {
			gotLevel, gotParsed := parseHALogLevel(tt.line)
			if gotParsed != tt.wantParsed {
				t.Errorf("parseHALogLevel(%q) parsed=%v, want %v", tt.line, gotParsed, tt.wantParsed)
			}
			if gotParsed && gotLevel != tt.wantLevel {
				t.Errorf("parseHALogLevel(%q) level=%v, want %v", tt.line, gotLevel, tt.wantLevel)
			}
		})
	}
}
