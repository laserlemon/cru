package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/laserlemon/cru"
)

func TestParseScoring(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		total     int
		owned     int
		risk      cru.Risk
		wantError string
	}{
		{name: "total only", args: []string{"100"}, total: 100, owned: 100, risk: cru.RiskLow},
		{name: "total + owned", args: []string{"100", "85"}, total: 100, owned: 85, risk: cru.RiskLow},
		{name: "total + risk low", args: []string{"100", "low"}, total: 100, owned: 100, risk: cru.RiskLow},
		{name: "total + risk l", args: []string{"100", "l"}, total: 100, owned: 100, risk: cru.RiskLow},
		{name: "total + risk medium", args: []string{"100", "medium"}, total: 100, owned: 100, risk: cru.RiskMedium},
		{name: "total + risk m", args: []string{"100", "m"}, total: 100, owned: 100, risk: cru.RiskMedium},
		{name: "total + risk high", args: []string{"100", "HIGH"}, total: 100, owned: 100, risk: cru.RiskHigh},
		{name: "total + risk h uppercase", args: []string{"100", "H"}, total: 100, owned: 100, risk: cru.RiskHigh},
		{name: "all three", args: []string{"100", "85", "high"}, total: 100, owned: 85, risk: cru.RiskHigh},
		{name: "zero owned", args: []string{"100", "0"}, total: 100, owned: 0, risk: cru.RiskLow},
		{name: "owned equals total", args: []string{"100", "100", "medium"}, total: 100, owned: 100, risk: cru.RiskMedium},

		{name: "zero args", args: nil, wantError: "expected 1 to 3 args"},
		{name: "four args", args: []string{"100", "85", "low", "extra"}, wantError: "expected 1 to 3 args"},
		{name: "non-integer total", args: []string{"abc"}, wantError: "total must be an integer"},
		{name: "zero total", args: []string{"0"}, wantError: "total must be > 0"},
		{name: "negative total", args: []string{"-5"}, wantError: "total must be > 0"},
		{name: "2-arg neither int nor risk", args: []string{"100", "bogus"}, wantError: "second arg must be owned"},
		{name: "2-arg owned negative", args: []string{"100", "-3"}, wantError: "owned must be between 0 and 100"},
		{name: "2-arg owned > total", args: []string{"100", "150"}, wantError: "owned must be between 0 and 100"},
		{name: "3-arg owned non-int", args: []string{"100", "abc", "low"}, wantError: "owned must be an integer"},
		{name: "3-arg owned > total", args: []string{"100", "200", "low"}, wantError: "owned must be between 0 and 100"},
		{name: "3-arg bad risk", args: []string{"100", "85", "extreme"}, wantError: "risk must be one of low"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			total, owned, risk, err := parseScoring(tc.args)
			if tc.wantError != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantError)
				}
				if !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if total != tc.total {
				t.Errorf("total = %d, want %d", total, tc.total)
			}
			if owned != tc.owned {
				t.Errorf("owned = %d, want %d", owned, tc.owned)
			}
			if risk != tc.risk {
				t.Errorf("risk = %v, want %v", risk, tc.risk)
			}
		})
	}
}

func TestEmitBare(t *testing.T) {
	var out bytes.Buffer
	emit(&out, 100, 85, cru.RiskLow, false, false)
	got := strings.TrimSpace(out.String())
	if got != "1.536003" {
		t.Errorf("bare output = %q, want %q", got, "1.536003")
	}
}

