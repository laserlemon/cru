#!/usr/bin/env python3
"""
Render the cru README visualization suite.

Generates transparent-background SVGs in both light and dark variants so
the README can use <picture> elements without bleeding the canvas color.

Run:
    python3 scripts/render-graphs.py

Outputs (24 files in docs/img/):

    Distribution shape (with median marker)
      distribution-linear-{light,dark}.svg
      distribution-log-{light,dark}.svg

    Distribution sliced into shirt-size quintiles
      quintiles-linear-{light,dark}.svg
      quintiles-log-{light,dark}.svg

    Size factor derivation, step by step
      derivation-frame1-{light,dark}.svg         (log LOC X, linear SF Y)
      derivation-frame1-linear-{light,dark}.svg  (linear LOC X, linear SF Y)
      derivation-frame2-{light,dark}.svg         (percentile X, linear SF Y)
      derivation-frame3-{light,dark}.svg         (percentile X, log2 SF Y)
      derivation-frame4-{light,dark}.svg         (frame 3 + corner-to-corner line)
      derivation-frame2-line-{light,dark}.svg    (frame 2 + formula curve)
      derivation-frame1-log-line-{light,dark}.svg    (frame 1 + formula curve)
      derivation-frame1-linear-line-{light,dark}.svg (frame 1 linear + formula curve)

The math is identical to cru.go. The constants are duplicated here for
plot-rendering convenience; if cru.go changes, update both.
"""

from __future__ import annotations

import math
from dataclasses import dataclass
from pathlib import Path

import matplotlib.pyplot as plt
import numpy as np
from matplotlib.ticker import FixedLocator, FuncFormatter
from scipy.stats import norm

# ----------------------------------------------------------------------
# Locked constants (mirror cru.go)
# ----------------------------------------------------------------------
MU = 3.808551
SIGMA = 1.802600
SIZE_RANGE = 5.0

BUCKETS = ["XS", "S", "M", "L", "XL"]

# Bucket palette: GitHub Primer brand scales, theme-agnostic.
# Same hex values render legibly on both light and dark canvases (chosen
# from the Primer brand swatches at primer.style/brand/primitives/color).
#   XS = green-5, S = lime-5, M = yellow-3, L = orange-3, XL = red-4
BUCKET_COLORS = ["#0fbf3e", "#92c219", "#fabf21", "#f08a3a", "#fa4549"]

# Raw quintile cuts on the locked log-normal
RAW_QUINTILES = [math.exp(MU + SIGMA * norm.ppf(p)) for p in (0.2, 0.4, 0.6, 0.8)]
# Bucket medians (10/30/50/70/90th percentile)
BUCKET_MEDIANS = [math.exp(MU + SIGMA * norm.ppf(p)) for p in (0.1, 0.3, 0.5, 0.7, 0.9)]
# Doubling targets at those medians: 2^(-2 .. +2)
DOUBLING_TARGETS = [0.25, 0.5, 1.0, 2.0, 4.0]
MEDIAN = math.exp(MU)
FLOOR = 2 ** -2.5
CEIL = 2 ** 2.5

PERCENTILE_LABELS = ["p20", "p40", "p60", "p80"]
# Staircase the percentile lines so each label sits above the next line
PERCENTILE_STEPS = [1.0, 0.92, 0.84, 0.76]


# ----------------------------------------------------------------------
# Theme
# ----------------------------------------------------------------------
@dataclass
class Theme:
    name: str
    fg: str
    fg_muted: str
    grid: str
    accent: str
    band_alpha: float


LIGHT = Theme(name="light", fg="#1f2328", fg_muted="#59636e", grid="#d1d9e0",
              accent="#0969da", band_alpha=0.30)
DARK = Theme(name="dark", fg="#f0f6fc", fg_muted="#9198a1", grid="#30363d",
             accent="#58a6ff", band_alpha=0.36)


# ----------------------------------------------------------------------
# Math
# ----------------------------------------------------------------------
def pdf_loc(L):
    """Log-normal PDF under (MU, SIGMA)."""
    return np.where(
        L > 0,
        np.exp(-((np.log(np.maximum(L, 1e-9)) - MU) ** 2) / (2 * SIGMA ** 2))
        / (np.maximum(L, 1e-9) * SIGMA * math.sqrt(2 * math.pi)),
        0,
    )


