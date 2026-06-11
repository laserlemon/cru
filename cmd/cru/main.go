// Command cru scores a single pull request from positional arguments.
//
//	cru <total> [owned] [risk]
//
// total  required, int, the pull request's full additions + deletions
// owned  optional, int, the lines this reviewer is responsible for
//
//	(defaults to total: full ownership)
//
// risk   optional, one of "low" / "medium" / "high"
//
//	(defaults to "low")
//
// Output adapts to context:
//
//	TTY      labeled, with gray ANSI on field names
//	pipe     bare number, one per scoring
//	--json   structured object (per scoring; NDJSON in batch)
//
// Stdin batch mode: with no positional arguments and stdin attached to a
// pipe, reads one scoring per line in the same arg shape:
//
//	$ printf "100 85 low\n240\n50 high\n" | cru
//	1.235
//	5.169
//	0.792
//
// Blank lines and lines starting with "#" are ignored. A bad line writes
// to stderr with the line number and does not abort the batch; the exit
// code is 1 if any line failed, 0 otherwise.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/laserlemon/cru"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is main extracted for testability. Returns the process exit code.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cru", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, usage)
	}
	jsonOut := fs.Bool("json", false, "emit a JSON object per scoring (NDJSON in batch mode)")

	// Stdlib flag stops parsing at the first non-flag token, so
	// "cru 100 --json" would treat --json as a positional. Pre-split
	// so flags work in any position, the way users expect.
	flagArgs, positional := splitFlags(args)
	if err := fs.Parse(flagArgs); err != nil {
		// flag prints its own message + usage; just return non-zero.
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	// Mode dispatch.
	switch {
	case len(positional) == 0 && isPipe(stdin):
		return runBatch(stdin, stdout, stderr, *jsonOut)
	case len(positional) == 0:
		fmt.Fprint(stderr, usage)
		return 2
	default:
		return runOne(positional, stdout, stderr, *jsonOut)
	}
}

// splitFlags partitions args into flag-looking tokens and positional
// tokens, preserving order within each bucket. Lets us call flag.Parse
// without requiring users to put flags before positional args.
//
// A token is a flag when it starts with "-" and the next character
// isn't a digit. The digit exception preserves negative numbers as
// positional input, even though "total must be > 0" makes them an
// error downstream anyway (better error message via the validator).
func splitFlags(args []string) (flags, positional []string) {
	for _, a := range args {
		if looksLikeFlag(a) {
			flags = append(flags, a)
		} else {
			positional = append(positional, a)
		}
	}
	return flags, positional
}

func looksLikeFlag(s string) bool {
	if len(s) < 2 || s[0] != '-' {
		return false
	}
	// "-5" is a number, not a flag.
	if s[1] >= '0' && s[1] <= '9' {
		return false
	}
	return true
}

// runOne scores a single positional invocation.
func runOne(args []string, stdout, stderr io.Writer, jsonOut bool) int {
	total, owned, risk, err := parseScoring(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	emit(stdout, total, owned, risk, jsonOut, isTTY(stdout))
	return 0
}

// runBatch scores one input per line from stdin.
func runBatch(stdin io.Reader, stdout, stderr io.Writer, jsonOut bool) int {
	scanner := bufio.NewScanner(stdin)
	// Allow long lines (default 64 KB is plenty for our 1-3 fields, but
	// be polite about pathological whitespace).
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	failed := false
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		fields := strings.Fields(raw)
		total, owned, risk, err := parseScoring(fields)
		if err != nil {
			fmt.Fprintf(stderr, "line %d: %v\n", lineNum, err)
			failed = true
			continue
		}
		// Batch output never decorates: no TTY heuristic. Bare or
		// NDJSON only.
		emit(stdout, total, owned, risk, jsonOut, false)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(stderr, "error reading stdin:", err)
		return 1
	}
	if failed {
		return 1
	}
	return 0
}

// parseScoring resolves positional args into (total, owned, risk).
// Accepts 1, 2, or 3 args. With 2 args, type-detects whether arg2 is
// the owned count (int) or a risk label (string).
func parseScoring(args []string) (total, owned int, risk cru.Risk, err error) {
	if len(args) == 0 || len(args) > 3 {
		return 0, 0, nil, fmt.Errorf("expected 1 to 3 args (total [owned] [risk]); got %d", len(args))
	}

	total, err = parsePositiveInt(args[0], "total")
	if err != nil {
		return 0, 0, nil, err
	}
	owned = total
	risk = cru.RiskLow

	switch len(args) {
	case 1:
		return total, owned, risk, nil
	case 2:
		// Disambiguate by type: int → owned; non-int → risk label.
		if n, intErr := strconv.Atoi(args[1]); intErr == nil {
			if n < 0 || n > total {
				return 0, 0, nil, fmt.Errorf("owned must be between 0 and %d; got %d", total, n)
			}
			return total, n, risk, nil
		}
		r, riskErr := parseRisk(args[1])
		if riskErr != nil {
			// Surface as an arg-2 problem: the user gave us
			// something that's neither a valid int nor a valid
			// risk label.
			return 0, 0, nil, fmt.Errorf("second arg must be owned (int) or risk (low|medium|high or l|m|h); got %q", args[1])
		}
		return total, owned, r, nil
	case 3:
		n, intErr := strconv.Atoi(args[1])
		if intErr != nil {
			return 0, 0, nil, fmt.Errorf("owned must be an integer; got %q", args[1])
		}
		if n < 0 || n > total {
			return 0, 0, nil, fmt.Errorf("owned must be between 0 and %d; got %d", total, n)
		}
		r, riskErr := parseRisk(args[2])
		if riskErr != nil {
			return 0, 0, nil, riskErr
		}
		return total, n, r, nil
	}
	// Unreachable: switch covers 1, 2, 3 and the length guard rejected
	// everything else.
	return 0, 0, nil, nil
}

