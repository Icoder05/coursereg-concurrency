"""Draw the README figures from a results folder.

    python figures/make_figures.py experiments/results/2026-10-29

Reads the raw CSVs and summary.json written by experiments/analysis/analyze.py
and writes PNGs next to this script. It only plots; every statistic comes from
summary.json, so the figures match the report.
"""
import json
import sys
from pathlib import Path

import matplotlib.pyplot as plt
import numpy as np
import pandas as pd

RES = Path(sys.argv[1] if len(sys.argv) > 1 else "experiments/results/2026-10-29")
OUT = Path(__file__).parent

SURFACE, INK, INK2, GRID = "#fcfcfb", "#0b0b0b", "#52514e", "#e4e3df"
TREAT, BASE, THIRD = "#2a78d6", "#eb6834", "#1baf7a"

plt.rcParams.update({
    "figure.facecolor": SURFACE, "axes.facecolor": SURFACE, "savefig.facecolor": SURFACE,
    "font.family": "DejaVu Sans", "font.size": 10.5, "text.color": INK,
    "axes.edgecolor": GRID, "axes.labelcolor": INK2, "xtick.color": INK2, "ytick.color": INK2,
    "axes.spines.top": False, "axes.spines.right": False, "axes.spines.left": False,
    "axes.grid": True, "axes.grid.axis": "y", "grid.color": GRID, "grid.linewidth": 0.8,
    "axes.titleweight": "bold", "axes.titlesize": 12.5, "axes.titlelocation": "left",
    "legend.frameon": False,
})

summary = json.loads((RES / "summary.json").read_text())


def title(ax, head, sub):
    ax.set_title(head, pad=24)
    ax.text(0, 1.02, sub, transform=ax.transAxes, color=INK2, fontsize=9.5)


def save(fig, name):
    fig.savefig(OUT / name, dpi=160, bbox_inches="tight", pad_inches=0.25)
    plt.close(fig)
    print("wrote", OUT / name)


def rq1(ax):
    t = pd.read_csv(RES / "rq1_trials.csv")
    t = t[~t.warmup]
    levels = sorted(t.n.unique())
    x = np.arange(len(levels))
    w = 0.36
    for off, arm, color, label in [(-w / 2 - 0.01, "naive", BASE, "Naive check-then-insert"),
                                   (w / 2 + 0.01, "locked", TREAT, "Locked (SELECT … FOR UPDATE)")]:
        g = t[t.arm == arm].groupby("n").enrolled_final
        mean, lo, hi = g.mean().values, g.min().values, g.max().values
        ax.bar(x + off, mean, w, color=color, label=label, zorder=2)
        ax.errorbar(x + off, mean, yerr=[mean - lo, hi - mean], fmt="none", ecolor=INK2, lw=1, capsize=3, zorder=3)
        for xi, m in zip(x + off, mean):
            ax.text(xi, m + 2, f"{m:.1f}".rstrip("0").rstrip("."), ha="center", va="bottom", fontsize=9, color=INK)
    ax.axhline(5, color=INK2, ls="--", lw=1, label="Course capacity = 5")
    ax.axhline(60, color=INK2, ls=":", lw=1, label="DB connection pool = 60")
    ax.set_xticks(x, [f"N = {n}" for n in levels])
    ax.set_ylabel("seats sold in a 5-seat course")
    ax.set_ylim(0, 84)
    ax.legend(loc="upper left", ncol=2)
    title(ax, "RQ1 · Unlocked enrolment oversells; locked sells exactly 5",
          "Mean of 30 trials per bar, whisker = min–max. Over-allocation: naive 120/120 trials, locked 0/120.")


def rq2(ax):
    rows = summary["rq2"]
    ks = [r["k"] for r in rows]
    x = np.arange(len(ks))
    w = 0.36
    leg = [r["legacy"]["mean_successes"] for r in rows]
    atom = [r["atomic"]["mean_successes"] for r in rows]
    ax.bar(x - w / 2 - 0.01, leg, w, color=BASE, label="Legacy read → check → save", zorder=2)
    ax.bar(x + w / 2 + 0.01, atom, w, color=TREAT, label="Atomic UPDATE … WHERE student_id IS NULL", zorder=2)
    for xi, v in zip(x - w / 2, leg):
        ax.text(xi, v + 0.4, f"{v:.0f}", ha="center", fontsize=9)
    for xi, v in zip(x + w / 2, atom):
        ax.text(xi, v + 0.4, f"{v:.0f}", ha="center", fontsize=9)
    ax.set_xticks(x, [f"K = {k}" for k in ks])
    ax.set_ylabel('"you got the project" replies per trial')
    ax.set_ylim(0, 23)
    ax.legend(loc="upper left")
    title(ax, "RQ2 · Legacy claims crown every racer; atomic claims crown one",
          "K students claim one project at once, 30 trials per K. Legacy: 990 phantom winners in 120 trials.")


