# scripts

Supporting scripts for the `cru` package.

## `render-graphs.py`

Renders the eight visualizations embedded in the project README. Each
graph is rendered in both light and dark variants and saved under
`../docs/img/<name>-{light,dark}.png`. GitHub's `<picture>` element in
the README selects the right one for the reader's color scheme.

### Requirements

- Python 3.10+
- `matplotlib`, `numpy`, `scipy`

```bash
pip install matplotlib numpy scipy
```

### Run

```bash
python3 scripts/render-graphs.py
```

### When to re-render

Re-run after changing the locked constants `Mu` or `Sigma` in `cru.go`,
or after editing the bucket boundary derivation. The script keeps a
copy of the constants at the top; if `cru.go` changes, update them
there too.
