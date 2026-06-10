# laserlemon/cru

The canonical Go implementation of the Code Review Unit (CRU) formula.

[![Made by laserlemon](https://img.shields.io/badge/laser-lemon-fc0?style=flat-square)](https://github.com/laserlemon)
[![Latest tag](https://img.shields.io/github/v/tag/laserlemon/cru?style=flat-square&label=tag)](https://github.com/laserlemon/cru/tags)
[![CI](https://img.shields.io/github/actions/workflow/status/laserlemon/cru/ci.yml?style=flat-square)](https://github.com/laserlemon/cru/actions/workflows/ci.yml)

A CRU is a unit of code-review effort. One CRU equals the work of reviewing
a typical pull request, where "typical" is anchored to a locked reference
distribution of real merged pull requests. The unit is stable across time:
a CRU today and a CRU five years from now mean the same thing.

This package is the formula by itself. To measure code review effort for
GitHub pull requests, see [`gh-cru`](https://github.com/laserlemon/gh-cru),
a GitHub CLI extension.

## Install

```bash
go get github.com/laserlemon/cru
```

## Quick start

```go
import "github.com/laserlemon/cru"

// A 250-line pull request, the reviewer owns 100 lines of it, low risk.
cru.Calculate(250, 100, cru.RiskLow) // => 1.3800...

// The size value carries both its factor and its label.
sz := cru.CalculateSize(250)
sz.Factor() // => 3.4499...
sz.String() // => "XL"
```

## Formula

```
CRU = size factor × ownership share × risk multiplier
```

### Size factor

Bigger pull requests are harder to review, but not linearly: a 1000-line
review isn't 100× the effort of a 10-line review. The size factor is a
smooth, continuous function of line count, anchored at 1.0 for a typical
pull request. It ranges from about 0.18 for a one-liner to about 5.66
for an enormous refactor. The exact curve comes from a log-normal fit
of merged pull request sizes in a large monolithic GitHub repository,
locked once and never re-tuned.

The t-shirt scale (XS, S, M, L, XL) is post-hoc labeling for display.
Each step up the scale corresponds to a doubling of the size factor,
but the underlying number is continuous, not bucketed.

### Ownership share

A number between 0 and 1: how many of the pull request's lines you're
responsible for. If you own all of the pull request's changed files via
CODEOWNERS, your share is 1.0 and you carry the full size factor. If you
own 100 of 250 changed lines, your ownership share is 0.4.

### Risk multiplier

Three tiers: 1× for low (default), 2× for medium, 4× for high.
Authors may tag pull requests that touch sensitive paths, where the
same line count deserves more rigorous review. By default, an untagged
pull request is considered low risk.

## API

### Functions

#### `cru.Calculate`

```go
func Calculate(totalLines, ownedLines int, risk Risk) float64
```

Returns the full CRU for a single (reviewer, pull request) pair. This is
the headline function: `CalculateSize(totalLines).Factor() × (ownedLines
/ totalLines) × risk.Multiplier()`. Returns 0 when `totalLines` is 0. Clamps
`ownedLines` to `[0, totalLines]` so callers can't double-count overlap
or pass a negative share. Panics if `risk` is nil.

#### `cru.CalculateSize`

```go
func CalculateSize(lines int) Size
```

Returns just the size factor for a given line count, without ownership
or risk applied. Useful when you want the raw size measurement (the
"how big is this PR" question) decoupled from any particular reviewer.
At `lines ≤ 0` returns the bounded floor (about 0.18) instead of zero,
keeping the function total.

### Types

#### `cru.Size`

```go
type Size float64
```

A pull request's size factor. The `float64` value is the factor itself;
the categorical t-shirt label is derived from it and surfaced via
`String()`. Read the numeric value via `Factor()`, which is equivalent
to `float64(s)`.

Construct via `CalculateSize`. Direct conversion (`cru.Size(0.5)`) is
legal but the resulting label is whatever bucket the equivalent line
count falls into.

#### `cru.Risk`

```go
type Risk interface { ... }
```

A pull request's risk tier. The interface is sealed: only the three
exported constants (`RiskLow`, `RiskMedium`, `RiskHigh`) satisfy it,
and external packages can't construct alternatives. Values are
comparable via `==` and `switch`, so a callsite like:

```go
switch r {
case cru.RiskHigh:   // ...
case cru.RiskMedium: // ...
default:             // low
}
```

is provably exhaustive.

### Constants

```go
cru.Mu    = 3.526665     // log-normal μ
cru.Sigma = 1.867217     // log-normal σ

cru.RiskLow    // factor 1.0, the default
cru.RiskMedium // factor 2.0, author-tagged
cru.RiskHigh   // factor 4.0, author-tagged

cru.SizeXS // "XS"
cru.SizeS  // "S"
cru.SizeM  // "M"
cru.SizeL  // "L"
cru.SizeXL // "XL"
```

The size constants are exactly the strings `Size.String()` returns. Switch on
them in downstream code instead of bare string literals.

## Calibration

The size factor is a continuous function, but t-shirt labels need
cutoff points. Those cutoffs are the four equal-mass quintile
boundaries of the locked log-normal: the lines at the 20th, 40th,
60th, and 80th percentiles of the baseline distribution. Each bucket
holds exactly one-fifth of the historical pull request mass.

The raw cuts are computed at package init from `Mu` and `Sigma`, then
each is floored down to the nearest even integer:

| Size | Percentile range | Raw bounds | Raw mass | Final bounds | Final mass |
|---|---|---|---|---|---|
| XS | (0%, 20%]   | (0, 7.07]       | 20.00% | (0, 6]    | 17.64% |
| S  | (20%, 40%]  | (7.07, 21.19]   | 20.00% | (6, 20]   | 21.17% |
| M  | (40%, 60%]  | (21.19, 54.58]  | 20.00% | (20, 54]  | 20.97% |
| L  | (60%, 80%]  | (54.58, 163.72] | 20.00% | (54, 162] | 20.06% |
| XL | (80%, 100%] | (163.72, ∞)     | 20.00% | (162, ∞)  | 20.16% |

Why even? Real pull requests have jagged line counts: peaks at even
counts (every full-line edit contributes both a `-` and a `+` to the
diff), valleys at odd. By flooring every boundary to an even number,
every bucket spans an even number of lines and so contains the same
count of peaks and valleys. Labels stay balanced no matter how jagged
the underlying distribution is, at the small cost of a one- or
two-point deviation from perfect equal-fifths.

The constants are locked. Like a foot, the value of the unit is in the
unchanging standard, not in how closely it matches any current reality.

## License

MIT
