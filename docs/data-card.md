# Data card: CourseReg experiment run 2026-10-29

| | |
|---|---|
| **Location** | [`experiments/results/2026-10-29/`](../experiments/results/2026-10-29/) |
| **Collected** | 2026-10-29, 05:00:29–05:03:20 UTC, one host |
| **Generator** | [`experiments/bench/main.go`](../experiments/bench/main.go), seed `20261029`, run tag `BX1029T0500` |
| **Code fingerprint** | SHA-256 over all `.go` sources: `ad717c4fa68aefefc9d7aba9822afeb654c17148c3ad6cba6f74d738faf2dca1` |
| **Nature** | Synthetic. Measurements of a system under generated load. **No personal data.** Names in the demo dashboard are randomly generated. |
| **License** | MIT, same as the code |

## Files

| File | Rows | Grain | Key columns |
|---|---|---|---|
| `rq1_trials.csv` | 256 | one enrolment trial | `trial_id, arm, n, capacity, rep, warmup, order, started_at, wall_ms, successes, full, errors, enrolled_final, overbooked` |
| `rq1_requests.csv` | 11,840 | one enrolment request | `trial_id, arm, n, rep, warmup, req, offset_ms, latency_ms, outcome, error` |
| `rq2_trials.csv` | 256 | one project-claim trial | `trial_id, arm, k, rep, warmup, order, started_at, wall_ms, successes, conflicts, errors, final_owner_set, phantom_winners, lost_updates` |
| `rq2_requests.csv` | 2,368 | one claim request | same shape as RQ1 |
| `rq4_delays.csv` | 16,900 | one delivery to one viewer | `mode, level, event, client, action, sent_at, post_ms, delay_ms, delivered` |
| `resources.csv` | 159 | one container sample (53 × 3 containers) | `ts, container, cpu_pct, mem_usage` |
| `manifest.json` | 1 | the run | versions, CPU count, isolation level, all parameters, seed, hash |
| `summary.json` | 1 | derived | every statistic quoted in the README and report |
| `figures/` | 2 | diagnostic | plots written by `analyze.py` |

## Column notes

- `arm`: `naive` / `locked` (RQ1), `legacy` / `atomic` (RQ2).
- `warmup = true` rows are kept for transparency and **excluded from every analysis**.
- `order`: position of the trial in the shuffled run order.
- `outcome`: `ok` or `full` (RQ1), `ok` or `conflict` (RQ2); the `error` column holds any server error text (none occurred).
- `enrolled_final`: enrolments read back from the database after the trial; `overbooked = max(0, enrolled_final − capacity)`.
- `delay_ms`: time from the action's send to its arrival at a viewer; `post_ms` is the actor's own request latency.
- Times are UTC (RFC 3339); durations are milliseconds.

## Quality checks performed

- Every reported success was matched against a stored row (`successes_match_rows = true` in `summary.json`).
- Zero server errors in all 512 trials.
- 16,900 of 16,900 RQ4 deliveries arrived (`delivered = true`).
- The source hash was recomputed after analysis and still matched `manifest.json`.

## Known issues

- RQ4 polling actions were phase-locked to the pollers' clocks: about 10 effective samples per interval (see [methodology §6](methodology.md#6-deviations-and-threats-found-after-the-run)).
- Single host and single date; not representative of internet latency.
