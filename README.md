# cru

The canonical Go implementation of the Code Review Unit (CRU) formula.

```go
import "github.com/laserlemon/cru"

// Score a 250-LOC PR, 100% owned by the reviewer, low risk:
score := cru.Score(250, 1.0, cru.RiskLow)

// Just the size factor (input to your own composition):
sf := cru.SizeFactor(250)

// T-shirt bucket label for a PR:
bucket := cru.Bucket(250) // -> cru.SizeXL
```

## Formula

```
CRU = size_factor × ownership_share × risk

size_factor(L) = 2^(5·F(L) − 2.5)
F(L)           = Φ((ln L − μ) / σ)
```

Where `μ = 3.526665` and `σ = 1.867217` come from the locked log-normal fit
of github/github merged PRs before the Copilot rollout (n = 65,609).

## What you get

| Function | Purpose |
|---|---|
| `cru.SizeFactor(loc int) float64` | Just the size factor — bounded ~0.18 to ~5.66 |
| `cru.Score(loc int, ownership, risk float64) float64` | Full CRU |
| `cru.Bucket(loc int) cru.Size` | T-shirt label (XS / S / M / L / XL) |
| `cru.Percentile(loc int) float64` | PR's percentile rank in the baseline |

| Constant | Value | |
|---|---|---|
| `cru.Mu` | `3.526665` | |
| `cru.Sigma` | `1.867217` | |
| `cru.RiskLow` | `1.0` | default risk multiplier |
| `cru.RiskHigh` | `4.0` | author-marked high-risk PRs |
| `cru.FormulaVersion` | `"1.0.0"` | bump on any constant change |

## Stability

The constants are locked. A "CRU" measured today and a "CRU" measured five
years from now reference the same fixed baseline distribution, so trends in
review effort are measurable rather than baked into the metric.

## License

MIT
