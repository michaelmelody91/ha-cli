package cmd

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/home-assistant/cli/client"
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

	URL, err := client.URLHelper(section, command)
	if err != nil {
		return nil, err
	}

	accept := "text/plain"
	verbose, _ := cmd.Flags().GetBool("verbose")
	if verbose {
		accept = "text/x-log"
	}

	/* Disable timeouts to allow following forever */
	request := client.GetRequestTimeout(0).SetHeader("Accept", accept).SetDoNotParseResponse(true)

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

// parseTimestamp parses a timestamp string into Unix microseconds.
// Supported formats:
//   - RFC 3339:   "2024-01-15T10:30:00Z" or "2024-01-15T10:30:00+02:00"
//   - Date only:  "2024-01-15" (interpreted as 00:00:00 UTC)
//   - Relative:   "1h", "30m", "2d", "1w" (time ago from now); a leading "-"
//     is accepted and ignored so that "-1h" and "1h" are equivalent.
func parseTimestamp(s string) (int64, error) {
	for _, format := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(format, s); err == nil {
			return t.UTC().UnixMicro(), nil
		}
	}

	// Relative duration – strip an optional leading dash so that both "1h"
	// and "-1h" are treated as "1 hour ago".
	durStr := strings.TrimPrefix(s, "-")
	if d, err := parseRelativeDuration(durStr); err == nil {
		return time.Now().Add(-d).UnixMicro(), nil
	}

	return 0, fmt.Errorf(
		"unrecognized timestamp %q: use RFC3339 (e.g. 2024-01-15T10:30:00Z), "+
			"date (e.g. 2024-01-15), or relative duration (e.g. 1h, 30m, 2d, 1w)",
		s,
	)
}