def sf_from_F(F):
    """Size factor 2^(5F - 2.5) for percentile fraction F in [0, 1]."""
    return np.power(2.0, SIZE_RANGE * F - SIZE_RANGE / 2)


def loc_from_F(F):
    """Inverse log-normal CDF: percentile fraction back to line count."""
    F = np.clip(F, 1e-9, 1 - 1e-9)
    return np.exp(MU + SIGMA * norm.ppf(F))


# ----------------------------------------------------------------------
# Matplotlib helpers
# ----------------------------------------------------------------------
def apply_theme(theme: Theme):
    plt.rcParams.update({
        # transparent everywhere; the README composites onto its own bg
        "figure.facecolor": "none",
        "axes.facecolor": "none",
        "savefig.facecolor": "none",
        "savefig.edgecolor": "none",
        "savefig.transparent": True,
        # text and spines (only the bits that need theme contrast)
        "axes.edgecolor": theme.fg_muted,
        "axes.labelcolor": theme.fg,
        "axes.labelsize": 10.5,
        "axes.spines.top": False,
        "axes.spines.right": False,
        "axes.grid": False,
        "axes.axisbelow": True,
        "axes.linewidth": 0.8,
        "xtick.color": theme.fg_muted,
        "ytick.color": theme.fg_muted,
        "xtick.labelsize": 9.5,
        "ytick.labelsize": 10,
        "xtick.major.size": 0,
        "ytick.major.size": 0,
        "font.family": ["DejaVu Sans"],
        "font.size": 11,
        "mathtext.fontset": "dejavusans",
        "figure.dpi": 100,
        "savefig.dpi": 200,
        "savefig.bbox": "tight",
        "savefig.pad_inches": 0.15,
    })


def loc_formatter(x, _pos):
    if x >= 10_000:
        return f"{int(x):,}"
    if x >= 1:
        return f"{int(x)}"
    return f"{x:g}"


EXPONENT_LABELS = {0.25: r"$2^{-2}$", 0.5: r"$2^{-1}$", 1.0: r"$2^{0}$",
                   2.0: r"$2^{1}$", 4.0: r"$2^{2}$"}


def drop_left_spine(ax):
    """Hide the y-axis entirely (used by distribution graphs)."""
    ax.spines["left"].set_visible(False)
    ax.set_yticks([])
    ax.yaxis.set_visible(False)


# ----------------------------------------------------------------------
# Graph: distribution shape (with median marker)
# ----------------------------------------------------------------------
def render_distribution(theme: Theme, outpath: Path, *, log: bool):
    apply_theme(theme)
    fig, ax = plt.subplots(figsize=(10, 4.4))
    if log:
        L = np.logspace(0, 4, 2000)
    else:
        L = np.linspace(0, 600, 2000)
    y = pdf_loc(L)
    ax.fill_between(L, y, color=theme.accent, alpha=0.22, lw=0)
    ax.plot(L, y, color=theme.accent, lw=2.2)
    if log:
        ax.set_xscale("log")
        ax.set_xlim(1, 10_000)
        ax.set_xlabel("Lines changed (log scale)")
    else:
        ax.set_xlim(0, 600)
        ax.set_xlabel("Lines changed (linear)")
    ax.set_ylim(0, None)
    ax.margins(y=0)
    ax.set_ylabel("Pull request density")
    ax.xaxis.set_major_formatter(FuncFormatter(loc_formatter))
    drop_left_spine(ax)

    # Median anchor line
    ax.axvline(MEDIAN, color=theme.fg_muted, lw=1.0, ls=":", alpha=0.9, zorder=3)
    ax.annotate(
        f"  median = {MEDIAN:.0f}",
        xy=(MEDIAN, 1.0), xycoords=("data", "axes fraction"),
        xytext=(4, -6), textcoords="offset points",
        color=theme.fg, fontsize=10, va="top", ha="left",
        fontweight="medium",
    )

    fig.savefig(outpath, transparent=True)
    plt.close(fig)


