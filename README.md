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
cru.Calculate(250, 100, cru.RiskLow) // => 1.2510...

// The size value carries both its factor and its label.
sz := cru.CalculateSize(250)
sz.Factor() // => 3.1275...
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
for an enormous refactor.

The t-shirt scale (XS, S, M, L, XL) is post-hoc labeling for display.
Each step up the scale corresponds to a doubling of the size factor,
but the underlying number is continuous, not bucketed.

For the full derivation, see [🤓 For the math nerds…](#-for-the-math-nerds)
at the bottom of this README.

### Ownership share

A number between 0 and 1: how many of the pull request's lines you're
responsible for. If you own all of the pull request's changed files via
CODEOWNERS, your share is 1.0 and you carry the full size factor. If you
own 100 of 250 changed lines, your ownership share is 0.4.

### Risk multiplier

Three tiers: 1× for low (default), 2× for medium, 4× for high.
Authors may flag pull requests that touch sensitive paths, where the
same line count deserves more rigorous review. By default, an unflagged
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
cru.Mu    = 3.808551     // log-normal μ
cru.Sigma = 1.802600     // log-normal σ

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

## 🤓 For the math nerds…

The size factor is a continuous function of line count anchored to a
fixed reference distribution. Both halves (the distribution and the
mapping onto it) are locked, so the unit is stable across time.

### The locked baseline

Pull request sizes follow a log-normal distribution: lots of small
diffs, a long tail of large ones, and a median far enough below the
peak to mislead anyone who reads the chart by eye.

On a linear x-axis the distribution looks like a cliff. Most of what
your eye reads as empty space to the right is actually half of all
pull requests. The median sits well off in the flatlands, hidden by
the visual weight of the spike on the left:

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="/docs/img/distribution-linear-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="/docs/img/distribution-linear-light.png">
  <img alt="The locked log-normal distribution of pull request sizes on a linear x-axis. Long, flat tail. The median sits at 45 lines, far to the right of the visual peak." src="/docs/img/distribution-linear-light.png">
</picture>

Put the x-axis on a log scale and it turns into a familiar bell shape
with the median sitting near the visual center:

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="/docs/img/distribution-log-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="/docs/img/distribution-log-light.png">
  <img alt="The same log-normal distribution with a log-scaled x-axis. The shape is bell-like and the median at 45 lines now sits where the eye expects." src="/docs/img/distribution-log-light.png">
</picture>

The parameters are `μ = 3.808551` and `σ = 1.802600`, fit from real
merged pull requests in a large monolithic GitHub repository. They're
locked once and never re-tuned. Like a foot, the value of the unit is
in the unchanging standard, not in how closely it matches any current
reality.

### Five buckets, equal mass

The shirt-size labels (XS, S, M, L, XL) are quintiles of the locked
distribution. The four cuts that separate them are the 20th, 40th,
60th, and 80th percentiles, so every bucket holds about 20% of all
pull requests:

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="/docs/img/quintiles-log-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="/docs/img/quintiles-log-light.png">
  <img alt="The log-scaled distribution divided into five equal-mass quintiles, labeled XS through XL. Boundaries at p20, p40, p60, p80." src="/docs/img/quintiles-log-light.png">
</picture>

On a linear x-axis those quintile boundaries crowd into the left third
of the chart, with XL claiming everything to the right. Same math,
different view:

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="/docs/img/quintiles-linear-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="/docs/img/quintiles-linear-light.png">
  <img alt="The same five quintiles shown on a linear x-axis. XS, S, and M crowd the left edge; XL covers most of the visible range." src="/docs/img/quintiles-linear-light.png">
</picture>

### Building the size factor, one axis at a time

The buckets are the target. The size factor is the smooth, continuous
function that lands every bucket median on a doubling: XS → 0.25,
S → 0.5, M → 1.0, L → 2.0, XL → 4.0. Five anchors, five doublings,
ratio of 16× from smallest to largest. Everything else (the curve, the
floor, the ceiling) falls out of the math.

Start with the picture from before, plus a dot at the median of each
bucket sitting at its target size factor. The y-axis is the size factor
itself, linear from 0 to 6:

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="/docs/img/derivation-frame1-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="/docs/img/derivation-frame1-light.png">
  <img alt="Step 1: log-LOC x-axis, linear size factor y-axis. Five colored dots at each bucket median, labeled with their target doubling factor: 0.25, 0.5, 1, 2, 4." src="/docs/img/derivation-frame1-light.png">
</picture>

Same dots, linear x-axis. The XS, S, and M dots all crush together on
the left edge:

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="/docs/img/derivation-frame1-linear-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="/docs/img/derivation-frame1-linear-light.png">
  <img alt="Step 1 with a linear x-axis instead of log: the small-bucket dots all crowd against the left edge while XL drifts far to the right." src="/docs/img/derivation-frame1-linear-light.png">
</picture>

Switching the x-axis from line count to percentile evens the spacing.
By construction the bucket medians sit at the 10th, 30th, 50th, 70th,
and 90th percentile, so the dots are evenly distributed across the
chart:

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="/docs/img/derivation-frame2-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="/docs/img/derivation-frame2-light.png">
  <img alt="Step 2: x-axis switched to percentile. Same five dots, now evenly spaced at p10, p30, p50, p70, p90." src="/docs/img/derivation-frame2-light.png">
</picture>

Now switch the y-axis to log₂. The y-ticks are still at the doubling
levels (0.25, 0.5, 1, 2, 4), but each step is now equispaced because
they're all powers of two. The dots are colinear:

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="/docs/img/derivation-frame3-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="/docs/img/derivation-frame3-light.png">
  <img alt="Step 3: y-axis switched to log₂. The size factor ticks are labeled as powers of two (2⁻², 2⁻¹, 2⁰, 2¹, 2²). The five dots now lie on a perfectly straight line." src="/docs/img/derivation-frame3-light.png">
</picture>

Draw the line. With the y-axis stretched exactly half a doubling beyond
each endpoint, the line goes corner to corner of the chart. This is the
entire formula. Every dot is on it. Every 20 percentile points equals
one doubling of the size factor:

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="/docs/img/derivation-frame4-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="/docs/img/derivation-frame4-light.png">
  <img alt="Step 4: a blue line drawn corner to corner of the chart. The line threads through every dot. The size factor formula, in its native log₂ × percentile space." src="/docs/img/derivation-frame4-light.png">
</picture>

In one expression:

```
size factor = 2 ^ (5·F(L) − 2.5)
```

where `F(L) = Φ((ln L − μ) / σ)` is the pull request's percentile rank
in the locked distribution. The `5·F − 2.5` rescaling takes a percentile
in `[0, 1]` and turns it into a log₂ exponent in `[−2.5, +2.5]`, hence
the floor of `2⁻²·⁵ ≈ 0.18` and the ceiling of `2²·⁵ ≈ 5.66`.

Now unwind the axes back to LOC and linear size factor and watch the
straight line bend into the curve everyone recognizes.

Y-axis from log₂ back to linear: the formula becomes exponential in
percentile, hugging the floor for small pull requests and racing
upward through M and beyond:

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="/docs/img/derivation-frame2-line-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="/docs/img/derivation-frame2-line-light.png">
  <img alt="Unwinding the y-axis: percentile x, linear size factor y. The straight line is now an exponential curve. Dots still in place." src="/docs/img/derivation-frame2-line-light.png">
</picture>

X-axis from percentile back to log LOC: the exponential turns into the
classic logistic-shaped S-curve. The floor at the left and the
approach to the ceiling at the right both become visible:

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="/docs/img/derivation-frame1-log-line-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="/docs/img/derivation-frame1-log-line-light.png">
  <img alt="Unwinding the x-axis: log-LOC x, linear size factor y. The curve takes on its familiar S-shape with floor at the left, ceiling approached at the right." src="/docs/img/derivation-frame1-log-line-light.png">
</picture>

X-axis from log back to linear LOC: the final form. The same curve,
shown the way most people first see it: line count on the bottom,
size factor on the side, both linear. The same five dots still sit on
the same five doublings:

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="/docs/img/derivation-frame1-linear-line-dark.png">
  <source media="(prefers-color-scheme: light)" srcset="/docs/img/derivation-frame1-linear-line-light.png">
  <img alt="Unwound completely: linear x and linear y. The size factor curve in its native shape, with the five anchored dots still on their doublings." src="/docs/img/derivation-frame1-linear-line-light.png">
</picture>

### Why the bucket boundaries are even integers

The four cuts that separate the shirt sizes are raw quintile values
that don't land on round numbers (about 9.89, 28.56, 71.18, 205.54).
Real pull requests, though, have jagged line counts: peaks at even
counts (a one-line edit contributes both a `-` and a `+` to the diff)
and valleys at odd counts. Cutting the line count at an odd number
gives one neighboring bucket an extra peak the other doesn't get.

So the raw cuts get rounded to the nearest even integer, keeping
peaks and valleys distributed evenly across all five buckets:

| Size | Percentile range | Raw bound | Raw mass | Final bound | Final mass |
|---|---|---|---|---|---|
| XS | (0%, 20%]   | (0, 9.89]       | 20.00% | (0, 10]    | 20.17% |
| S  | (20%, 40%]  | (9.89, 28.56]   | 20.00% | (10, 28]   | 19.41% |
| M  | (40%, 60%]  | (28.56, 71.18]  | 20.00% | (28, 72]   | 20.67% |
| L  | (60%, 80%]  | (71.18, 205.54] | 20.00% | (72, 206]  | 19.79% |
| XL | (80%, 100%] | (205.54, ∞)     | 20.00% | (206, ∞)   | 19.97% |

Every final bucket lands within one percentage point of perfect
equal-fifths, with the rounding artifact spread invisibly across the
distribution instead of concentrated in any one bucket.

---

The graphs above are rendered by [`scripts/render-graphs.py`](scripts/render-graphs.py)
using the GitHub Primer color palette. Backgrounds are transparent so
the chart text reads against either light or dark themes; the colored
shirt-size bands sit at constant alpha and work on both. Re-run after
any change to `Mu` or `Sigma` to keep the visuals in sync with the
code.
