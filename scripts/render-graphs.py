#!/usr/bin/env python3
"""
Render the cru size-factor visualization suite.

Generates light + dark variants of each graph at high DPI for the
laserlemon/cru README. Each graph lives under docs/img/<name>-{light,dark}.png
and is wired into the README via <picture> elements.

Run:
    python3 scripts/render-graphs.py

Outputs:
    docs/img/size-factor-hero-{light,dark}.png
    docs/img/distribution-linear-vs-log-{light,dark}.png
    docs/img/distribution-with-buckets-{light,dark}.png
    docs/img/cdf-with-quintiles-{light,dark}.png
    docs/img/size-factor-derivation-{light,dark}.png
    docs/img/size-factor-log-log-{light,dark}.png
    docs/img/bucket-mass-{light,dark}.png
    docs/img/doubling-anchors-{light,dark}.png

The math is identical to cru.go. The constants are duplicated here for
plot-rendering convenience; if cru.go changes, update both.
"""

from __future__ import annotations

import math
from dataclasses import dataclass
from pathlib import Path

import matplotlib.pyplot as plt
import numpy as np
from matplotlib.ticker import FuncFormatter
from scipy.stats import norm

# ----------------------------------------------------------------------
# Locked constants (mirror cru.go)
# ----------------------------------------------------------------------
MU = 3.808551
SIGMA = 1.802600
SIZE_RANGE = 5.0  # len(sizes) — doubling range across F

BUCKETS = ["XS", "S", "M", "L", "XL"]

# Nearest-even quintile boundaries (derived from MU, SIGMA)
RAW_QUINTILES = [math.exp(MU + SIGMA * norm.ppf(p)) for p in (0.2, 0.4, 0.6, 0.8)]
FINAL_BOUNDS = [round(b / 2) * 2 for b in RAW_QUINTILES]  # [10, 28, 72, 206]

# Bucket median LOC (10th, 30th, 50th, 70th, 90th percentile)
BUCKET_MEDIANS = [math.exp(MU + SIGMA * norm.ppf(p)) for p in (0.1, 0.3, 0.5, 0.7, 0.9)]

# Doubling targets at bucket medians
DOUBLING_LEVELS = [2 ** (5 * p - 2.5) for p in (0.1, 0.3, 0.5, 0.7, 0.9)]
# => [0.25, 0.5, 1.0, 2.0, 4.0]

ANCHOR_LOC = math.exp(MU)  # 45.09 — geomean, where size factor = 1.0
FLOOR = 2 ** -2.5  # 0.1768
CEIL = 2 ** 2.5  # 5.6569


# ----------------------------------------------------------------------
# Theme palettes (GitHub Primer)
# ----------------------------------------------------------------------
@dataclass
class Theme:
    name: str
    bg: str
    fg: str
    fg_muted: str
    grid: str
    accent: str  # primary curve
    accent_dim: str  # secondary
    bucket_colors: list[str]
    bucket_fill_alpha: float


LIGHT = Theme(
    name="light",
    bg="#ffffff",
    fg="#1f2328",
    fg_muted="#59636e",
    grid="#d1d9e0",
    accent="#0969da",
    accent_dim="#218bff",
    bucket_colors=["#1a7f37", "#4f9e1c", "#9a6700", "#bc4c00", "#cf222e"],
    bucket_fill_alpha=0.18,
)

DARK = Theme(
    name="dark",
    bg="#0d1117",
    fg="#f0f6fc",
    fg_muted="#9198a1",
    grid="#30363d",
    accent="#58a6ff",
    accent_dim="#388bfd",
    bucket_colors=["#3fb950", "#82c91e", "#d29922", "#db6d28", "#f85149"],
    bucket_fill_alpha=0.22,
)


# ----------------------------------------------------------------------
# Math helpers
# ----------------------------------------------------------------------
def pdf_loc(L):
    """Log-normal PDF of LOC under (MU, SIGMA)."""
    return np.where(
        L > 0,
        np.exp(-((np.log(np.maximum(L, 1e-9)) - MU) ** 2) / (2 * SIGMA ** 2))
        / (np.maximum(L, 1e-9) * SIGMA * math.sqrt(2 * math.pi)),
        0,
    )