# ----------------------------------------------------------------------
# Graph: distribution sliced into shirt-size quintiles
# ----------------------------------------------------------------------
def render_quintiles(theme: Theme, outpath: Path, *, log: bool):
    apply_theme(theme)
    fig, ax = plt.subplots(figsize=(10, 4.4))

    if log:
        L = np.logspace(0, 4, 4000)
        edges = [1.0] + RAW_QUINTILES + [10_000.0]
    else:
        L = np.linspace(0, 600, 4000)
        edges = [0.0] + RAW_QUINTILES + [600.0]
    y = pdf_loc(L)

    for i in range(5):
        lo, hi = edges[i], edges[i + 1]
        mask = (L >= lo) & (L <= hi)
        ax.fill_between(L[mask], y[mask], color=BUCKET_COLORS[i],
                         alpha=theme.band_alpha, lw=0)

    ax.plot(L, y, color=theme.accent, lw=2.2, zorder=4)

    if log:
        ax.set_xscale("log")
        ax.set_xlim(1, 10_000)
        ax.set_xlabel("Lines changed (log scale)")
    else:
        ax.set_xlim(0, 600)
        ax.set_xlabel("Lines changed (linear)")
    ax.set_ylim(0, None)
    ax.margins(y=0)
    ax.xaxis.set_major_formatter(FuncFormatter(loc_formatter))
    drop_left_spine(ax)

    for label, bound, ymax in zip(PERCENTILE_LABELS, RAW_QUINTILES, PERCENTILE_STEPS):
        ax.axvline(bound, ymin=0, ymax=ymax, color=theme.fg_muted,
                   lw=1.0, ls=":", alpha=0.9, zorder=3)
        ax.annotate(
            f" {label} ≈ {bound:.1f}",
            xy=(bound, ymax), xycoords=("data", "axes fraction"),
            xytext=(1, -3), textcoords="offset points",
            color=theme.fg_muted, fontsize=8, va="top", ha="left",
        )

    # Shirt-size badges: log graph runs straight across, linear graph
    # stair-steps XS/S/M to avoid pile-up.
    if log:
        label_ys = [0.0022] * 5
        centers = [math.sqrt(edges[i] * edges[i + 1]) for i in range(5)]
    else:
        label_ys = [0.0072, 0.0047, 0.0022, 0.0022, 0.0022]
        centers = [(edges[i] + edges[i + 1]) / 2 for i in range(5)]

    for bucket, color, center, ly in zip(BUCKETS, BUCKET_COLORS, centers, label_ys):
        ax.annotate(
            f" {bucket} ",
            xy=(center, ly), xycoords="data",
            ha="center", va="center",
            color=theme.fg, fontsize=11, fontweight="bold",
            zorder=6,
            bbox=dict(boxstyle="round,pad=0.35", facecolor=color,
                      edgecolor="none", alpha=0.95),
        )

    fig.savefig(outpath, transparent=True)
    plt.close(fig)


# ----------------------------------------------------------------------
# Derivation frames (size factor build-up)
# ----------------------------------------------------------------------
F_CURVE = np.linspace(0.0, 1.0, 1000)
SF_CURVE = sf_from_F(F_CURVE)
LOC_CURVE = loc_from_F(F_CURVE)

Y_LIM_LINEAR = (0, 6)
Y_TICKS_LINEAR = [0, 1, 2, 3, 4, 5, 6]
BUCKET_MEDIAN_PCT = [10, 30, 50, 70, 90]
QUINTILE_PCT = [20, 40, 60, 80]


def _derivation_dots(ax, theme, xs, ys, *, clip=True):
    for x, y, color, target in zip(xs, ys, BUCKET_COLORS, DOUBLING_TARGETS):
        ax.scatter([x], [y], s=140, color=color,
                   edgecolor=theme.fg, linewidth=1.4, zorder=6,
                   clip_on=clip)
        ax.annotate(f"{target:g}", xy=(x, y), xycoords="data",
                    xytext=(11, 0), textcoords="offset points",
                    color=theme.fg, fontsize=10, fontweight="medium",
                    va="center", ha="left", zorder=7,
                    annotation_clip=clip)


