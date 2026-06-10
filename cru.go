// Package cru is the canonical implementation of the Code Review Unit (CRU)
// formula. Import this package to score pull requests from your own Go
// programs without pulling in the gh CLI extension wrapper.
//
// Quick start:
//
//	import "github.com/laserlemon/cru"
//
//	// A 250-line pull request, the reviewer owns 100 lines of it, low risk:
//	score := cru.Calculate(250, 100, cru.RiskLow)
//
//	// The size value carries both its factor and its label:
//	sz := cru.CalculateSize(250)
//	fmt.Println(sz)            // "XL"
//	fmt.Println(sz.Factor())   // 3.4499... (the size factor)
//
// All constants come from a locked log-normal fit of merged pull request
// sizes in a large monolithic GitHub repository with thousands of
// individual contributors.
//
// The unit is intentionally stable: a "CRU" today and a "CRU" five years
// from now both refer to the same fixed reference distribution. Like a
// "foot" as a unit of measurement, the value of the unit is in the
// unchanging standard, not in how closely it matches any current reality.
//
// CRU = size factor × ownership share × risk multiplier
//
//	size(L) = 2^(5·F(L) − 2.5)
//	F(L)    = Φ((ln L − μ) / σ)
//
// μ and σ are baked-in constants. Φ is the standard normal CDF.
// L is the pull request's line count (additions + deletions).
package cru

import "math"

// Locked baseline. DO NOT CHANGE without releasing a new major version.
// These values define what 1 CRU means.
const (
	// Mu (μ) and Sigma (σ) of the log-normal fit of merged pull request
	// line counts from a large monolithic GitHub repository with
	// thousands of individual contributors.
	Mu    = 3.526665
	Sigma = 1.867217
)

// Size is a pull request's size factor. The float64 value IS the factor
// used in the CRU formula; the categorical label (XS/S/M/L/XL) is derived
// from a floor-to-even quintile partition of the locked log-normal
// distribution and surfaced via String(). Read the factor via Factor(),
// which is equivalent to float64(s).
//
// Construct via CalculateSize. Direct float64 conversion (cru.Size(0.5))
// is legal but produces an arbitrary label via String() based on which
// bucket the equivalent line count falls into.
type Size float64

// Factor returns the size multiplier used in the CRU formula. Equivalent
// to float64(s); provided so Size and Risk read symmetrically (both have
// Factor() and String()).
func (s Size) Factor() float64 { return float64(s) }

// String returns the t-shirt size label for s, derived from the line
// count that would produce this factor. Buckets are labels only; the
// formula does not reference them.
func (s Size) String() string {
	// Invert size factor: F = (log2(s) + sizeRange/2) / sizeRange, then
	// look up the bucket by the line count at percentile F. This keeps
	// String honest: the label always matches what CalculateSize would
	// produce for the equivalent line count.
	if s <= 0 {
		return sizes[0]
	}
	f := (math.Log2(float64(s)) + sizeRange/2) / sizeRange
	lines := int(math.Round(math.Exp(Mu + Sigma*probit(f))))
	return sizeLabel(lines)
}

// CalculateSize returns the Size for a pull request of the given line
// count (additions + deletions).
//
// Returns the bounded floor (2^-2.5 ≈ 0.177) at lines ≤ 0 to keep the
// function total: even a typo carries some context cost.
func CalculateSize(lines int) Size {
	if lines <= 0 {
		return Size(math.Pow(2, -sizeRange/2))
	}
	// F(L) = Φ((ln L − μ) / σ) is the pull request's percentile rank in
	// the locked baseline distribution of merged pull request sizes. The
	// size factor is a doubling-rescaled function of that rank, anchored
	// so that the median pull request (F = 0.5) scores exactly 1.
	z := (math.Log(float64(lines)) - Mu) / Sigma
	f := 0.5 * (1 + math.Erf(z/math.Sqrt2))
	return Size(math.Pow(2, sizeRange*f-sizeRange/2))
}

// probit is Φ⁻¹, the inverse normal CDF. Used to derive bucket boundaries
// from quintile probabilities. Implemented via the standard relation
// Φ⁻¹(p) = √2 · erfinv(2p − 1).
func probit(p float64) float64 {
	return math.Sqrt2 * math.Erfinv(2*p-1)
}

// Risk is a pull request's risk tier. Three values exist: RiskLow, RiskMedium,
// RiskHigh. The interface is sealed (unexported isRisk method); external
// packages cannot construct alternative Risk values.
type Risk interface {
	// Factor returns the risk multiplier (1.0 / 2.0 / 4.0).
	Factor() float64
	// String returns the tier label ("low" / "medium" / "high").
	String() string
	// isRisk is a sealing method. It exists only to prevent external types
	// from satisfying Risk, ensuring the three canonical constants are the
	// only valid instances.
	isRisk()
}