def cdf_loc(L):
    """CDF F(L) = Φ((ln L − μ) / σ)."""
    return norm.cdf((np.log(np.maximum(L, 1e-9)) - MU) / SIGMA)


def size_factor(L):
    """Size factor 2^(5F(L) − 2.5)."""
    f = cdf_loc(L)
    return np.power(2.0, SIZE_RANGE * f - SIZE_RANGE / 2)


def bucket_mass(lo, hi):
    """Mass of (lo, hi] under the log-normal."""
    lo_p = cdf_loc(np.array([lo]))[0] if lo > 0 else 0.0
    hi_p = cdf_loc(np.array([hi]))[0] if hi != math.inf else 1.0
    return float(hi_p - lo_p)


# ----------------------------------------------------------------------
# Matplotlib setup
# ----------------------------------------------------------------------
def apply_theme(theme: Theme):
    plt.rcParams.update(
        {
            "figure.facecolor": theme.bg,
            "axes.facecolor": theme.bg,
            "savefig.facecolor": theme.bg,
            "savefig.edgecolor": "none",
            "axes.edgecolor": theme.fg_muted,
            "axes.labelcolor": theme.fg,
            "axes.titlecolor": theme.fg,
            "axes.titlesize": 13,
            "axes.titleweight": "semibold",
            "axes.titlepad": 14,
            "axes.labelsize": 10.5,
            "axes.labelweight": "normal",
            "axes.spines.top": False,
            "axes.spines.right": False,
            "axes.grid": True,
            "axes.axisbelow": True,
            "axes.linewidth": 0.8,
            "grid.color": theme.grid,
            "grid.linewidth": 0.6,
            "grid.alpha": 0.7,
            "xtick.color": theme.fg_muted,
            "ytick.color": theme.fg_muted,
            "xtick.labelsize": 9.5,
            "ytick.labelsize": 9.5,
            "xtick.major.size": 0,
            "ytick.major.size": 0,
            "legend.frameon": False,
            "legend.fontsize": 10,
            "legend.labelcolor": theme.fg,
            "font.family": ["-apple-system", "BlinkMacSystemFont", "Segoe UI", "Helvetica", "Arial", "sans-serif"],
            "font.size": 11,
            "figure.dpi": 100,
            "savefig.dpi": 200,
            "savefig.bbox": "tight",
            "savefig.pad_inches": 0.3,
        }
    )


def loc_formatter(x, _pos):
    """Format LOC tick labels with thousands separators."""
    if x >= 10_000:
        return f"{int(x):,}"
    if x >= 1:
        return f"{int(x)}"
    return f"{x:g}"


def draw_top_bucket_badges(ax, theme: Theme, y_position: float):
    """Draw colored bucket name badges along the top edge of an x-log plot."""
    bounds = [0.5] + FINAL_BOUNDS + [10_000]
    for i, (color, label) in enumerate(zip(theme.bucket_colors, BUCKETS)):
        center = math.sqrt(bounds[i] * bounds[i + 1])
        ax.annotate(
            f" {label} ",
            xy=(center, y_position),
            xycoords=("data", "axes fraction"),
            ha="center", va="center",
            color=theme.bg, fontsize=10.5, fontweight="bold",
            bbox=dict(boxstyle="round,pad=0.35", facecolor=color, edgecolor="none"),
        )


