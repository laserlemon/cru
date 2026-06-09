package cru

import (
	"math"
	"strings"
	"testing"
)

// closeTo asserts a ≈ b within tol (default 1e-9 when tol == 0).
func closeTo(t *testing.T, label string, got, want, tol float64) {
	t.Helper()
	if tol == 0 {
		tol = 1e-9
	}
	if math.Abs(got-want) > tol {
		t.Errorf("%s: got %v, want %v (tol %v)", label, got, want, tol)
	}
}

// TestSizeFactorDoublingTargets is the headline property: CRU doubles at
// the median of each shirt-size quintile of the baseline distribution.
// These five targets are why the formula was derived; if they ever drift,
// the formula has been changed by accident and SHOULD fail loudly.
func TestSizeFactorDoublingTargets(t *testing.T) {
	cases := []struct {
		percentile float64
		want       float64
	}{
		{0.1, 0.25},
		{0.3, 0.5},
		{0.5, 1.0},
		{0.7, 2.0},
		{0.9, 4.0},
	}
	for _, c := range cases {
		// Invert F to get the LOC corresponding to the percentile, then
		// re-apply SizeOf.
		// L = exp(μ + σ · Φ⁻¹(p))
		z := probit(c.percentile)
		loc := math.Exp(Mu + Sigma*z)
		// Closed-form check against the formula.
		got := math.Pow(2, 5*c.percentile-2.5)
		closeTo(t, "doubling target", got, c.want, 1e-12)
		// Sanity: SizeOf on the actual rounded LOC is close (we lose a
		// hair to the discrete integer cast).
		sz := SizeOf(int(math.Round(loc)))
		if math.Abs(float64(sz)-c.want) > 0.01 {
			t.Errorf("SizeOf(%d) = %v, want ≈ %v (off by %.4f)",
				int(math.Round(loc)), float64(sz), c.want, float64(sz)-c.want)
		}
	}
}

func TestSizeAnchor(t *testing.T) {
	// CRU(median LOC) = 1.0; median of the lognormal is exp(μ).
	loc := int(math.Round(math.Exp(Mu)))
	closeTo(t, "anchor", float64(SizeOf(loc)), 1.0, 0.01)
}

func TestSizeBounds(t *testing.T) {
	floor := math.Pow(2, -2.5)
	ceil := math.Pow(2, 2.5)
	closeTo(t, "floor at 0", float64(SizeOf(0)), floor, 1e-12)
	closeTo(t, "floor at -1", float64(SizeOf(-1)), floor, 1e-12)
	if float64(SizeOf(1)) <= floor {
		t.Errorf("SizeOf(1) should be above floor, got %v", SizeOf(1))
	}
	huge := float64(SizeOf(10_000_000))
	if huge >= ceil {
		t.Errorf("SizeOf(10M) should be below ceiling %v, got %v", ceil, huge)
	}
	if ceil-huge > 0.001 {
		t.Errorf("SizeOf(10M) should be very close to ceiling %v, got %v", ceil, huge)
	}
}

func TestSizeMonotonic(t *testing.T) {
	prev := SizeOf(0)
	for loc := 1; loc <= 5000; loc++ {
		curr := SizeOf(loc)
		if curr < prev {
			t.Errorf("not monotonic at loc=%d: %v < %v", loc, curr, prev)
		}
		prev = curr
	}
}

// TestSizeBoundariesDerived verifies that the cached bucket boundaries
// are exactly the floored-to-even quintile cuts of the locked log-normal,
// confirming the package's "no magic numbers" property: change Mu/Sigma
// or len(sizes) and the boundaries follow.
func TestSizeBoundariesDerived(t *testing.T) {
	n := len(sizes)
	for i := 1; i < n; i++ {
		p := float64(i) / float64(n)
		raw := math.Exp(Mu + Sigma*probit(p))
		want := int(math.Floor(raw))
		if want%2 != 0 {
			want--
		}
		got := sizeBoundaries[i-1]
		if got != want {
			t.Errorf("sizeBoundaries[%d] = %d, want %d (raw=%v at p=%v)",
				i-1, got, want, raw, p)
		}
	}
}

