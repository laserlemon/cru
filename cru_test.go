package cru

import (
	"math"
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

// TestSizeFactorDoublingTargets — the headline property: CRU doubles at the
// median of each shirt-size quintile of the baseline distribution. These five
// targets are why the formula was derived; if they ever drift, the formula
// has been changed by accident and SHOULD fail loudly.
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
		// re-apply SizeFactor.
		// L = exp(μ + σ · Φ⁻¹(p))
		z := probit(c.percentile)
		loc := math.Exp(Mu + Sigma*z)
		got := math.Pow(2, 5*c.percentile-2.5)
		closeTo(t, "doubling target", got, c.want, 1e-12)
		// Sanity: SizeFactor on the actual rounded LOC is close (we lose
		// a hair to the discrete integer cast).
		sf := SizeFactor(int(math.Round(loc)))
		if math.Abs(sf-c.want) > 0.01 {
			t.Errorf("SizeFactor(%d) = %v, want ≈ %v (off by %.4f)",
				int(math.Round(loc)), sf, c.want, sf-c.want)
		}
	}
}

func TestSizeFactorAnchor(t *testing.T) {
	// CRU(median LOC) = 1.0
	// median of the lognormal is exp(μ).
	loc := int(math.Round(math.Exp(Mu)))
	closeTo(t, "anchor", SizeFactor(loc), 1.0, 0.01)
}

func TestSizeFactorBounds(t *testing.T) {
	floor := math.Pow(2, -2.5)
	ceil := math.Pow(2, 2.5)
	closeTo(t, "floor at 0", SizeFactor(0), floor, 1e-12)
	closeTo(t, "floor at -1", SizeFactor(-1), floor, 1e-12)
	if SizeFactor(1) <= floor {
		t.Errorf("SizeFactor(1) should be above floor, got %v", SizeFactor(1))
	}
	huge := SizeFactor(10_000_000)
	if huge >= ceil {
		t.Errorf("SizeFactor(10M) should be below ceiling %v, got %v", ceil, huge)
	}
	if ceil-huge > 0.001 {
		t.Errorf("SizeFactor(10M) should be very close to ceiling %v, got %v", ceil, huge)
	}
}

func TestSizeFactorMonotonic(t *testing.T) {
	prev := SizeFactor(0)
	for loc := 1; loc <= 5000; loc++ {
		curr := SizeFactor(loc)
		if curr < prev {
			t.Errorf("not monotonic at loc=%d: %v < %v", loc, curr, prev)
		}
		prev = curr
	}
}

func TestBucketBoundaries(t *testing.T) {
	cases := []struct {
		loc  int
		want Size
	}{
		{0, SizeXS}, {1, SizeXS}, {6, SizeXS},
		{7, SizeS}, {20, SizeS},
		{21, SizeM}, {54, SizeM},
		{55, SizeL}, {162, SizeL},
		{163, SizeXL}, {100_000, SizeXL},
	}
	for _, c := range cases {
		if got := Bucket(c.loc); got != c.want {
			t.Errorf("Bucket(%d) = %v, want %v", c.loc, got, c.want)
		}
	}
}

func TestScoreComposition(t *testing.T) {
	loc := 34 // ≈ anchor
	// Single owner, low risk: CRU ≈ 1.0
	closeTo(t, "single owner low risk", Score(loc, 1.0, RiskLow), SizeFactor(loc), 1e-12)
	// 50% ownership halves it.
	closeTo(t, "50% owner", Score(loc, 0.5, RiskLow), SizeFactor(loc)*0.5, 1e-12)
	// High risk 4x.
	closeTo(t, "high risk", Score(loc, 1.0, RiskHigh), SizeFactor(loc)*4, 1e-12)
}

// probit is Φ⁻¹ for the test. Implemented via Newton on erf for accuracy.
func probit(p float64) float64 {
	// Use math.Erfinv via the standard relation: Φ⁻¹(p) = √2 · erfinv(2p − 1)
	return math.Sqrt2 * math.Erfinv(2*p-1)
}