# ----------------------------------------------------------------------
# Individual graph renderers
# ----------------------------------------------------------------------
def graph_size_factor_hero(theme: Theme, outpath: Path):
    """The headline image: size factor curve with bucket regions and doubling levels."""
    apply_theme(theme)
    fig, ax = plt.subplots(figsize=(11, 5.6))

    L = np.logspace(0, 4, 1000)
    sf = size_factor(L)

    # Shade bucket regions
    bounds = [0.5] + FINAL_BOUNDS + [10_000]
    for i, color in enumerate(theme.bucket_colors):
        ax.axvspan(bounds[i], bounds[i + 1], color=color, alpha=theme.bucket_fill_alpha, lw=0)

    # The curve
    ax.plot(L, sf, color=theme.accent, lw=2.6, zorder=5)

    # Floor / ceiling reference lines
    ax.axhline(FLOOR, color=theme.fg_muted, lw=0.8, ls=":", alpha=0.5, zorder=2)
    ax.axhline(CEIL, color=theme.fg_muted, lw=0.8, ls=":", alpha=0.5, zorder=2)
    ax.text(11000, FLOOR, f"  floor 2⁻²·⁵ ≈ {FLOOR:.3f}", color=theme.fg_muted, fontsize=9, va="center", ha="left")
    ax.text(11000, CEIL, f"  ceiling 2²·⁵ ≈ {CEIL:.3f}", color=theme.fg_muted, fontsize=9, va="center", ha="left")

    # Doubling horizontal lines + labels
    for level in DOUBLING_LEVELS:
        ax.axhline(level, color=theme.fg_muted, lw=0.5, ls="--", alpha=0.4, zorder=2)

    # Bucket median anchors
    for med, level, color, label in zip(BUCKET_MEDIANS, DOUBLING_LEVELS, theme.bucket_colors, BUCKETS):
        ax.scatter([med], [level], s=70, color=color, edgecolor=theme.bg, lw=2, zorder=10)
        ax.annotate(
            f"{level}×",
            xy=(med, level),
            xytext=(0, 14),
            textcoords="offset points",
            ha="center",
            color=color,
            fontsize=10.5,
            fontweight="bold",
        )

    # Anchor point (typical PR = 1.0) — extra ring marker
    ax.scatter([ANCHOR_LOC], [1.0], s=140, marker="o", facecolor="none",
               edgecolor=theme.accent, lw=2, zorder=11)

    ax.set_xscale("log")
    ax.set_xlim(0.5, 10_000)
    ax.set_ylim(0, 6.8)
    ax.set_xlabel("Pull request size (lines changed)")
    ax.set_ylabel("Size factor")
    ax.set_title("Size factor across the locked log-normal", color=theme.fg, loc="left", pad=22, fontsize=14)
    ax.xaxis.set_major_formatter(FuncFormatter(loc_formatter))
    ax.set_yticks([0, 0.25, 0.5, 1.0, 2.0, 4.0, 5.66])
    ax.set_yticklabels(["0", "0.25", "0.5", "1", "2", "4", "5.66"])

    # Top bucket badges (drawn at y = 1.02 in axes fraction)
    draw_top_bucket_badges(ax, theme, 1.04)

    # Footer
    fig.text(
        0.5, -0.02,
        f"μ = {MU:.6f}    σ = {SIGMA:.6f}    anchor at LOC = exp(μ) ≈ {ANCHOR_LOC:.1f} → size factor 1.0",
        ha="center", color=theme.fg_muted, fontsize=9,
    )

    fig.savefig(outpath, facecolor=theme.bg)
    plt.close(fig)


