package cmd

import (
	"bufio"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	helper "github.com/home-assistant/cli/client"
	"github.com/spf13/cobra"
)

var hostCmd = &cobra.Command{
	Use:     "host",
	Aliases: []string{"ho"},
	Short:   "Control the host/system that Home Assistant is running on",
	Long: `
The host command provides commandline tools to control the host (system) that
Home Assistant is running on. It allows you do thing like reboot or shutdown the
system, but also provides option to change the hostname of the system.`,
	Example: `
  ha host reboot
  ha host options --hostname "homeassistant.local"
`,
}

func init() {
	rootCmd.AddCommand(hostCmd)
}

func addLogsFlags(cmd *cobra.Command) {
	cmd.Flags().BoolP("follow", "f", false, "Continuously print new log entries")
	cmd.Flags().Uint32P("lines", "n", 0, "Number of log entries to show")
	cmd.Flags().StringP("boot", "b", "", "Logs of particular boot ID")
	cmd.Flags().BoolP("verbose", "v", false, "Return logs in verbose format")
	cmd.Flags().String("from", "", "Show entries starting from this point in time (RFC3339 e.g. '2024-01-15T10:30:00Z', date e.g. '2024-01-15', or relative duration e.g. '1h', '30m', '2d', '1w')")
	cmd.Flags().String("to", "", "Show entries up to this point in time (RFC3339 e.g. '2024-01-15T10:30:00Z', date e.g. '2024-01-15', or relative duration e.g. '1h', '30m', '2d', '1w')")
	cmd.Flags().Lookup("follow").NoOptDefVal = "true"
	cmd.Flags().Lookup("verbose").NoOptDefVal = "true"

	cmd.RegisterFlagCompletionFunc("follow", boolCompletions)
	cmd.RegisterFlagCompletionFunc("verbose", boolCompletions)
	cmd.RegisterFlagCompletionFunc("lines", cobra.NoFileCompletions)
	cmd.RegisterFlagCompletionFunc("boot", hostBootCompletions)
	cmd.RegisterFlagCompletionFunc("from", cobra.NoFileCompletions)
	cmd.RegisterFlagCompletionFunc("to", cobra.NoFileCompletions)
}

func processLogsFlags(section string, cmd *cobra.Command) (*resty.Request, error) {
	command := "logs"

	boot, _ := cmd.Flags().GetString("boot")
	if len(boot) > 0 {
		command += "/boots/{boot}"
	}

	follow, _ := cmd.Flags().GetBool("follow")
	if follow {
		command += "/follow"
	}

	URL, err := helper.URLHelper(section, command)
	if err != nil {
		return nil, err
	}

	// When --from or --to is set we must receive verbose output so that
	// each line carries a parseable timestamp for client-side filtering.
	fromStr, _ := cmd.Flags().GetString("from")
	toStr, _ := cmd.Flags().GetString("to")
	filterActive := fromStr != "" || toStr != ""

	verbose, _ := cmd.Flags().GetBool("verbose")
	accept := "text/plain"
	if verbose || filterActive {
		accept = "text/x-log"
	}

	/* Disable timeouts to allow following forever */
	request := helper.GetRequestTimeout(0).SetHeader("Accept", accept).SetDoNotParseResponse(true)

	lines, _ := cmd.Flags().GetUint32("lines")
	if lines > 0 {
		rangeHeader := fmt.Sprintf("entries=:%d:", -(int(lines) - 1))
		slog.Debug("Range header", "value", rangeHeader)
		request.SetHeader("Range", rangeHeader)
	}

	fromStr, _ := cmd.Flags().GetString("from")
	if fromStr != "" {
		ts, err := parseTimestamp(fromStr)
		if err != nil {
			return nil, fmt.Errorf("invalid --from value: %w", err)
		}
		request.SetQueryParam("range_start", strconv.FormatInt(ts, 10))
	}

	toStr, _ := cmd.Flags().GetString("to")
	if toStr != "" {
		ts, err := parseTimestamp(toStr)
		if err != nil {
			return nil, fmt.Errorf("invalid --to value: %w", err)
		}
		request.SetQueryParam("range_end", strconv.FormatInt(ts, 10))
	}

	request.SetPathParam("boot", boot)
	request.URL = URL

	return request, nil
}

// verboseTimestampLayout is the format produced by the supervisor's
// journal_verbose_formatter: "YYYY-MM-DD HH:MM:SS.mmm"
const verboseTimestampLayout = "2006-01-02 15:04:05.000"

// verboseTimestampLen is the fixed number of characters that make up the
// timestamp prefix in a verbose log line.
const verboseTimestampLen = len(verboseTimestampLayout) // 23

