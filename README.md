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
CRU = size factor × ownership share × risk multiplier
```

### Size factor

Bigger pull requests are harder to review, but not linearly: a 1000-line
change isn't 100× the work of a 10-line one. The factor anchors at 1.0
for a typical pull request and doubles at each step up the t-shirt scale
(XS, S, M, L, XL). It ranges from about 0.18 for a typo to about 5.66
for an enormous refactor. The exact curve comes from a log-normal fit
of merged pull request sizes in a large monolithic GitHub repository,
locked once and never re-tuned.

### Ownership share

A number between 0 and 1: how much of the pull request's lines you're
on the hook for. If you own all 250 of its lines via CODEOWNERS, your
share is 1.0 and you carry the full size factor. If you own 100 of 250,
your share is 0.4 and you carry 40% of the work. Shared ownership across
teams gets deduplicated so nobody is double-counted.

### Risk multiplier

Three tiers: 1× for the default (low), 2× for medium, 4× for high.
Authors mark this on pull requests that touch sensitive paths (auth
code, migration scripts, billing logic), where the same line count
deserves more careful eyes. Most code is unmarked and lands at low.

## API

### Functions

| | |
|---|---|
| `cru.Calculate(totalLines, ownedLines int, risk Risk) float64` | Full CRU for one (reviewer, pull request) pair |
| `cru.CalculateSize(lines int) Size` | Size factor plus derived t-shirt label |

### Types

| | |
|---|---|
| `cru.Size` | `float64` named type; `String()` returns `"XS"`/`"S"`/`"M"`/`"L"`/`"XL"` |
| `cru.Risk` | Sealed interface; only the three constants below satisfy it. Comparable via `==` and `switch` |

### Constants

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