def graph_distribution_linear_vs_log(theme: Theme, outpath: Path):
    """Side-by-side PDF: linear X (the cliff) vs log X (the bell)."""
    apply_theme(theme)
    fig, (ax_lin, ax_log) = plt.subplots(1, 2, figsize=(12, 4.8), gridspec_kw={"wspace": 0.25})

    # Linear view
    L_lin = np.linspace(0.5, 600, 1500)
    ax_lin.plot(L_lin, pdf_loc(L_lin), color=theme.accent, lw=2.2)
    ax_lin.fill_between(L_lin, pdf_loc(L_lin), color=theme.accent, alpha=0.18)
    ax_lin.set_xlim(0, 600)
    ax_lin.set_xlabel("Pull request size (lines changed)")
    ax_lin.set_ylabel("Probability density")
    ax_lin.set_title("Linear scale: the cliff", color=theme.fg, loc="left", pad=10)
    ax_lin.xaxis.set_major_formatter(FuncFormatter(loc_formatter))

    # Log view
    L_log = np.logspace(0, 4, 1500)
    ax_log.plot(L_log, pdf_loc(L_log), color=theme.accent, lw=2.2)
    ax_log.fill_between(L_log, pdf_loc(L_log), color=theme.accent, alpha=0.18)
    ax_log.set_xscale("log")
    ax_log.set_xlim(1, 10_000)
    ax_log.set_xlabel("Pull request size (lines changed, log scale)")
    ax_log.set_ylabel("Probability density")
    ax_log.set_title("Log scale: the bell", color=theme.fg, loc="left", pad=10)
    ax_log.xaxis.set_major_formatter(FuncFormatter(loc_formatter))

    # Annotate the geomean on the log view
    ax_log.axvline(ANCHOR_LOC, color=theme.fg_muted, lw=0.8, ls="--", alpha=0.5)
    ax_log.annotate(
        f"  exp(μ) ≈ {ANCHOR_LOC:.1f}",
        xy=(ANCHOR_LOC, pdf_loc(np.array([ANCHOR_LOC]))[0]),
        xytext=(12, -8), textcoords="offset points",
        color=theme.fg_muted, fontsize=9,
    )

    fig.suptitle(
        "Same distribution, two scales: log normal looks normal once you log the x axis",
        color=theme.fg, fontsize=13.5, y=1.06, fontweight="semibold",
    )

    fig.savefig(outpath, facecolor=theme.bg)
    plt.close(fig)


def graph_distribution_with_buckets(theme: Theme, outpath: Path):
    """Log-X PDF with five colored bucket regions and mass labels.

    Mass labels are drawn near the top of the axes, not inside the curve, so
    the XL region (where the curve is essentially zero) gets a legible label.
    """
    apply_theme(theme)
    fig, ax = plt.subplots(figsize=(11, 5.3))

    L = np.logspace(0, 4, 2000)
    y = pdf_loc(L)
    ax.plot(L, y, color=theme.fg, lw=1.5, zorder=10, alpha=0.85)

    bounds = [0.5] + FINAL_BOUNDS + [10_000]
    y_max = float(np.max(y))
    for i, color in enumerate(theme.bucket_colors):
        lo, hi = bounds[i], bounds[i + 1]
        mask = (L >= lo) & (L <= hi)
        ax.fill_between(L[mask], y[mask], color=color, alpha=0.55, lw=0, zorder=5)

        # Mass for the actual (final) bucket
        actual_lo = 0 if i == 0 else FINAL_BOUNDS[i - 1]
        actual_hi = FINAL_BOUNDS[i] if i < 4 else math.inf
        mass = bucket_mass(actual_lo, actual_hi)
        bound_str = (f"(0, {FINAL_BOUNDS[0]}]" if i == 0
                     else f"({FINAL_BOUNDS[i-1]}, {FINAL_BOUNDS[i]}]" if i < 4
                     else f"({FINAL_BOUNDS[3]}, ∞)")

        # Label at top of plot, not on curve. Use axes-fraction y-coord.
        center = math.sqrt(max(lo, 0.8) * hi) if hi < 1e5 else 1200
        ax.annotate(
            f"{BUCKETS[i]}\n{bound_str}\n{mass*100:.1f}%",
            xy=(center, 0.78),
            xycoords=("data", "axes fraction"),
            ha="center", va="top",
            color=color, fontsize=10.5, fontweight="bold",
        )

    # Boundary verticals
    for b in FINAL_BOUNDS:
        ax.axvline(b, color=theme.fg_muted, lw=0.8, ls=":", alpha=0.6, zorder=3)

    ax.set_xscale("log")
    ax.set_xlim(1, 10_000)
    ax.set_ylim(0, y_max * 1.18)
    ax.set_xlabel("Pull request size (lines changed, log scale)")
    ax.set_ylabel("Probability density")
    ax.set_title("Five buckets, each ≈ 20% of the distribution", color=theme.fg, loc="left")
    ax.xaxis.set_major_formatter(FuncFormatter(loc_formatter))

    fig.savefig(outpath, facecolor=theme.bg)
    plt.close(fig)


