# Experiments

Reproduces the measurements in the report "Concurrency-Safe Real-Time Course Registration".

| RQ | What | Baseline vs treatment |
|---|---|---|
| RQ1/RQ3 | Seat over-allocation and latency, N = 10/25/50/100 students racing for 5 seats | naive check-then-insert vs locked service path |
| RQ2 | Lost project claims, K = 2/5/10/20 claimants | legacy read-check-save vs atomic conditional UPDATE |
| RQ4 | Action-to-screen delay | SSE (1/10/50/100 viewers) vs polling every 500/2000 ms |

## Rerun
```
docker compose up -d                       # db + service must be running (RQ4 uses the service)
curl -X POST localhost:8080/simulator/stop  # keep background traffic off

docker run --rm --network go-student-service-complete_default -e DB_HOST=db \
  -v "$PWD:/app" -w /app golang:1.20-alpine \
  go run ./experiments/bench -out experiments/results/<date>

pip install -r experiments/analysis/requirements.txt
python experiments/analysis/analyze.py experiments/results/<date>
```
Flags: `-reps` (30), `-warmup` (2), `-seed` (20261029), `-only rq1,rq2,rq4`, `-pool` (60).
Benchmark rows are tagged and deleted afterwards (`-cleanup=false` keeps them).

## Outputs (one folder per run)
| File | One row per |
|---|---|
| `rq1_trials.csv`, `rq2_trials.csv` | trial: arm, level, repetition, warm-up flag, UTC start, wall time, successes, conflicts, errors, final DB state |
| `rq1_requests.csv`, `rq2_requests.csv` | request: start offset, latency, outcome, error text |
| `rq4_delays.csv` | delivery: mode, level, event, client, action, send time, POST time, delay, delivered |
| `resources.csv` | container CPU / memory sample (host `docker stats`) |
| `manifest.json` | run: versions, CPU count, parameters, seed, SHA-256 of all `.go` sources |
| `summary.json`, `figures/` | derived statistics and diagnostic plots from `analyze.py` |

The 2026-10-29 run's manifest hash is `ad717c4fa68a…`; recompute it over the `.go` files to confirm the code is unchanged.