func TestEmitJSON(t *testing.T) {
	var out bytes.Buffer
	emit(&out, 100, 85, cru.RiskMedium, true, false)
	var obj map[string]any
	if err := json.Unmarshal(out.Bytes(), &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	wantKeys := []string{"total_lines", "size_label", "size_factor", "owned_lines", "ownership_share", "risk_label", "risk_multiplier", "base_cru", "cru"}
	for _, k := range wantKeys {
		if _, ok := obj[k]; !ok {
			t.Errorf("JSON missing key %q; got %v", k, obj)
		}
	}
	// Field order in the raw JSON should mirror the human output rows.
	// We index into the serialized bytes (not the unmarshaled map, which
	// is unordered) and verify each key appears after the previous one.
	raw := out.String()
	prev := -1
	for _, k := range wantKeys {
		idx := strings.Index(raw, `"`+k+`"`)
		if idx < 0 {
			t.Fatalf("key %q not found in raw JSON: %s", k, raw)
		}
		if idx < prev {
			t.Errorf("key %q at index %d appears before previous key at %d; JSON: %s", k, idx, prev, raw)
		}
		prev = idx
	}
	if obj["risk_label"] != "medium" {
		t.Errorf("risk_label = %v, want medium", obj["risk_label"])
	}
	if obj["total_lines"].(float64) != 100 {
		t.Errorf("total_lines = %v, want 100", obj["total_lines"])
	}
	// All float fields should be 6-decimal strings as numbers, never
	// 14-digit float-noise representations. size_factor for L = 1.807063,
	// ownership_share = 0.850000 (trailing zeros preserved by %.6f).
	got := out.String()
	if !strings.Contains(got, "1.807063") {
		t.Errorf("expected 6-decimal size_factor 1.807063 in JSON, got %q", got)
	}
	if !strings.Contains(got, "0.850000") {
		t.Errorf("expected 6-decimal ownership_share 0.850000 in JSON, got %q", got)
	}
	// base_cru = size_factor × risk_multiplier, rounded once from full
	// precision: 1.8070631… × 2.0 = 3.614125 (NOT 3.614126, which is what
	// multiplying the displayed 6-decimal factor would give). Rounding the
	// true product, like cru itself, is what keeps base_cru × share == cru.
	if !strings.Contains(got, "3.614125") {
		t.Errorf("expected 6-decimal base_cru 3.614125 in JSON, got %q", got)
	}
	if strings.Contains(got, "0.85,") || strings.Contains(got, ":2,") {
		t.Errorf("JSON has bare float (no .6 padding), got %q", got)
	}
}

func TestEmitHuman(t *testing.T) {
	t.Setenv("NO_COLOR", "1") // deterministic: skip color escapes
	var out bytes.Buffer
	emit(&out, 100, 85, cru.RiskLow, false, true)
	got := out.String()
	for _, want := range []string{
		"Total lines      100",
		"Size             L",
		"Size factor      1.807",
		"Owned lines      85",
		"Ownership share  0.850",
		"Risk             low",
		"Risk multiplier  1.000",
		"CRU              1.536",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("human output missing line %q\nfull output:\n%s", want, got)
		}
	}
	// Verify order.
	lines := strings.Split(strings.TrimSpace(got), "\n")
	wantLabels := []string{"Total lines", "Size ", "Size factor", "Owned lines", "Ownership share", "Risk ", "Risk multiplier", "CRU"}
	if len(lines) != len(wantLabels) {
		t.Fatalf("expected %d lines, got %d", len(wantLabels), len(lines))
	}
	for i, want := range wantLabels {
		if !strings.HasPrefix(lines[i], want) {
			t.Errorf("line %d = %q, want prefix %q", i, lines[i], want)
		}
	}
}

func TestRunOne(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var stdout, stderr bytes.Buffer
	code := run([]string{"100", "85", "low"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr.String())
	}
	// bytes.Buffer triggers bare (non-TTY) output, which is 6 decimals.
	if !strings.Contains(stdout.String(), "1.536003") {
		t.Errorf("expected 1.536003 in output, got %q", stdout.String())
	}
}

// TestFlagPositionAgnostic locks in that --json can appear before, after,
// or between positionals. Stdlib flag stops parsing at the first non-flag
// token, so we pre-split; these cases pin that behavior.
func TestFlagPositionAgnostic(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "flag before all positionals", args: []string{"--json", "100", "85", "low"}},
		{name: "flag after total", args: []string{"100", "--json"}},
		{name: "flag after total+owned", args: []string{"100", "85", "--json"}},
		{name: "flag after all positionals", args: []string{"100", "85", "low", "--json"}},
		{name: "flag between positionals", args: []string{"100", "--json", "85", "low"}},
		{name: "flag at the end", args: []string{"100", "85", "low", "--json"}},
		{name: "equals form", args: []string{"100", "--json=true"}},
		{name: "single-dash form", args: []string{"100", "-json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tc.args, &bytes.Buffer{}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr.String())
			}
			// Validate JSON output regardless of arg shape.
			if !strings.Contains(stdout.String(), `"cru":`) {
				t.Errorf("expected JSON output, got %q", stdout.String())
			}
		})
	}
}