func TestSizeStringLabels(t *testing.T) {
	cases := []struct {
		loc  int
		want string
	}{
		{0, "XS"}, {1, "XS"}, {6, "XS"},
		{7, "S"}, {20, "S"},
		{21, "M"}, {54, "M"},
		{55, "L"}, {162, "L"},
		{163, "XL"}, {100_000, "XL"},
	}
	for _, c := range cases {
		got := SizeOf(c.loc).String()
		if got != c.want {
			t.Errorf("SizeOf(%d).String() = %q, want %q", c.loc, got, c.want)
		}
	}
}

func TestCalculateComposition(t *testing.T) {
	loc := 34 // ≈ anchor
	sf := float64(SizeOf(loc))
	// Single owner, low risk: CRU == size factor.
	closeTo(t, "single owner low risk",
		Calculate(loc, 1.0, RiskLow), sf, 1e-12)
	// 50% ownership halves it.
	closeTo(t, "50% owner",
		Calculate(loc, 0.5, RiskLow), sf*0.5, 1e-12)
	// Medium risk 2x.
	closeTo(t, "medium risk",
		Calculate(loc, 1.0, RiskMedium), sf*2, 1e-12)
	// High risk 4x.
	closeTo(t, "high risk",
		Calculate(loc, 1.0, RiskHigh), sf*4, 1e-12)
}

func TestCalculatePanicsOnNilRisk(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Calculate(_, _, nil) did not panic")
			return
		}
		msg, ok := r.(string)
		if !ok {
			t.Errorf("panic value = %v (type %T), want string", r, r)
			return
		}
		if !strings.Contains(msg, "nil Risk") {
			t.Errorf("panic message = %q, want it to mention \"nil Risk\"", msg)
		}
	}()
	_ = Calculate(100, 1.0, nil)
}

func TestRiskTiers(t *testing.T) {
	// Doubling at each step: low → medium → high.
	if RiskMedium.Factor()/RiskLow.Factor() != 2.0 {
		t.Errorf("RiskMedium/RiskLow = %v, want 2.0",
			RiskMedium.Factor()/RiskLow.Factor())
	}
	if RiskHigh.Factor()/RiskMedium.Factor() != 2.0 {
		t.Errorf("RiskHigh/RiskMedium = %v, want 2.0",
			RiskHigh.Factor()/RiskMedium.Factor())
	}
	if RiskHigh.Factor()/RiskLow.Factor() != 4.0 {
		t.Errorf("RiskHigh/RiskLow = %v, want 4.0",
			RiskHigh.Factor()/RiskLow.Factor())
	}
}

func TestRiskStringLabels(t *testing.T) {
	if got := RiskLow.String(); got != "low" {
		t.Errorf("RiskLow.String() = %q, want %q", got, "low")
	}
	if got := RiskMedium.String(); got != "medium" {
		t.Errorf("RiskMedium.String() = %q, want %q", got, "medium")
	}
	if got := RiskHigh.String(); got != "high" {
		t.Errorf("RiskHigh.String() = %q, want %q", got, "high")
	}
}

// TestRiskIdentity verifies the three constants are distinct and
// comparable. Interface comparison works because all three values share
// the same unexported concrete type and have distinct field contents.
func TestRiskIdentity(t *testing.T) {
	if RiskLow == RiskMedium {
		t.Errorf("RiskLow == RiskMedium; want distinct")
	}
	if RiskMedium == RiskHigh {
		t.Errorf("RiskMedium == RiskHigh; want distinct")
	}
	if RiskLow == RiskHigh {
		t.Errorf("RiskLow == RiskHigh; want distinct")
	}
	// Assignment + identity round-trip.
	var r Risk = RiskMedium
	if r != RiskMedium {
		t.Errorf("round-trip Risk assignment lost identity: %v != %v", r, RiskMedium)
	}
}
