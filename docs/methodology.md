# Methodology

This page documents how the experiment was designed, controlled and analysed, so that each result can be checked against the procedure that produced it. Full narrative: [research-report.pdf](research-report.pdf).

## 1. Research questions and arms

Each experiment compares the shipped implementation (**treatment**) with a **baseline** that reproduces the earlier, unprotected logic on the same database, models and connection pool.

| RQ | Baseline arm | Treatment arm | Factor (levels) | Primary metric |
|---|---|---|---|---|
| RQ1 | `naiveEnroll`: count, then insert, no locks | `services.Enroll`: `SELECT … FOR UPDATE` in sorted order | concurrent students N = 10, 25, 50, 100; capacity C = 5 | over-allocation O = max(0, enrolled − C) |
| RQ2 | `legacyAssign`: read, check `student_id IS NULL`, `Save` | `services.AssignProject`: `UPDATE … WHERE student_id IS NULL` | concurrent claimants K = 2, 5, 10, 20 | lost updates L = successes − 1 |
| RQ3 | same as RQ1 | same as RQ1 | same as RQ1 | per-trial mean and p95 request latency (ms) |
| RQ4 | HTTP polling, 10 clients, T = 500 / 2,000 ms, 40 events | SSE, S = 1, 10, 50, 100 subscribers, 100 events | as listed | action-to-screen delay (ms), delivery ratio |

## 2. Hypotheses (fixed before data collection)

- **H1.** The naive arm over-allocates with a probability that grows with N; the locked arm never does.
- **H2.** The legacy arm reports more than one winner for one project; the atomic arm never does.
- **H3.** Locking raises latency, with an effect that grows with N.
- **H4.** Median SSE delay is below both polling intervals, and polling delay ≈ Uniform(0, T) + one round trip.

## 3. Design of experiments

| Element | Choice | Reason |
|---|---|---|
| Layout | Full factorial: 2 arms × 4 levels per RQ | every arm sees every level |
| Replication | 30 measured repetitions per cell + 2 warm-ups (discarded) | 0/120 per arm bounds the failure rate below 2.5% (rule of three) |
| Randomisation | cell order shuffled per repetition with seed `20261029` | slow drift (cache, table growth) cannot favour one arm |
| Independence | fresh course / project per trial, tagged with a run id | no carry-over between trials |
| Blocking the other rule | benchmark students: 1,000-credit limit; courses: 1 credit | only the seat rule can bind in RQ1 |
| Simultaneity | goroutines block on a barrier and are released together | maximum contention, the realistic worst case |
| Fairness | both arms share one 60-connection pool and write the same activity row after commit | arms differ only in locking |
| Background noise | traffic simulator off | no unrelated writes |
| Clean-up | rows carrying the run tag are deleted after the run | the demo database stays untouched |

## 4. Measurement

| Source | Recorded per row |
|---|---|
| `rq1_trials.csv`, `rq2_trials.csv` | trial id, arm, level, repetition, warm-up flag, run order, UTC start, wall time, successes, conflicts, server errors, **final database state** |
| `rq1_requests.csv`, `rq2_requests.csv` | start offset from release, latency, outcome, error text |
| `rq4_delays.csv` | mode, level, event, client, action, send time, POST latency, delay, delivered flag |
| `resources.csv` | `docker stats` CPU % and memory per container, about every 3 s |

The final database state is read back after every trial. A reported success is therefore checked against a stored row, not trusted from the HTTP reply.

## 5. Statistical analysis plan

| Question | Method | Notes |
|---|---|---|
| How often does each arm fail? | proportion with **Wilson** 95% CI | stable near 0 and 1 |
| Do the arms differ? | one-sided **Fisher exact** test per level | exact for small counts |
| How safe is "zero failures"? | exact one-sided upper bound (≈ rule of three) | 0/120 → < 2.5% |
| Latency difference | **Mann–Whitney U** on per-trial means | no normality assumption |
| Effect size | **Cliff's δ**, Romano thresholds (0.147 / 0.33 / 0.474) | ordinal, robust to tails |
| Uncertainty of medians and ratios | **bootstrap**, 10,000 resamples, percentile CI | |
| Many levels tested | **Holm** step-down correction, α = 0.05 | controls family-wise error |
| Trend with load | **Spearman ρ** | monotone, not linear |
| Distribution model | **Kolmogorov–Smirnov** vs Uniform(0, T) | tests H4's second part |

**Unit of analysis.** Requests inside one trial compete for the same rows and are not independent. All tests use **one trial = one observation** (n = 30 per cell). Testing the 9,300 requests directly would be pseudo-replication and would overstate the evidence.

## 6. Deviations and threats found after the run

- **RQ4 phase-locking.** Each poller kept a fixed phase, and the next action was sent 50–150 ms after the slowest poller saw the previous one. The effective sample is 10 phases per interval, not 400. The uniform model was rejected for this reason, and the polling CIs are too narrow. The SSE-vs-polling conclusion is unaffected (the fastest polling p5 is 70 ms; the slowest SSE p99 is 8 ms).
- **Pool-bound plateau.** The naive arm's over-allocation levels off at the 60-connection pool size, so the RQ1 effect size depends on that setting.
- **Table growth.** Benchmark students accumulate enrolments, which slowly lengthens the credit-sum query. Randomised order spreads this over both arms.

## 7. Iterations

- **Pilot.** A manual burst of 40 parallel enrolments into a 5-seat course returned exactly 5 × `201` and 35 × `409` after locking was added.
- **Main run.** 32 repetitions of every cell (2 warm-ups + 30 measured) on 2026-10-29, 05:00:29–05:03:20 UTC. Per-repetition results are plotted in [`figures/iterations_n100.png`](../figures/iterations_n100.png). The locked arm was slower in 28–29 of 30 paired repetitions at every N.