// risk is the unexported concrete type backing the three Risk constants.
type risk struct {
	name   string
	factor float64
}

func (r risk) Factor() float64 { return r.factor }
func (r risk) String() string  { return r.name }
func (r risk) isRisk()         {}

// Risk tiers. Authors mark risk; everything else defaults to low. The
// three tiers double at each step (1× → 2× → 4×), giving the same span
// from low to high (4×) as exists between two adjacent size buckets.
//
// These values are comparable via == and switch: identity uniquely
// identifies the tier (no fourth value can exist), so callers can write
// switch r { case cru.RiskHigh: ... case cru.RiskMedium: ... default: ... }
// and the default branch covers low exhaustively.
//
// These are var rather than const because interface values cannot be
// const in Go. External packages cannot construct alternative Risk
// values (see the sealed Risk interface) but can technically reassign
// these vars in-process; treat them as immutable.
var (
	RiskLow    Risk = risk{name: "low", factor: 1.0}
	RiskMedium Risk = risk{name: "medium", factor: 2.0}
	RiskHigh   Risk = risk{name: "high", factor: 4.0}
)

// Calculate returns the full CRU for a single (reviewer, pull request) pair.
//
//	CRU = CalculateSize(totalLines) × (ownedLines / totalLines) × risk
//
// totalLines is the pull request's full additions + deletions. ownedLines
// is the portion this reviewer is responsible for (their CODEOWNERS-matched
// lines, deduplicated across direct @login and team memberships). risk
// is one of RiskLow, RiskMedium, RiskHigh.
//
// Returns 0 when totalLines == 0. Clamps ownedLines to [0, totalLines] so
// callers can't double-count overlap or pass a negative share. Panics
// if risk is nil; pass cru.RiskLow explicitly for the default tier.
func Calculate(totalLines, ownedLines int, risk Risk) float64 {
	if risk == nil {
		panic("cru: nil Risk; use RiskLow, RiskMedium, or RiskHigh")
	}
	if totalLines <= 0 {
		return 0
	}
	if ownedLines < 0 {
		ownedLines = 0
	} else if ownedLines > totalLines {
		ownedLines = totalLines
	}
	share := float64(ownedLines) / float64(totalLines)
	return float64(CalculateSize(totalLines)) * share * risk.Factor()
}

// --- bucket derivation ---------------------------------------------------

// Size labels. These are exactly the strings returned by Size.String() and
// are the only valid bucket label values. Untyped string constants so they
// drop into any string context (switch cases, JSON, formatting) without
// conversion.
const (
	SizeXS = "XS"
	SizeS  = "S"
	SizeM  = "M"
	SizeL  = "L"
	SizeXL = "XL"
)

// sizes is the canonical ordered list of bucket labels, smallest to
// largest. Everything downstream (boundary count, quintile cut
// probabilities, size factor range, CalculateSize lookup) derives from this list
// and len(sizes). Adding a bucket here automatically adjusts boundaries.
var sizes = [...]string{SizeXS, SizeS, SizeM, SizeL, SizeXL}

// sizeRange is the doubling range of the size factor across the F axis.
// Size = 2^(sizeRange · F − sizeRange/2), so a size factor spans
// 2^(-sizeRange/2) at F=0 to 2^(sizeRange/2) at F=1. Equal to len(sizes)
// for the locked 5-bucket calibration: every adjacent quintile median
// represents a doubling of CRU.
const sizeRange = float64(len(sizes))

// sizeBoundaries holds the floored-to-even quintile boundaries of the
// locked log-normal. sizeBoundaries[i] is the highest line count that
// still belongs to sizes[i]; anything above the last boundary is
// sizes[len-1]. Computed at init from Mu, Sigma, and len(sizes), so
// calibration changes propagate to bucket cut points automatically.
//
// Each boundary is exp(μ + σ · Φ⁻¹(i/N)) for i in 1..N-1, then floored
// down to the nearest even integer for human-friendly display.
var sizeBoundaries [len(sizes) - 1]int

func init() {
	n := len(sizes)
	for i := 1; i < n; i++ {
		p := float64(i) / float64(n)
		raw := math.Exp(Mu + Sigma*probit(p))
		b := int(math.Floor(raw))
		if b%2 != 0 {
			b-- // floor to even
		}
		sizeBoundaries[i-1] = b
	}
}

// sizeLabel returns the bucket label for a given line count. Used by
// Size.String to keep labels and CalculateSize consistent.
func sizeLabel(lines int) string {
	for i, b := range sizeBoundaries {
		if lines <= b {
			return sizes[i]
		}
	}
	return sizes[len(sizes)-1]
}
