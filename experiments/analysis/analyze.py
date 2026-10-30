"""Statistical analysis for the course-registration experiments.

Reads the CSVs written by experiments/bench and writes:
  summary.json   every number quoted in the report
  figures/*.png  diagnostic plots (the report's charts are drawn from summary.json)

Usage: python analyze.py <results dir>
"""
import json
import sys
from pathlib import Path

import numpy as np
import pandas as pd
from scipy import stats
from statsmodels.stats.multitest import multipletests
from statsmodels.stats.proportion import proportion_confint

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt  # noqa: E402

RNG = np.random.default_rng(20261029)
B = 10_000  # bootstrap resamples

res = Path(sys.argv[1])
fig_dir = res / "figures"
fig_dir.mkdir(exist_ok=True)


def wilson(k, n):
    lo, hi = proportion_confint(k, n, alpha=0.05, method="wilson")
    return [round(float(lo), 4), round(float(hi), 4)]


def boot_ci(x, stat=np.median):
    x = np.asarray(x)
    idx = RNG.integers(0, len(x), size=(B, len(x)))
    reps = stat(x[idx], axis=1)
    return [round(float(np.percentile(reps, 2.5)), 3), round(float(np.percentile(reps, 97.5)), 3)]


def boot_ratio_ci(a, b, stat=np.median):
    a, b = np.asarray(a), np.asarray(b)
    ia = RNG.integers(0, len(a), size=(B, len(a)))
    ib = RNG.integers(0, len(b), size=(B, len(b)))
    r = stat(a[ia], axis=1) / stat(b[ib], axis=1)
    return [round(float(np.percentile(r, 2.5)), 3), round(float(np.percentile(r, 97.5)), 3)]


def cliffs_delta(a, b):
    a, b = np.asarray(a), np.asarray(b)
    gt = (a[:, None] > b[None, :]).sum()
    lt = (a[:, None] < b[None, :]).sum()
    return float((gt - lt) / (len(a) * len(b)))


def magnitude(d):
    d = abs(d)
    return "negligible" if d < 0.147 else "small" if d < 0.33 else "medium" if d < 0.474 else "large"


def r(x, n=3):
    return round(float(x), n)


out = {}

# ── RQ1: over-allocation ─────────────────────────────────────────────
t1 = pd.read_csv(res / "rq1_trials.csv")
q1 = pd.read_csv(res / "rq1_requests.csv")
t1m = t1[~t1.warmup].copy()
q1m = q1[~q1.warmup].copy()
rq1 = []
pvals = []
for n, g in t1m.groupby("n"):
    row = {"n": int(n)}
    for arm in ["naive", "locked"]:
        a = g[g.arm == arm]
        k = int((a.overbooked > 0).sum())
        row[arm] = {
            "trials": int(len(a)),
            "overbooked_trials": k,
            "rate": r(k / len(a), 4),
            "rate_ci": wilson(k, len(a)),
            "mean_overbook": r(a.overbooked.mean()),
            "mean_overbook_ci": boot_ci(a.overbooked, np.mean),
            "max_overbook": int(a.overbooked.max()),
            "mean_enrolled": r(a.enrolled_final.mean()),
            "successes_match_rows": bool((a.successes == a.enrolled_final).all()),
            "errors": int(a.errors.sum()),
        }
    nv, lk = g[g.arm == "naive"], g[g.arm == "locked"]
    table = [[row["naive"]["overbooked_trials"], len(nv) - row["naive"]["overbooked_trials"]],
             [row["locked"]["overbooked_trials"], len(lk) - row["locked"]["overbooked_trials"]]]
    _, p = stats.fisher_exact(table, alternative="greater")
    row["fisher_p"] = float(p)
    row["risk_difference"] = r(row["naive"]["rate"] - row["locked"]["rate"], 4)
    pvals.append(p)
    rq1.append(row)
holm = multipletests(pvals, method="holm")[1]
for row, p in zip(rq1, holm):
    row["fisher_p_holm"] = float(p)