def _quintile_bands(ax, theme, edges, bounds):
    for i in range(5):
        ax.axvspan(edges[i], edges[i + 1], color=BUCKET_COLORS[i],
                   alpha=theme.band_alpha, lw=0, zorder=1)
    for bound in bounds:
        ax.axvline(bound, color=theme.fg_muted, lw=1.0, ls=":",
                   alpha=0.9, zorder=3)


def render_derivation_log(theme: Theme, outpath: Path, *, with_line: bool):
    apply_theme(theme)
    fig, ax = plt.subplots(figsize=(10, 4.8))
    edges = [1.0] + RAW_QUINTILES + [10_000.0]
    _quintile_bands(ax, theme, edges, RAW_QUINTILES)
    ax.yaxis.grid(True, which="major", color=theme.grid, lw=0.6,
                  alpha=0.7, zorder=2)
    ax.set_xscale("log")
    ax.set_xlim(1, 10_000)
    ax.set_ylim(*Y_LIM_LINEAR)
    ax.set_xlabel("Lines changed (log scale)")
    ax.set_ylabel("Size factor (linear)")
    ax.xaxis.set_major_formatter(FuncFormatter(loc_formatter))
    ax.yaxis.set_major_locator(FixedLocator(Y_TICKS_LINEAR))
    if with_line:
        ax.plot(LOC_CURVE, SF_CURVE, color=theme.accent, lw=2.2, alpha=0.85, zorder=4)
    _derivation_dots(ax, theme, BUCKET_MEDIANS, DOUBLING_TARGETS)
    fig.savefig(outpath, transparent=True)
    plt.close(fig)


def render_derivation_linear(theme: Theme, outpath: Path, *, with_line: bool):
    apply_theme(theme)
    fig, ax = plt.subplots(figsize=(10, 4.8))
    edges = [0.0] + RAW_QUINTILES + [600.0]
    _quintile_bands(ax, theme, edges, RAW_QUINTILES)
    ax.yaxis.grid(True, which="major", color=theme.grid, lw=0.6,
                  alpha=0.7, zorder=2)
    ax.set_xlim(0, 600)
    ax.set_ylim(*Y_LIM_LINEAR)
    ax.set_xlabel("Lines changed (linear)")
    ax.set_ylabel("Size factor (linear)")
    ax.xaxis.set_major_formatter(FuncFormatter(loc_formatter))
    ax.yaxis.set_major_locator(FixedLocator(Y_TICKS_LINEAR))
    if with_line:
        ax.plot(LOC_CURVE, SF_CURVE, color=theme.accent, lw=2.2, alpha=0.85, zorder=4)
    _derivation_dots(ax, theme, BUCKET_MEDIANS, DOUBLING_TARGETS, clip=False)
    fig.savefig(outpath, transparent=True)
    plt.close(fig)


def render_derivation_pct(theme: Theme, outpath: Path, *, with_line: bool):
    apply_theme(theme)
    fig, ax = plt.subplots(figsize=(10, 4.8))
    edges = [0, 20, 40, 60, 80, 100]
    _quintile_bands(ax, theme, edges, QUINTILE_PCT)
    ax.yaxis.grid(True, which="major", color=theme.grid, lw=0.6,
                  alpha=0.7, zorder=2)
    ax.set_xlim(0, 100)
    ax.set_ylim(*Y_LIM_LINEAR)
    ax.set_xlabel("Lines changed (percentile)")
    ax.set_ylabel("Size factor (linear)")
    ax.xaxis.set_major_locator(FixedLocator([0, 20, 40, 60, 80, 100]))
    ax.xaxis.set_major_formatter(FuncFormatter(lambda v, _: f"p{int(v)}"))
    ax.yaxis.set_major_locator(FixedLocator(Y_TICKS_LINEAR))
    if with_line:
        ax.plot(100 * F_CURVE, SF_CURVE, color=theme.accent, lw=2.2,
                alpha=0.85, zorder=4)
    _derivation_dots(ax, theme, BUCKET_MEDIAN_PCT, DOUBLING_TARGETS)
    fig.savefig(outpath, transparent=True)
    plt.close(fig)