def graph_cdf_with_quintiles(theme: Theme, outpath: Path):
    """CDF F(L) with quintile dashed cross-hairs showing the boundary derivation."""
    apply_theme(theme)
    fig, ax = plt.subplots(figsize=(11, 5.6))

    L = np.logspace(0, 4, 1500)
    F = cdf_loc(L)
    ax.plot(L, F, color=theme.accent, lw=2.4, zorder=8)

    # Quintile cross-hairs
    for q, raw, color in zip([0.2, 0.4, 0.6, 0.8], RAW_QUINTILES, theme.bucket_colors[:-1]):
        # Horizontal line at percentile
        ax.plot([1, raw], [q, q], color=color, lw=1.0, ls="--", alpha=0.75, zorder=3)
        # Vertical drop
        ax.plot([raw, raw], [0, q], color=color, lw=1.0, ls="--", alpha=0.75, zorder=3)
        # X-axis annotation: raw + rounded
        rounded = round(raw / 2) * 2
        ax.annotate(
            f"{raw:.2f}\n→ {rounded}",
            xy=(raw, 0),
            xytext=(0, -30), textcoords="offset points",
            ha="center", va="top",
            color=color, fontsize=9.5, fontweight="semibold",
        )

    # Anchor: median maps to 50%
    ax.scatter([ANCHOR_LOC], [0.5], s=80, color=theme.accent, edgecolor=theme.bg, lw=2, zorder=12)
    ax.annotate(
        f"median: F({ANCHOR_LOC:.1f}) = 0.5",
        xy=(ANCHOR_LOC, 0.5),
        xytext=(12, -22), textcoords="offset points",
        color=theme.accent, fontsize=10, fontweight="semibold",
    )

    ax.set_xscale("log")
    ax.set_xlim(1, 10_000)
    ax.set_ylim(0, 1.05)
    ax.set_xlabel("Pull request size (lines changed, log scale)")
    ax.set_ylabel("F(L) = percentile rank")
    ax.set_title("CDF: where the bucket boundaries come from", color=theme.fg, loc="left")
    ax.xaxis.set_major_formatter(FuncFormatter(loc_formatter))

    # Single set of y ticks, formatted as percentiles (no double-labeling)
    ax.set_yticks([0, 0.2, 0.4, 0.6, 0.8, 1.0])
    ax.yaxis.set_major_formatter(FuncFormatter(lambda v, _: f"{int(v*100)}%"))

    # Highlight the four percentile rows by recoloring those specific labels
    yticklabels = ax.get_yticklabels()
    # Indices 1..4 of [0, 0.2, 0.4, 0.6, 0.8, 1.0] correspond to quintile rows
    for idx, color in zip([1, 2, 3, 4], theme.bucket_colors[:-1]):
        yticklabels[idx].set_color(color)
        yticklabels[idx].set_fontweight("semibold")

    fig.savefig(outpath, facecolor=theme.bg)
    plt.close(fig)


