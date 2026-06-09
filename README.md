# cru

The canonical Go implementation of the Code Review Unit (CRU) formula.

```go
import "github.com/laserlemon/cru"

// Compute a 250-LOC PR's CRU, 100% owned by the reviewer, low risk:
cru.Calculate(250, 250, cru.RiskLow)

// The size value carries both its factor and its label:
sz := cru.CalculateSize(250)
fmt.Println(sz)            // "XL"
fmt.Println(float64(sz))   // 3.4499...
```

## Formula

```
CRU = size × ownership × risk

size(L) = 2^(5·F(L) − 2.5)
F(L)    = Φ((ln L − μ) / σ)
```

Where `μ = 3.526665` and `σ = 1.867217` come from a locked log-normal fit of
merged PR sizes from a large monolithic GitHub repository with thousands of
individual contributors.

## What you get

| Function | Purpose |
|---|---|
| `cru.CalculateSize(loc int) Size` | Size factor + derived t-shirt label |
| `cru.Calculate(totalLOC, ownedLOC int, risk Risk) float64` | Full CRU |

| Type | Notes |
|---|---|
| `cru.Size` | `float64` named type. `String()` returns "XS"/"S"/"M"/"L"/"XL". |
| `cru.Risk` | Sealed interface. The only valid values are the three constants below. |

| Constant | Value | |
|---|---|---|
| `cru.Mu` | `3.526665` | |
| `cru.Sigma` | `1.867217` | |
| `cru.RiskLow` | `1.0` | default risk multiplier |
| `cru.RiskMedium` | `2.0` | author-marked medium-risk PRs |
| `cru.RiskHigh` | `4.0` | author-marked high-risk PRs |

The shirt-size buckets (XS/S/M/L/XL) partition the locked log-normal into
five equal-mass quintiles. Boundaries are derived at package init from
`Mu`, `Sigma`, and `len(sizes)`, so calibration changes propagate
automatically. Current cuts: (0, 6], (6, 20], (20, 54], (54, 162], (162, ∞).

## Stability

The constants are locked. A "CRU" measured today and a "CRU" measured five
years from now reference the same fixed baseline distribution, so trends in
review effort are measurable rather than baked into the metric.

## License

MIT