locked_all = t1m[t1m.arm == "locked"]
out["rq1"] = {
    "levels": rq1,
    "locked_total_trials": int(len(locked_all)),
    "locked_total_overbooked": int((locked_all.overbooked > 0).sum()),
    "locked_upper95_exact": r(1 - 0.05 ** (1 / len(locked_all)), 4),  # Clopper-Pearson bound for 0/n
    "naive_total_trials": int((t1m.arm == "naive").sum()),
    "naive_total_overbooked": int(((t1m.arm == "naive") & (t1m.overbooked > 0)).sum()),
    "requests_measured": int(len(q1m)),
    "request_outcomes": q1m.groupby(["arm", "outcome"]).size().to_dict().__repr__(),
}
# Spearman: does naive over-allocation grow with N?
nv = t1m[t1m.arm == "naive"]
rho, p_rho = stats.spearmanr(nv.n, nv.overbooked)
out["rq1"]["naive_trend_spearman"] = {"rho": r(rho), "p": float(p_rho)}
# linear fit of mean overbook vs N (descriptive)
means = nv.groupby("n").overbooked.mean()
slope, intercept, r_lin, p_lin, se = stats.linregress(means.index, means.values)
out["rq1"]["naive_linear_fit"] = {"slope_per_student": r(slope, 4), "intercept": r(intercept), "r2": r(r_lin ** 2)}
# iteration stability: per-rep mean overbook (naive) across repetitions, by N
out["rq1"]["naive_by_rep"] = {
    int(n): [r(v) for v in g.sort_values("rep").overbooked.tolist()] for n, g in nv.groupby("n")
}
# distribution of overbook values for naive per N
out["rq1"]["naive_overbook_quantiles"] = {
    int(n): {q: r(g.overbooked.quantile(q / 100)) for q in (0, 25, 50, 75, 100)} for n, g in nv.groupby("n")
}

# ── RQ3: latency cost (unit = trial) ─────────────────────────────────
trial_lat = q1m.groupby(["trial_id", "arm", "n"]).latency_ms.agg(
    mean="mean", p95=lambda x: np.percentile(x, 95)).reset_index()
rq3, pv3 = [], []
for n, g in trial_lat.groupby("n"):
    a = g[g.arm == "locked"]["mean"].values
    b = g[g.arm == "naive"]["mean"].values
    a95 = g[g.arm == "locked"]["p95"].values
    b95 = g[g.arm == "naive"]["p95"].values
    u, p = stats.mannwhitneyu(a, b, alternative="two-sided")
    d = cliffs_delta(a, b)
    pv3.append(p)
    rq3.append({
        "n": int(n),
        "locked_median_ms": r(np.median(a)), "locked_ci": boot_ci(a),
        "naive_median_ms": r(np.median(b)), "naive_ci": boot_ci(b),
        "ratio": r(np.median(a) / np.median(b)), "ratio_ci": boot_ratio_ci(a, b),
        "locked_p95_median_ms": r(np.median(a95)), "naive_p95_median_ms": r(np.median(b95)),
        "mann_whitney_u": float(u), "p": float(p), "cliffs_delta": r(d), "magnitude": magnitude(d),
        "locked_cv": r(np.std(a, ddof=1) / np.mean(a)), "naive_cv": r(np.std(b, ddof=1) / np.mean(b)),
    })
for row, p in zip(rq3, multipletests(pv3, method="holm")[1]):
    row["p_holm"] = float(p)
out["rq3"] = rq3
# wall time per trial
out["rq3_wall"] = {f"{arm}-{int(n)}": r(g.wall_ms.median()) for (arm, n), g in t1m.groupby(["arm", "n"])}

# ── RQ2: project lost updates ────────────────────────────────────────
t2 = pd.read_csv(res / "rq2_trials.csv")
t2m = t2[~t2.warmup]
rq2, pv2 = [], []
for k, g in t2m.groupby("k"):
    row = {"k": int(k)}
    for arm in ["legacy", "atomic"]:
        a = g[g.arm == arm]
        bad = int((a.lost_updates > 0).sum())
        row[arm] = {
            "trials": int(len(a)), "trials_with_lost_update": bad, "rate": r(bad / len(a), 4),
            "rate_ci": wilson(bad, len(a)), "mean_successes": r(a.successes.mean()),
            "mean_successes_ci": boot_ci(a.successes, np.mean), "max_successes": int(a.successes.max()),
            "phantom_winners": int(a.phantom_winners.sum()), "server_errors": int(a.errors.sum()),
        }
    lg, at = g[g.arm == "legacy"], g[g.arm == "atomic"]
    tbl = [[row["legacy"]["trials_with_lost_update"], len(lg) - row["legacy"]["trials_with_lost_update"]],
           [row["atomic"]["trials_with_lost_update"], len(at) - row["atomic"]["trials_with_lost_update"]]]
    _, p = stats.fisher_exact(tbl, alternative="greater")
    row["fisher_p"] = float(p)
    pv2.append(p)
    rq2.append(row)
for row, p in zip(rq2, multipletests(pv2, method="holm")[1]):
    row["fisher_p_holm"] = float(p)
out["rq2"] = rq2

# ── RQ4: propagation delay ───────────────────────────────────────────
d4 = pd.read_csv(res / "rq4_delays.csv")
deliv = d4.groupby(["mode", "level"]).delivered.mean().to_dict()
ok = d4[d4.delivered]
rq4 = []
for (mode, level), g in ok.groupby(["mode", "level"]):
    x = g.delay_ms.values
    rq4.append({
        "mode": mode, "level": int(level), "samples": int(len(x)),
        "delivery_ratio": r(deliv[(mode, level)], 4),
        "p50": r(np.median(x)), "p50_ci": boot_ci(x),
        "p95": r(np.percentile(x, 95)), "p95_ci": boot_ci(x, lambda v, axis: np.percentile(v, 95, axis=axis)),
        "p99": r(np.percentile(x, 99)), "mean": r(x.mean()), "sd": r(x.std(ddof=1)),
        "post_ms_median": r(g.post_ms.median()),
    })