// parseVerboseTimestamp extracts the UTC timestamp from the first
// verboseTimestampLen characters of a verbose log line. Returns the parsed
// time and true on success, or the zero time and false when the line is too
// short or the prefix does not match the expected format.
func parseVerboseTimestamp(line string) (time.Time, bool) {
	if len(line) < verboseTimestampLen {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation(verboseTimestampLayout, line[:verboseTimestampLen], time.UTC)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// stripVerbosePrefix removes the timestamp, hostname, and syslog-identifier
// prefix from a verbose log line, returning only the plain message.  The
// verbose format produced by the supervisor is:
//
// "YYYY-MM-DD HH:MM:SS.mmm <hostname> <identifier>[<pid>]: <message>"
//
// If the prefix cannot be identified the original line is returned unchanged.
func stripVerbosePrefix(line string) string {
	// Skip the fixed-width timestamp plus the following space.
	if len(line) <= verboseTimestampLen+1 {
		return line
	}
	rest := line[verboseTimestampLen+1:]
	idx := strings.Index(rest, ": ")
	if idx < 0 {
		return line
	}
	return rest[idx+2:]
}

// streamLogsFromResponse streams the response body to stdout, applying
// optional client-side time-range filtering when --from or --to flags are set.
//
// When filtering is active the function reads the verbose log stream line by
// line, parses the timestamp embedded in each line, and skips lines that fall
// outside the requested window.  Lines whose timestamp cannot be parsed are
// passed through unconditionally so that unexpected output (e.g. empty lines,
// binary artefacts) is never silently dropped.  If --verbose was not
// explicitly requested the verbose prefix is stripped before printing so that
// the output matches what a plain-format response would have produced.
//
// When neither --from nor --to is set the function falls back to the
// unfiltered StreamTextResponse helper for identical behaviour to the
// pre-existing code path.
func streamLogsFromResponse(resp *resty.Response, cmd *cobra.Command) bool {
	fromStr, _ := cmd.Flags().GetString("from")
	toStr, _ := cmd.Flags().GetString("to")

	// Fast path: no time-range filtering requested.
	if fromStr == "" && toStr == "" {
		return helper.StreamTextResponse(resp)
	}

	var from, to time.Time
	var hasFrom, hasTo bool

	if fromStr != "" {
		t, err := parseTimeValue(fromStr)
		if err != nil {
			helper.PrintError(fmt.Errorf("invalid --from value: %w", err))
			_ = resp.RawBody().Close()
			return false
		}
		from = t
		hasFrom = true
	}
	if toStr != "" {
		t, err := parseTimeValue(toStr)
		if err != nil {
			helper.PrintError(fmt.Errorf("invalid --to value: %w", err))
			_ = resp.RawBody().Close()
			return false
		}
		to = t
		hasTo = true
	}

	verbose, _ := cmd.Flags().GetBool("verbose")
	follow, _ := cmd.Flags().GetBool("follow")

	body := resp.RawBody()
	defer body.Close()

	// Use a large scanner buffer so that unusually long log lines do not
	// cause an unexpected truncation error.
	scanner := bufio.NewScanner(body)
	buf := make([]byte, 512*1024)
	scanner.Buffer(buf, len(buf))

	for scanner.Scan() {
		line := scanner.Text()

		ts, ok := parseVerboseTimestamp(line)
		if ok {
			if hasFrom && ts.Before(from) {
				continue
			}
			if hasTo && ts.After(to) {
				// When following, stop the stream once we have passed the
				// requested end time rather than continuing to discard lines.
				if follow {
					break
				}
				continue
			}
		}
		// Lines whose timestamp cannot be parsed are passed through so that
		// unexpected output is visible.

		outputLine := line
		if !verbose {
			outputLine = stripVerbosePrefix(line)
		}
		fmt.Println(outputLine)
	}

	return scanner.Err() == nil
}

// parseTimeValue converts a user-supplied timestamp string into a time.Time.
// It is the canonical parsing function used throughout the logs filtering code;
// parseTimestamp delegates to it for callers that need a Unix-microsecond int64.
//
// Supported formats:
//   - RFC 3339:   "2024-01-15T10:30:00Z" or "2024-01-15T10:30:00+02:00"
//   - Date only:  "2024-01-15" (interpreted as 00:00:00 UTC)
//   - Relative:   "1h", "30m", "2d", "1w" (time ago from now); a leading "-"
//     is accepted and ignored so that "-1h" and "1h" are equivalent.
func parseTimeValue(s string) (time.Time, error) {
	for _, format := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(format, s); err == nil {
			return t.UTC(), nil
		}
	}

	// Relative duration – strip an optional leading dash so that both "1h"
	// and "-1h" are treated as "1 hour ago".
	durStr := strings.TrimPrefix(s, "-")
	if d, err := parseRelativeDuration(durStr); err == nil {
		return time.Now().Add(-d).UTC(), nil
	}

	return time.Time{}, fmt.Errorf(
		"unrecognized timestamp %q: use RFC3339 (e.g. 2024-01-15T10:30:00Z), "+
			"date (e.g. 2024-01-15), or relative duration (e.g. 1h, 30m, 2d, 1w)",
		s,
	)
}

// parseRelativeDuration parses a relative duration string, extending Go's
// time.ParseDuration with support for days ("d") and weeks ("w").
func parseRelativeDuration(s string) (time.Duration, error) {
	if len(s) > 1 {
		switch s[len(s)-1] {
		case 'd':
			n, err := strconv.ParseInt(s[:len(s)-1], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid duration %q", s)
			}
			return time.Duration(n) * 24 * time.Hour, nil
		case 'w':
			n, err := strconv.ParseInt(s[:len(s)-1], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid duration %q", s)
			}
			return time.Duration(n) * 7 * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
}

// parseTimestamp parses a user-supplied timestamp string and returns the
// corresponding Unix time in microseconds.  It delegates to parseTimeValue;
// see that function for supported formats.
func parseTimestamp(s string) (int64, error) {
	t, err := parseTimeValue(s)
	if err != nil {
		return 0, err
	}
	return t.UnixMicro(), nil
}