def graph_size_factor_derivation(theme: Theme, outpath: Path):
    """Three-panel: F(L) → 5F-2.5 → 2^(5F-2.5). The transformation chain."""
    apply_theme(theme)
    fig, axes = plt.subplots(1, 3, figsize=(13.5, 4.6), gridspec_kw={"wspace": 0.32})

    L = np.logspace(0, 4, 1200)
    F = cdf_loc(L)
    exponent = SIZE_RANGE * F - SIZE_RANGE / 2  # 5F − 2.5
    sf = np.power(2.0, exponent)

    # Panel 1: F(L)
    ax = axes[0]
    ax.plot(L, F, color=theme.accent, lw=2.2)
    ax.set_xscale("log")
    ax.set_xlim(1, 10_000)
    ax.set_ylim(0, 1)
    ax.set_xlabel("Lines changed (log)")
    ax.set_ylabel("F(L)")
    ax.set_title("1. Percentile rank\nF(L) = Φ((ln L − μ) / σ)", color=theme.fg, loc="left", fontsize=11.5, pad=10)
    ax.xaxis.set_major_formatter(FuncFormatter(loc_formatter))
    ax.set_yticks([0, 0.5, 1.0])

    # Panel 2: 5F − 2.5
    ax = axes[1]
    ax.plot(L, exponent, color=theme.accent, lw=2.2)
    ax.axhline(0, color=theme.fg_muted, lw=0.6, alpha=0.5)
    ax.set_xscale("log")
    ax.set_xlim(1, 10_000)
    ax.set_ylim(-2.7, 2.7)
    ax.set_xlabel("Lines changed (log)")
    ax.set_ylabel("5·F(L) − 2.5")
    ax.set_title("2. Rescale to ±2.5\n(so anchor = 0, range = 5)", color=theme.fg, loc="left", fontsize=11.5, pad=10)
    ax.xaxis.set_major_formatter(FuncFormatter(loc_formatter))
    ax.set_yticks([-2.5, -1.25, 0, 1.25, 2.5])

    # Panel 3: 2^(5F − 2.5)
    ax = axes[2]
    ax.plot(L, sf, color=theme.accent, lw=2.2)
    ax.set_xscale("log")
    ax.set_xlim(1, 10_000)
    ax.set_ylim(0, 6.2)
    ax.set_xlabel("Lines changed (log)")
    ax.set_ylabel("Size factor")
    ax.set_title("3. Exponentiate base 2\n→ doublings every 20 percentile points", color=theme.fg, loc="left", fontsize=11.5, pad=10)
    ax.xaxis.set_major_formatter(FuncFormatter(loc_formatter))
    ax.set_yticks([0, 1, 2, 4, 5.66])

    fig.suptitle(
        "From percentile rank to size factor in three steps",
        color=theme.fg, fontsize=13.5, y=1.04, fontweight="semibold",
    )

    fig.savefig(outpath, facecolor=theme.bg)
    plt.close(fig)


def graph_size_factor_log_log(theme: Theme, outpath: Path):
    """Size factor on log-LOC × log-size. Doublings as equal vertical steps."""
    apply_theme(theme)
    fig, ax = plt.subplots(figsize=(11, 5.4))

    L = np.logspace(0, 4, 1500)
    sf = size_factor(L)

    # Bucket regions
    bounds = [0.5] + FINAL_BOUNDS + [10_000]
    for i, color in enumerate(theme.bucket_colors):
        ax.axvspan(bounds[i], bounds[i + 1], color=color, alpha=theme.bucket_fill_alpha, lw=0)

    ax.plot(L, sf, color=theme.accent, lw=2.6, zorder=5)

    # Floor + ceiling
    ax.axhline(FLOOR, color=theme.fg_muted, lw=0.7, ls=":", alpha=0.5)
    ax.axhline(CEIL, color=theme.fg_muted, lw=0.7, ls=":", alpha=0.5)

    # Doubling markers — label BELOW the curve consistently so they don't overlap the line
    for med, level, color, label in zip(BUCKET_MEDIANS, DOUBLING_LEVELS, theme.bucket_colors, BUCKETS):
        ax.scatter([med], [level], s=80, color=color, edgecolor=theme.bg, lw=2, zorder=10)
        # Offset annotation diagonally down-right so it stays clear of the curve
        ax.annotate(
            f"{label}: {level}×",
            xy=(med, level),
            xytext=(14, -16), textcoords="offset points",
            ha="left", va="top",
            color=color, fontsize=10, fontweight="bold",
        )

    ax.set_xscale("log")
    ax.set_yscale("log", base=2)
    ax.set_xlim(0.5, 10_000)
    ax.set_ylim(0.15, 7)
    ax.set_xlabel("Pull request size (lines changed, log scale)")
    ax.set_ylabel("Size factor (log scale, base 2)")
    ax.set_title("Log-log view: each bucket median is a doubling", color=theme.fg, loc="left")
    ax.xaxis.set_major_formatter(FuncFormatter(loc_formatter))
    ax.set_yticks([0.25, 0.5, 1, 2, 4])
    ax.yaxis.set_major_formatter(FuncFormatter(lambda v, _: f"{v}"))
    ax.minorticks_off()

    fig.savefig(outpath, facecolor=theme.bg)
    plt.close(fig)