def render_derivation_log2(theme: Theme, outpath: Path, *, with_line: bool):
    apply_theme(theme)
    fig, ax = plt.subplots(figsize=(10, 4.8))
    edges = [0, 20, 40, 60, 80, 100]
    _quintile_bands(ax, theme, edges, QUINTILE_PCT)
    ax.yaxis.grid(True, which="major", color=theme.grid, lw=0.6,
                  alpha=0.7, zorder=2)
    ax.set_xlim(0, 100)
    ax.set_xlabel("Lines changed (percentile)")
    ax.set_ylabel("Size factor (log₂ scale)")
    ax.xaxis.set_major_locator(FixedLocator([0, 20, 40, 60, 80, 100]))
    ax.xaxis.set_major_formatter(FuncFormatter(lambda v, _: f"p{int(v)}"))
    ax.set_yscale("log", base=2)
    ax.set_ylim(FLOOR, CEIL)
    ax.yaxis.set_major_locator(FixedLocator(DOUBLING_TARGETS))
    ax.yaxis.set_major_formatter(FuncFormatter(lambda v, _: EXPONENT_LABELS[round(v, 3)]))
    ax.yaxis.set_minor_locator(FixedLocator([]))
    if with_line:
        # Corner-to-corner in axes coords (== the formula on log2 Y by construction)
        ax.plot([0, 1], [0, 1], transform=ax.transAxes,
                color=theme.accent, lw=2.2, alpha=0.85, zorder=4)
    _derivation_dots(ax, theme, BUCKET_MEDIAN_PCT, DOUBLING_TARGETS)
    fig.savefig(outpath, transparent=True)
    plt.close(fig)


# ----------------------------------------------------------------------
# Driver
# ----------------------------------------------------------------------
JOBS = [
    # Distribution shape
    ("distribution-linear", render_distribution, dict(log=False)),
    ("distribution-log",    render_distribution, dict(log=True)),
    # Quintile slicing
    ("quintiles-linear",    render_quintiles,    dict(log=False)),
    ("quintiles-log",       render_quintiles,    dict(log=True)),
    # Derivation, dots-only
    ("derivation-frame1",        render_derivation_log,    dict(with_line=False)),
    ("derivation-frame1-linear", render_derivation_linear, dict(with_line=False)),
    ("derivation-frame2",        render_derivation_pct,    dict(with_line=False)),
    ("derivation-frame3",        render_derivation_log2,   dict(with_line=False)),
    # Derivation, with formula line
    ("derivation-frame4",             render_derivation_log2,   dict(with_line=True)),
    ("derivation-frame2-line",        render_derivation_pct,    dict(with_line=True)),
    ("derivation-frame1-log-line",    render_derivation_log,    dict(with_line=True)),
    ("derivation-frame1-linear-line", render_derivation_linear, dict(with_line=True)),
]


def main():
    out = Path(__file__).resolve().parent.parent / "docs" / "img"
    out.mkdir(parents=True, exist_ok=True)
    written = 0
    for stem, fn, kwargs in JOBS:
        for theme in (LIGHT, DARK):
            path = out / f"{stem}-{theme.name}.svg"
            print(f"  rendering {path.relative_to(out.parent.parent)}")
            fn(theme, path, **kwargs)
            written += 1
    print(f"\nDone. {written} files written to {out}.")
    print(f"\nLocked constants:")
    print(f"  μ = {MU}, σ = {SIGMA}")
    print(f"  raw quintile cuts: {[f'{c:.3f}' for c in RAW_QUINTILES]}")
    print(f"  bucket medians:    {[f'{m:.2f}' for m in BUCKET_MEDIANS]}")
    print(f"  doubling targets:  {DOUBLING_TARGETS}")
    print(f"  median exp(μ):     {MEDIAN:.4f}")
    print(f"  floor 2^-2.5:      {FLOOR:.4f}")
    print(f"  ceil 2^+2.5:       {CEIL:.4f}")


if __name__ == "__main__":
    main()
