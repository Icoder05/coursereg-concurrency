# Literature review

## 1. Concurrency anomalies in enrolment

A capacity check followed by a write is a **time-of-check/time-of-use** race. Berenson et al. [1] showed that the ANSI SQL isolation levels miss several anomalies, including the **lost update (P4)**: transaction T1 reads a value, T2 writes it, and T1 then overwrites T2's write. PostgreSQL's default level, **Read Committed**, gives each statement a fresh snapshot [2]. Two transactions can therefore both run `SELECT COUNT(*)`, both see a free seat, and both `INSERT`.

## 2. Remedies

| Approach | Mechanism | Trade-off |
|---|---|---|
| Pessimistic row locks | `SELECT … FOR UPDATE` on the course row before counting [3] | writers queue; locks must be taken in one consistent order to avoid deadlocks [3] |
| Conditional single write | `UPDATE … WHERE <condition still holds>`; affected-rows = 0 means the condition failed | no wait, but the condition must fit in one statement |
| Constraint or trigger | counter column plus `CHECK (enrolled <= capacity)` maintained by a trigger | logic moves into the database; enforced for every client [6] |
| Serializable isolation | `SERIALIZABLE` plus retry on serialization failure [2] | no explicit locks; retry storms under contention |

A practitioner design record for an enrolment system rejects application-level checks for exactly this race and enforces capacity inside the database [6]. This project uses the first two approaches: row locks for enrolment, and a conditional write for project claims.

## 3. Pushing live updates to browsers

| Transport | Model | Notes |
|---|---|---|
| HTTP polling | client asks every T ms | staleness up to T; load grows with clients × frequency |
| Server-Sent Events | one long-lived `text/event-stream` response [4] | server → client only; automatic reconnect with `Last-Event-ID` |
| WebSockets | full-duplex framed channel [7] | more capable; needs its own protocol and proxy upgrade handling |

Van de Vyvere et al. [5] benchmarked polling against SSE for live open-data interfaces with up to 25,000 clients. SSE used less server CPU, gave lower client latency and served more than twice as many clients. Polling only won when clients accepted a fixed maximum delay.

## 4. Statistical practice for small, repeated experiments

- Proportions near 0 or 1 need intervals that stay in range: the **Wilson** score interval [11].
- When no failures are observed, the exact upper bound (≈ 3/n, the **rule of three**) states what can honestly be claimed [8].
- Latency is heavy-tailed, so rank methods (**Mann–Whitney U** [12]), ordinal effect sizes (**Cliff's δ** with Romano thresholds [9]) and **bootstrap** intervals [10] are preferred to means and t-tests.
- The report follows the **IMRaD** structure [13].

## 5. Research gap

Live-update comparisons focus on throughput at scale [5]. Concurrency guidance for enrolment is mostly informal [6]. No study we found measures, on one system, with repeated trials and uncertainty estimates:

1. how often a naive enrolment path over-allocates as contention grows,
2. the latency price of the locking fix, and
3. end-to-end staleness of SSE versus polling.

This project addresses that gap at small scale.

## References

1. H. Berenson, P. Bernstein, J. Gray, J. Melton, E. O'Neil, P. O'Neil. *A Critique of ANSI SQL Isolation Levels.* Proc. ACM SIGMOD, pp. 1–10, 1995.
2. PostgreSQL Global Development Group. *Transaction Isolation.* PostgreSQL 15 Documentation, §13.2.
3. PostgreSQL Global Development Group. *Explicit Locking.* PostgreSQL 15 Documentation, §13.3.
4. WHATWG. *Server-sent events.* HTML Living Standard, §9.2.
5. B. Van de Vyvere, P. Colpaert, R. Verborgh. *Comparing a Polling and Push-Based Approach for Live Open Data Interfaces.* Proc. ICWE 2020, LNCS 12128, Springer, pp. 87–101.
6. Curriculab. *Optimistic concurrency for enrollment* (architecture decision record). Practitioner documentation.
7. I. Fette, A. Melnikov. *RFC 6455: The WebSocket Protocol.* IETF, December 2011.
8. J. A. Hanley, A. Lippman-Hand. *If nothing goes wrong, is everything all right?* JAMA 249(13):1743–1745, 1983.
9. J. Romano, J. D. Kromrey, J. Coraggio, J. Skowronek. *Appropriate statistics for ordinal level data.* Annual meeting of the Florida Association of Institutional Research, 2006.
10. B. Efron, R. J. Tibshirani. *An Introduction to the Bootstrap.* Chapman & Hall, 1993.
11. E. B. Wilson. *Probable Inference, the Law of Succession, and Statistical Inference.* JASA 22(158):209–212, 1927.
12. H. B. Mann, D. R. Whitney. *On a Test of Whether One of Two Random Variables is Stochastically Larger than the Other.* Annals of Mathematical Statistics 18(1):50–60, 1947.
13. George Mason University Writing Center. *Scientific (IMRaD) Research Reports.*
