// Package cru is the canonical implementation of the Code Review Unit (CRU)
// formula. Import this package to score PRs from your own Go programs without
// pulling in the gh CLI extension wrapper.
//
// Quick start:
//
//	import "github.com/laserlemon/cru"
//
//	// Score a 250-LOC PR, 100% owned by the reviewer, low risk:
//	score := cru.Score(250, 1.0, cru.RiskLow)
//
//	// Just the size factor (input to your own composition):
//	sf := cru.SizeFactor(250)
//
//	// T-shirt bucket label:
//	bucket := cru.Bucket(250) // -> cru.SizeXL
//
// All constants come from the locked log-normal fit of github/github merged
// PRs before the Copilot rollout. See the CALIBRATION docs in the gh-cru
// repository for the derivation.
//
// The unit is intentionally stable: a "CRU" today and a "CRU" five years
// from now both refer to the same fixed reference distribution. Like a
// "foot" as a unit of measurement, the value of the unit is in the
// unchanging standard, not in how closely it matches any current reality.
//
// CRU = size_factor × ownership_share × risk
//
//	size_factor(L) = 2^(5·F(L) − 2.5)
//	F(L) = Φ((ln L − μ) / σ)
//
// μ and σ are baked-in constants. Φ is the standard normal CDF.
// L is the PR's LOC (additions + deletions).
package cru

import "math"

// Locked baseline. DO NOT CHANGE without bumping the formula version.
// These values define what 1 CRU means.
const (
	// Mu (μ) and Sigma (σ) of the log-normal fit of pre-Copilot
	// github/github merged PR sizes (n = 65,609; merged < 2025-05-23).
	Mu    = 3.526665
	Sigma = 1.867217

	// Risk multipliers. Authors mark high-risk PRs; everything else is unit.
	RiskLow  = 1.0
	RiskHigh = 4.0

	// FormulaVersion identifies this calibration. Bump on any constant change.
	FormulaVersion = "1.0.0"
)

// SizeFactor returns the size factor for a PR of the given LOC.
//
// Returns the bounded floor (2^-2.5 ≈ 0.177) at L ≤ 0 to keep the function
// total: even a typo carries some context cost.
func SizeFactor(loc int) float64 {
	if loc <= 0 {
		return math.Pow(2, -2.5)
	}
	f := percentile(float64(loc))
	return math.Pow(2, 5*f-2.5)
}

// Percentile returns F(L) = Φ((ln L − μ) / σ), the PR's percentile rank in the
// locked baseline distribution. Exposed for explainability / debugging.
func Percentile(loc int) float64 {
	if loc <= 0 {
		return 0
	}
	return percentile(float64(loc))
}

func percentile(loc float64) float64 {
	z := (math.Log(loc) - Mu) / Sigma
	return 0.5 * (1 + math.Erf(z/math.Sqrt2))
}

// Score returns the full CRU for a single (reviewer, PR) pair.
//
//	cru = SizeFactor(loc) × ownership × risk
//
// ownership is owned_loc / total_loc in [0, 1].
// risk is RiskLow (1.0) for low / unmarked PRs or RiskHigh (4.0) for high risk.
func Score(loc int, ownership, risk float64) float64 {
	return SizeFactor(loc) * ownership * risk
}

// Size is a shirt-size bucket. Buckets are labels only; the formula does not
// reference them. Boundaries are floored-to-even quintile boundaries of the
// locked log-normal.
type Size string

const (
	SizeXS Size = "XS"
	SizeS  Size = "S"
	SizeM  Size = "M"
	SizeL  Size = "L"
	SizeXL Size = "XL"
)

// Bucket returns the t-shirt bucket label for a given LOC.
//
//	XS: (0, 6]      S: (6, 20]     M: (20, 54]
//	L:  (54, 162]   XL: (162, ∞)
func Bucket(loc int) Size {
	switch {
	case loc <= 6:
		return SizeXS
	case loc <= 20:
		return SizeS
	case loc <= 54:
		return SizeM
	case loc <= 162:
		return SizeL
	default:
		return SizeXL
	}
}