// TestSplitFlags pins the per-token classification so the
// digit-exception (negative numbers stay positional) doesn't regress.
func TestSplitFlags(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		wantFlags []string
		wantPos   []string
	}{
		{name: "empty", args: nil, wantFlags: nil, wantPos: nil},
		{name: "all positional", args: []string{"100", "85", "low"}, wantFlags: nil, wantPos: []string{"100", "85", "low"}},
		{name: "all flags", args: []string{"--json", "--help"}, wantFlags: []string{"--json", "--help"}, wantPos: nil},
		{name: "interleaved", args: []string{"100", "--json", "85"}, wantFlags: []string{"--json"}, wantPos: []string{"100", "85"}},
		{name: "negative number stays positional", args: []string{"-5", "--json"}, wantFlags: []string{"--json"}, wantPos: []string{"-5"}},
		{name: "bare dash stays positional", args: []string{"-"}, wantFlags: nil, wantPos: []string{"-"}},
		{name: "equals form", args: []string{"100", "--json=true"}, wantFlags: []string{"--json=true"}, wantPos: []string{"100"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotFlags, gotPos := splitFlags(tc.args)
			if !equalSlices(gotFlags, tc.wantFlags) {
				t.Errorf("flags = %v, want %v", gotFlags, tc.wantFlags)
			}
			if !equalSlices(gotPos, tc.wantPos) {
				t.Errorf("positional = %v, want %v", gotPos, tc.wantPos)
			}
		})
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRunBadArg(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"abc"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "total must be an integer") {
		t.Errorf("stderr = %q, want error about total", stderr.String())
	}
}

func TestRunBadFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--bogus"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, &bytes.Buffer{}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("expected usage in stderr, got %q", stderr.String())
	}
}

func TestRunNoArgsNoStdin(t *testing.T) {
	// bytes.Buffer is not *os.File, so isPipe returns false. With no
	// positional args, this is the "needs input or args" error path.
	var stdout, stderr bytes.Buffer
	code := run(nil, &bytes.Buffer{}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("expected usage in stderr, got %q", stderr.String())
	}
}

func TestRunBatchBare(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	stdin := mustPipeReader(t, "100 85 low\n240\n50 high\n# comment\n\n")
	defer stdin.Close()
	var stdout, stderr bytes.Buffer
	code := run(nil, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	want := []string{"1.536003", "3.065154", "4.330209"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d: %v", len(lines), len(want), lines)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line %d = %q, want %q", i, lines[i], w)
		}
	}
}

func TestRunBatchJSON(t *testing.T) {
	stdin := mustPipeReader(t, "100 85 low\n240 high\n")
	defer stdin.Close()
	var stdout, stderr bytes.Buffer
	code := run([]string{"--json"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d JSON lines, want 2", len(lines))
	}
	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line %d not valid JSON: %v", i, err)
		}
	}
}

func TestRunBatchBadLine(t *testing.T) {
	stdin := mustPipeReader(t, "100 85 low\nabc\n240\n")
	defer stdin.Close()
	var stdout, stderr bytes.Buffer
	code := run(nil, stdin, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d, want 1 (one bad line)", code)
	}
	// The two good lines still produced output.
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("got %d output lines, want 2 (skipping the bad one): %v", len(lines), lines)
	}
	if !strings.Contains(stderr.String(), "line 2:") {
		t.Errorf("expected 'line 2:' in stderr, got %q", stderr.String())
	}
}

// TestRunBatchScannerErr exercises the empty-batch success path: stdin
// is a pipe with the write end immediately closed, so the scanner sees
// EOF without reading any lines.
func TestRunBatchScannerErr(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Close() // EOF immediately on read
	defer r.Close()
	var stdout, stderr bytes.Buffer
	code := run(nil, r, &stdout, &stderr)
	if code != 0 {
		// Empty batch is success; matches Unix convention.
		t.Errorf("exit = %d, want 0 on empty batch", code)
	}
}