out["rq4"] = rq4
# SSE vs each polling interval (SSE pooled at S=10, same client count as pollers)
sse10 = ok[(ok["mode"] == "sse") & (ok.level == 10)].delay_ms.values
cmp = []
for T in sorted(ok[ok["mode"] == "poll"].level.unique()):
    pl = ok[(ok["mode"] == "poll") & (ok.level == T)].delay_ms.values
    u, p = stats.mannwhitneyu(sse10, pl, alternative="less")
    # polling model: delay ~ Uniform(0, T) + request time; KS test on delay - median post time
    post = ok[(ok["mode"] == "poll") & (ok.level == T)].post_ms.median()
    ks = stats.kstest(np.clip(pl, 0, None), stats.uniform(loc=0, scale=T + post).cdf)
    cmp.append({"T": int(T), "sse_median": r(np.median(sse10)), "poll_median": r(np.median(pl)),
                "poll_expected_median_model": r(T / 2), "median_ratio": r(np.median(pl) / np.median(sse10), 1),
                "mann_whitney_p": float(p), "cliffs_delta": r(cliffs_delta(sse10, pl)),
                "ks_uniform_D": r(ks.statistic), "ks_uniform_p": float(ks.pvalue)})
out["rq4_compare"] = cmp
# SSE scaling: Spearman between subscriber count and delay
sse = ok[ok["mode"] == "sse"]
rho, p = stats.spearmanr(sse.level, sse.delay_ms)
out["rq4_sse_scaling"] = {"spearman_rho": r(rho), "p": float(p)}
# ECDF points for charts (deciles + tails)
qs = [0, 5, 10, 25, 50, 75, 90, 95, 99, 100]
out["rq4_quantiles"] = {f"{m}-{int(l)}": {q: r(np.percentile(g.delay_ms, q)) for q in qs}
                        for (m, l), g in ok.groupby(["mode", "level"])}

# ── resources ────────────────────────────────────────────────────────
rp = res / "resources.csv"
if rp.exists():
    rs = pd.read_csv(rp)
    rs["cpu"] = rs.cpu_pct.str.rstrip("%").astype(float)
    rs["mem_mib"] = rs.mem_usage.str.split("/").str[0].str.strip().apply(
        lambda s: float(s[:-3]) * (1024 if s.endswith("GiB") else 1) if s.endswith("iB") else float(s.rstrip("B")) / 2**20)
    out["resources"] = {c: {"samples": int(len(g)), "cpu_median": r(g.cpu.median()), "cpu_max": r(g.cpu.max()),
                            "mem_median_mib": r(g.mem_mib.median()), "mem_max_mib": r(g.mem_mib.max())}
                        for c, g in rs.groupby("container")}

out["manifest"] = json.loads((res / "manifest.json").read_text())
(res / "summary.json").write_text(json.dumps(out, indent=2, default=str))

# ── diagnostic figures ───────────────────────────────────────────────
fig, ax = plt.subplots(figsize=(7, 4))
for arm, c in [("naive", "tab:red"), ("locked", "tab:blue")]:
    xs = [row["n"] for row in rq1]
    ys = [row[arm]["mean_overbook"] for row in rq1]
    lo = [row[arm]["mean_overbook"] - row[arm]["mean_overbook_ci"][0] for row in rq1]
    hi = [row[arm]["mean_overbook_ci"][1] - row[arm]["mean_overbook"] for row in rq1]
    ax.errorbar(xs, ys, yerr=[lo, hi], marker="o", capsize=4, label=arm, color=c)
ax.set_xlabel("concurrent students N")
ax.set_ylabel("extra seats sold (mean, 95% CI)")
ax.legend()
fig.tight_layout()
fig.savefig(fig_dir / "rq1_overbook.png", dpi=130)

fig, ax = plt.subplots(figsize=(7, 4))
for (m, l), g in ok.groupby(["mode", "level"]):
    x = np.sort(g.delay_ms.values)
    ax.plot(x, np.arange(1, len(x) + 1) / len(x), label=f"{m} {l}")
ax.set_xscale("log")
ax.set_xlabel("action-to-screen delay (ms, log)")
ax.set_ylabel("ECDF")
ax.legend(fontsize=7)
fig.tight_layout()
fig.savefig(fig_dir / "rq4_ecdf.png", dpi=130)
print(json.dumps({k: out[k] for k in ("rq3", "rq4_compare")}, indent=1)[:3000])
