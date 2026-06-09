# laserlemon/cru

The canonical Go implementation of the Code Review Unit (CRU) formula.

[![Made by laserlemon](https://img.shields.io/badge/laser-lemon-fc0?style=flat-square)](https://github.com/laserlemon)
[![Latest tag](https://img.shields.io/github/v/tag/laserlemon/cru?style=flat-square&label=tag)](https://github.com/laserlemon/cru/tags)
[![CI](https://img.shields.io/github/actions/workflow/status/laserlemon/cru/ci.yml?style=flat-square)](https://github.com/laserlemon/cru/actions/workflows/ci.yml)

A CRU is a unit of code-review effort. One CRU equals the work of reviewing
a typical pull request, where "typical" is anchored to a locked reference
distribution of real merged pull requests. The unit is stable across time:
a CRU today and a CRU five years from now mean the same thing, the way a
foot has always meant a foot.

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
CRU = size × ownership × risk

size(L) = 2^(5·F(L) − 2.5)
F(L)    = Φ((ln L − μ) / σ)
```

`Φ` is the standard normal CDF. `L` is the pull request's line count
(additions + deletions, i.e. total diff churn). `μ = 3.526665` and
`σ = 1.867217` are baked-in constants from a log-normal fit of merged
pull request sizes in a large monolithic GitHub repository with thousands
of individual contributors.

## API

**Functions**

| | |
|---|---|
| `cru.Calculate(totalLines, ownedLines int, risk Risk) float64` | Full CRU for one (reviewer, pull request) pair |
| `cru.CalculateSize(lines int) Size` | Size factor plus derived t-shirt label |

**Types**

| | |
|---|---|
| `cru.Size` | `float64` named type; `String()` returns `"XS"`/`"S"`/`"M"`/`"L"`/`"XL"` |
| `cru.Risk` | Sealed interface; only the three constants below satisfy it. Comparable via `==` and `switch` |

**Constants**

```go
cru.Mu    = 3.526665     // log-normal μ
cru.Sigma = 1.867217     // log-normal σ

cru.RiskLow    // factor 1.0, the default
cru.RiskMedium // factor 2.0, author-marked
cru.RiskHigh   // factor 4.0, author-marked

cru.SizeXS // "XS"
cru.SizeS  // "S"
cru.SizeM  // "M"
cru.SizeL  // "L"
cru.SizeXL // "XL"
```

The size constants are exactly the strings `Size.String()` returns. Switch on
them in downstream code instead of bare string literals.

## Calibration

The shirt-size buckets partition the locked log-normal into five equal-mass
quintiles. Boundaries are derived at package init from `Mu`, `Sigma`, and the
bucket count, so changing the calibration would propagate to bucket cuts
automatically.

| Bucket | Lines |
|---|---|
| XS | (0, 6] |
| S | (6, 20] |
| M | (20, 54] |
| L | (54, 162] |
| XL | (162, ∞) |

The constants are locked on purpose. Like a foot, the value of the unit is in
the unchanging standard, not in how closely it matches any current reality.
Trends in review effort become measurable instead of getting absorbed into a
moving baseline.

## License

MIT