def graph_bucket_mass(theme: Theme, outpath: Path):
    """Bar chart of theoretical bucket mass, zoomed to highlight ±1pp deviations.

    Y range zoomed to 18-22% so the deviations from perfect 20% are visible.
    Reference annotation moved to the left side to avoid colliding with value labels.
    """
    apply_theme(theme)
    fig, ax = plt.subplots(figsize=(9.5, 4.8))

    masses = []
    labels = []
    bounds_full = [0] + FINAL_BOUNDS + [math.inf]
    for i in range(5):
        m = bucket_mass(bounds_full[i], bounds_full[i + 1])
        masses.append(m * 100)
        bound_str = (f"(0, {FINAL_BOUNDS[0]}]" if i == 0
                     else f"({FINAL_BOUNDS[i-1]}, {FINAL_BOUNDS[i]}]" if i < 4
                     else f"({FINAL_BOUNDS[3]}, ∞)")
        labels.append(f"{BUCKETS[i]}\n{bound_str}")

    x = np.arange(5)
    bars = ax.bar(x, masses, color=theme.bucket_colors, width=0.65,
                  edgecolor=theme.bg, lw=1.5, zorder=5)

    # 20% reference line
    ax.axhline(20, color=theme.fg_muted, lw=1.2, ls="--", alpha=0.85, zorder=3)

    # Annotation pinned to top-left in axes fraction, well clear of any bar
    ax.text(
        0.01, 0.95,
        "dashed line: perfect equal-fifths (20%)",
        transform=ax.transAxes,
        ha="left", va="top",
        color=theme.fg_muted, fontsize=9.5, fontweight="normal",
    )

    # Value labels above bars
    for bar, m in zip(bars, masses):
        ax.annotate(
            f"{m:.2f}%",
            xy=(bar.get_x() + bar.get_width() / 2, bar.get_height()),
            xytext=(0, 5), textcoords="offset points",
            ha="center", va="bottom",
            color=theme.fg, fontsize=10.5, fontweight="semibold",
        )

    ax.set_xticks(x)
    ax.set_xticklabels(labels, color=theme.fg, fontsize=10)
    ax.set_ylabel("Probability mass")
    ax.set_ylim(18, 22)
    ax.yaxis.set_major_formatter(FuncFormatter(lambda v, _: f"{v:.0f}%"))
    ax.set_yticks([18, 19, 20, 21, 22])
    ax.set_title("Theoretical mass per bucket (every bucket within 1pp of 20%)",
                 color=theme.fg, loc="left")
    ax.grid(axis="x", visible=False)

    fig.savefig(outpath, facecolor=theme.bg)
    plt.close(fig)