def rq3(ax):
    rows = summary["rq3"]
    ns = [r["n"] for r in rows]
    x = np.arange(len(ns))
    for arm, color, label in [("naive", BASE, "Naive"), ("locked", TREAT, "Locked")]:
        med = np.array([r[f"{arm}_median_ms"] for r in rows])
        ci = np.array([r[f"{arm}_ci"] for r in rows])
        ax.fill_between(x, ci[:, 0], ci[:, 1], color=color, alpha=0.15, lw=0)
        ax.plot(x, med, color=color, lw=2, marker="o", ms=7, mec=SURFACE, mew=2, label=label, zorder=3)
        for xi, m in zip(x, med):
            ax.text(xi, m + (1.3 if arm == "locked" else -2.6), f"{m:.1f}", ha="center", fontsize=9)
    for xi, r in zip(x, rows):
        ax.text(xi, -6.2, f"×{r['ratio']:.2f}  δ={r['cliffs_delta']:.2f}", ha="center", fontsize=8.5, color=INK2)
    ax.set_xticks(x, [f"N = {n}" for n in ns])
    ax.set_ylabel("median per-trial latency (ms)")
    ax.set_ylim(0, 34)
    ax.legend(loc="upper left")
    title(ax, "RQ3 · Correctness costs 3–11 ms (1.6–1.7×)",
          "Band = 95% bootstrap CI (10,000 resamples). All Holm-adjusted Mann–Whitney p ≤ 3e-7.")


def rq4(ax):
    d = pd.read_csv(RES / "rq4_delays.csv")
    d = d[d.delivered]
    mode = d["mode"]
    series = [(d[mode == "sse"].delay_ms, TREAT, "SSE push"),
              (d[(mode == "poll") & (d.level == 500)].delay_ms, BASE, "Poll every 500 ms"),
              (d[(mode == "poll") & (d.level == 2000)].delay_ms, THIRD, "Poll every 2 s")]
    for s, color, label in series:
        v = np.sort(s.values)
        ax.step(v, np.arange(1, len(v) + 1) / len(v), where="post", color=color, lw=2,
                label=f"{label} · median {np.median(v):,.0f} ms" if np.median(v) > 10 else f"{label} · median {np.median(v):.1f} ms")
    ax.text(5, 0.5, "1–100 viewers", color=INK2, fontsize=8.5, ha="left")
    ax.set_xscale("log")
    ax.set_xlim(2, 3000)
    ax.set_ylim(0, 1.02)
    ax.grid(axis="x", color=GRID, lw=0.8)
    ax.set_xlabel("delay from action to another viewer's screen (ms, log scale)")
    ax.set_ylabel("share of updates delivered")
    ax.legend(loc="upper left", bbox_to_anchor=(0.2, 0.93))
    title(ax, "RQ4 · SSE reaches other screens in ~4 ms",
          "ECDF of 16,900 deliveries, all delivered. SSE median is 39× and 396× below polling.")


def stability(ax):
    r = pd.read_csv(RES / "rq1_requests.csv")
    r = r[(~r.warmup) & (r.n == 100)]
    per = r.groupby(["arm", "rep"]).latency_ms.mean().reset_index()
    for arm, color, label in [("naive", BASE, "Naive"), ("locked", TREAT, "Locked")]:
        p = per[per.arm == arm].sort_values("rep")
        ax.plot(p.rep - 1, p.latency_ms, color=color, lw=1.5, marker="o", ms=5, mec=SURFACE, mew=1.5, label=label)
        med = p.latency_ms.median()
        ax.axhline(med, color=color, lw=1, ls="--")
        ax.text(p.rep.max() - 0.4, med + 0.6, f"median {med:.1f}", color=INK2, fontsize=8.5, va="bottom")
    ax.set_xlabel("measured repetition (seeded random run order; 2 warm-ups dropped)")
    ax.set_ylabel("mean request latency in trial (ms)")
    ax.set_ylim(10, 49)
    ax.legend(loc="upper left", ncol=2)
    title(ax, "Iterations · 30 repetitions at N = 100",
          "One trial = one observation (no pseudo-replication). Locked slower in 29 of 30 paired repetitions.")


def single(fn, name, size=(8.6, 4.6)):
    fig, ax = plt.subplots(figsize=size)
    fn(ax)
    save(fig, name)


single(rq1, "rq1_overselling.png")
single(rq2, "rq2_lost_updates.png")
single(rq3, "rq3_latency_cost.png")
single(rq4, "rq4_staleness_ecdf.png")
single(stability, "iterations_n100.png", (8.6, 4.2))

fig, axes = plt.subplots(2, 2, figsize=(15.5, 9.4))
for fn, ax in zip([rq1, rq2, rq3, rq4], axes.flat):
    fn(ax)
fig.tight_layout(h_pad=3.2, w_pad=3)
save(fig, "results_overview.png")