// parsePositiveInt parses a strictly positive integer.
func parsePositiveInt(s, name string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer; got %q", name, s)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s must be > 0; got %d", name, n)
	}
	return n, nil
}

// parseRisk maps a CLI label to a cru.Risk constant. Accepts the full
// tier name or its first letter, case-insensitive.
func parseRisk(s string) (cru.Risk, error) {
	switch strings.ToLower(s) {
	case "low", "l":
		return cru.RiskLow, nil
	case "medium", "m":
		return cru.RiskMedium, nil
	case "high", "h":
		return cru.RiskHigh, nil
	}
	return nil, fmt.Errorf("risk must be one of low / medium / high (or l / m / h); got %q", s)
}

// emit writes a single scoring's output in the appropriate format.
// All math happens here on raw float64; rounding is display-only.
func emit(w io.Writer, total, owned int, risk cru.Risk, jsonOut, decorate bool) {
	size := cru.CalculateSize(total)
	factor := size.Factor()
	mult := risk.Multiplier()
	share := float64(owned) / float64(total)
	score := cru.Calculate(total, owned, risk)

	switch {
	case jsonOut:
		obj := struct {
			Total           int     `json:"total"`
			Owned           int     `json:"owned"`
			OwnershipShare  float64 `json:"ownership_share"`
			Size            string  `json:"size"`
			SizeFactor      float64 `json:"size_factor"`
			Risk            string  `json:"risk"`
			RiskMultiplier  float64 `json:"risk_multiplier"`
			CRU             float64 `json:"cru"`
		}{
			Total:          total,
			Owned:          owned,
			OwnershipShare: share,
			Size:           size.String(),
			SizeFactor:     factor,
			Risk:           risk.String(),
			RiskMultiplier: mult,
			CRU:            score,
		}
		// Compact NDJSON: one object per line, no trailing whitespace
		// other than the newline json.Encoder writes for us.
		_ = json.NewEncoder(w).Encode(obj)
	case decorate:
		writeHuman(w, total, owned, risk, size, factor, mult, share, score)
	default:
		// Bare: just the number.
		fmt.Fprintf(w, "%.3f\n", score)
	}
}

// writeHuman emits the labeled, gray-on-labels human format.
func writeHuman(w io.Writer, total, owned int, risk cru.Risk, size cru.Size, factor, mult, share, score float64) {
	rows := []struct {
		label string
		value string
	}{
		{"Total LOC", strconv.Itoa(total)},
		{"Size", size.String()},
		{"Size factor", fmt.Sprintf("%.3f", factor)},
		{"Owned LOC", strconv.Itoa(owned)},
		{"Ownership share", fmt.Sprintf("%.3f", share)},
		{"Risk", risk.String()},
		{"Risk multiplier", fmt.Sprintf("%.3f", mult)},
		{"CRU", fmt.Sprintf("%.3f", score)},
	}
	// Right-pad labels to the longest one for column alignment.
	width := 0
	for _, r := range rows {
		if len(r.label) > width {
			width = len(r.label)
		}
	}
	for _, r := range rows {
		padded := r.label + strings.Repeat(" ", width-len(r.label))
		fmt.Fprintf(w, "%s  %s\n", colorize(padded), r.value)
	}
}

const usage = `Usage: cru <total> [owned] [risk]

  total  pull request line count (additions + deletions), required
  owned  reviewer-owned lines (defaults to total: full ownership)
  risk   low / medium / high (or l / m / h); defaults to low

Flags:
  --json  emit a JSON object per scoring (NDJSON in batch mode)
  --help  show this help

Stdin batch: with no positional args and stdin piped in, reads one
scoring per line (same arg shape). Blank lines and "#"-comment lines
are skipped.

Examples:
  cru 100              # 100-line PR, fully owned, low risk
  cru 100 85           # 100-line PR, 85 lines owned, low risk
  cru 100 h            # 100-line PR, fully owned, high risk
  cru 100 85 m         # 100-line PR, 85 owned, medium risk
  cru 100 --json       # structured output
  cat prs.txt | cru    # bulk score, one per line
`