def graph_doubling_anchors(theme: Theme, outpath: Path):
    """The five doubling anchors on the size factor curve.

    Annotations alternate above/below the curve to avoid collisions between
    adjacent bucket boxes. Each callout uses a leader to its anchor point.
    """
    apply_theme(theme)
    fig, ax = plt.subplots(figsize=(11.5, 6.4))

    L = np.logspace(0, 4, 1500)
    sf = size_factor(L)

    ax.plot(L, sf, color=theme.accent, lw=2.4, zorder=4)

    # Doubling reference lines
    for level in DOUBLING_LEVELS:
        ax.axhline(level, color=theme.fg_muted, lw=0.6, ls="--", alpha=0.45, zorder=2)

    # Annotation positions: alternate above (high y) and below (low y) so adjacent
    # boxes don't collide. Use axes-fraction y coords for stable placement.
    annotation_positions = [
        # (axes_y_fraction, va, vertical_arrow_direction)
        (0.85, "top",    -1),  # XS — top of plot
        (0.20, "bottom", +1),  # S  — bottom
        (0.85, "top",    -1),  # M  — top
        (0.20, "bottom", +1),  # L  — bottom
        (0.85, "top",    -1),  # XL — top
    ]

    for (med, level, color, label, (frac_y, va, _dir)) in zip(
        BUCKET_MEDIANS, DOUBLING_LEVELS, theme.bucket_colors, BUCKETS, annotation_positions
    ):
        # Anchor dot
        ax.scatter([med], [level], s=110, color=color, edgecolor=theme.bg, lw=2.2, zorder=10)

        # Callout box at the chosen vertical position
        ax.annotate(
            f"{label} bucket median\nLOC ≈ {med:.1f}\nsize factor = {level}",
            xy=(med, level),
            xytext=(med, frac_y),
            textcoords=("data", "axes fraction"),
            ha="center", va=va,
            color=color, fontsize=9.5, fontweight="semibold",
            bbox=dict(boxstyle="round,pad=0.4", facecolor=theme.bg,
                      edgecolor=color, lw=1.2, alpha=0.95),
            arrowprops=dict(
                arrowstyle="-",
                color=color, lw=0.8, alpha=0.6,
                shrinkA=4, shrinkB=8,
            ),
            zorder=12,
        )

    ax.set_xscale("log")
    ax.set_xlim(1, 10_000)
    ax.set_ylim(-0.1, 6.5)
    ax.set_xlabel("Pull request size (lines changed, log scale)")
    ax.set_ylabel("Size factor")
    ax.set_title("The doubling property: 5 bucket medians, 5 powers of 2",
                 color=theme.fg, loc="left")
    ax.xaxis.set_major_formatter(FuncFormatter(loc_formatter))
    ax.set_yticks([0, 0.25, 0.5, 1, 2, 4])

    fig.savefig(outpath, facecolor=theme.bg)
    plt.close(fig)


# ----------------------------------------------------------------------
# Driver
# ----------------------------------------------------------------------
GRAPHS = [
    ("size-factor-hero", graph_size_factor_hero),
    ("distribution-linear-vs-log", graph_distribution_linear_vs_log),
    ("distribution-with-buckets", graph_distribution_with_buckets),
    ("cdf-with-quintiles", graph_cdf_with_quintiles),
    ("size-factor-derivation", graph_size_factor_derivation),
    ("size-factor-log-log", graph_size_factor_log_log),
    ("bucket-mass", graph_bucket_mass),
    ("doubling-anchors", graph_doubling_anchors),
]


def main():
    outdir = Path(__file__).resolve().parent.parent / "docs" / "img"
    outdir.mkdir(parents=True, exist_ok=True)

    for name, renderer in GRAPHS:
        for theme in (LIGHT, DARK):
            path = outdir / f"{name}-{theme.name}.png"
            print(f"  rendering {path.relative_to(outdir.parent.parent)}")
            renderer(theme, path)
            assert path.stat().st_size > 1000, f"{path} too small"

    print()
    print(f"Done. {len(GRAPHS) * 2} files written to {outdir}.")
    print()
    print("Sanity check the math:")
    print(f"  μ = {MU}, σ = {SIGMA}")
    print(f"  raw quintile cuts: {[f'{b:.4f}' for b in RAW_QUINTILES]}")
    print(f"  final bounds (nearest-even): {FINAL_BOUNDS}")
    print(f"  bucket medians: {[f'{m:.2f}' for m in BUCKET_MEDIANS]}")
    print(f"  doubling levels at those medians: {DOUBLING_LEVELS}")
    print(f"  anchor LOC (= exp(μ)): {ANCHOR_LOC:.4f}")


if __name__ == "__main__":
    main()
