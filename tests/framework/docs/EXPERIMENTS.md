# Concurrency measurements

The framework has exactly two concurrency levels. Scenarios inside a runner are always
sequential — that is what lets a feature build state across scenarios — so all throughput comes
from adding **runners** (which share one gateway) or **blocks** (which each get their own).

The question worth answering is not "does concurrency help" but **what each axis costs, and
whether they compose**. So the axes are measured crossed, not one at a time.

Every cell ran **twice**. Single samples are not trusted here: this suite already produced a
failure that was 50% reproducible and was briefly called deterministic.

**Environment:** macOS 16 GB / 8 CPU, Colima (`vz`, aarch64) with 6 CPU / 12 GB, docker 27.4.0.
Absolute seconds are host-specific; the ratios and container costs are the transferable part.

---

## The matrix

Per-block workload is held constant: the same 6 features / 64 scenarios over 4 runners.
`B=1` is `gateway-core/sqlite`; `B=2` adds the identical `postgres` leg; `B=3` adds
`gateway-jwt` (19 scenarios). Block concurrency is pinned with `-block-parallel`, runner
concurrency with `-runner-parallel`.

| blocks | runners | rep 1 | rep 2 | mean | spread | containers | scenarios | scen/s |
|---|---|---|---|---|---|---|---|---|
| 1 | 1 | 185.1s | 184.4s | **184.8s** | 0.4% | 4 | 64 | 0.35 |
| 1 | 2 | 120.5s | 103.3s | **111.9s** | 15.4% | 4 | 64 | 0.57 |
| 1 | 4 | 96.3s | 96.4s | **96.3s** | 0.1% | 4 | 64 | 0.66 |
| 2 | 1 | 250.4s | 250.3s | **250.4s** | 0.0% | 7 | 128 | 0.51 |
| 2 | 2 | 107.8s | 160.7s | **134.2s** | 39.4% | 7 | 128 | 0.95 |
| 2 | 4 | 98.6s | 99.7s | **99.2s** | 1.1% | 7 | 128 | 1.29 |
| 3 | 1 | 185.4s | 253.8s | **219.6s** | 31.1% | 9 | 147 | 0.67 |
| 3 | 2 | 105.3s | 166.5s | **135.9s** | 45.0% | 9 | 147 | 1.08 |
| 3 | 4 | 98.1s | 96.2s | **97.2s** | 2.0% | 9 | 147 | 1.51 |

**0 failures in all 18 cells.**

---

## What it says

### 1. The axes compose — neither cannibalises the other

Throughput gain from adding blocks, held at each runner level:

```
R=1:  B1 1.00x   B2 1.48x   B3 1.93x
R=2:  B1 1.00x   B2 1.67x   B3 1.89x
R=4:  B1 1.00x   B2 1.94x   B3 2.28x
```

Speedup from adding runners, held at each block level:

```
B=1:  R1 1.00x   R2 1.65x   R4 1.92x
B=2:  R1 1.00x   R2 1.86x   R4 2.52x
B=3:  R1 1.00x   R2 1.62x   R4 2.26x
```

Both axes keep paying regardless of the other's setting. End to end, `B=1,R=1` to `B=3,R=4`
is **0.35 -> 1.51 scenarios/sec, a 4.3x throughput gain**.

### 2. At R=4, extra blocks are nearly FREE in wall-clock

This is the most useful number in the matrix:

| blocks | wall (R=4) | scenarios | containers |
|---|---|---|---|
| 1 | 96.3s | 64 | 4 |
| 2 | 99.2s | 128 | 7 |
| 3 | 97.2s | 147 | 9 |

Wall-clock is flat within noise (96-99s) while scenarios executed more than doubles. Once
runners are parallel, a block's work overlaps almost perfectly with its siblings' — so a block
buys throughput at a cost paid in **containers, not time**.

The inverse holds too, and matters more for a laptop: at `R=1` the same three blocks take
185 -> 250 -> 220s. Block concurrency without runner concurrency mostly buys contention.

**Set runner concurrency first. Only then add blocks.**

### 3. Runner concurrency stays free; block concurrency never is

| | throughput | container cost |
|---|---|---|
| 1 -> 4 runners | 1.92x | 4 -> 4 (**+0**) |
| 1 -> 3 blocks (at R=4) | 2.28x | 4 -> 9 (**+5**) |

A runner costs contention on a gateway that is already running. A block costs a gateway pair
(plus a database where the matrix leg needs one — that is why `B=2` is 7 containers, not 8:
the sqlite leg needs no database container, the postgres leg does).

Note also what does NOT scale: the shared testbench and Ryuk appear once regardless of block
count. Going 1 -> 3 blocks added only gateway pairs and one database, not a mock container per
block, which is the shared-component design doing its job.

### 4. Per-unit time is unchanged by concurrency — the ceiling is the slowest unit

Per-runner durations, serial vs 4-way concurrent on the same gateway:

| runner | scenarios | serial | 4 concurrent | delta |
|---|---|---|---|---|
| deploy | 7 | 2.65s | 2.74s | +3% |
| routing | 8 | 20.79s | 20.44s | -2% |
| policies | 22 | 64.88s | 64.71s | -0.3% |
| guardrails | 27 | 79.45s | 79.20s | -0.3% |

Three of four got marginally *faster* — noise around zero. So the runner axis is not limited by
contention; it is limited by **imbalance**. Sum of runners is 167.8s and the slowest is 79.2s,
so the theoretical best is 2.12x and we measure 1.92x, the rest being ~15-20s of fixed boot
(4.2s per block) and teardown.

**Balance runners by DURATION, not scenario count.** In this workload `deploy` is 7 scenarios in
2.7s while `guardrails` is 27 in 79s — a 29x spread. Raising `-runner-parallel` past 4 changes
nothing until `guardrails` is split, and scenario count is a poor proxy: `policies` has fewer
scenarios than `guardrails` (22 vs 27) at nearly the same duration.

Separately measured on a 12-runner workload (232 scenarios, one block): R1 497s, R4 233s,
R12 168s — **2.96x**, so the axis keeps scaling well past 4 when there are enough runners to
fill it.

### 5. Intermediate concurrency is the least predictable

Spread across the two reps, by runner setting:

```
R=1:  0.4%   0.0%   31.1%
R=2: 15.4%  39.4%   45.0%     <-- every R=2 cell is the noisiest in its row
R=4:  0.1%   1.1%    2.0%
```

At `R=4` every runner starts at once, so there is no scheduling choice to get wrong. At `R=2`
wall-clock depends on **which pair happens to be scheduled together** — an unlucky pairing puts
the two long runners in sequence. Partial concurrency is where a CI job's runtime becomes
unpredictable, which is worth knowing before picking a value: R=2 is the worst of both worlds.

---

## Host limit found while running this

The first attempt at the 2D matrix used the FULL 232-scenario workload per block. At `B>=2`
that reliably took the docker daemon down mid-run — `Cannot connect to the Docker daemon`,
hundreds of connection-refused errors, and cells recording 96-121 failures that are outage
artifacts, not results. It reproduced on a freshly restarted Colima.

Diagnosis, from the Lima host-agent log rather than guesswork: **the VM never died.** It was
healthy throughout, with 11.5 GB free and no OOM in `dmesg`. What fails is the host's path to
it — Colima forwards every published container port through its own SSH forward, and each
gateway publishes 8 ports. Multiple blocks under sustained load churn enough forwards to make
the daemon socket unresponsive from the host.

It is a function of **sustained load duration x block count**, not block count alone: three
blocks at 42 scenarios / 75s ran clean twice, while two blocks at 464 scenarios / 325s killed
it every time. The matrix above uses a workload this host sustains, and ran all 18 cells with
zero daemon drops.

This is a macOS/Colima property, not a framework or product defect — CI on `ubuntu-latest` has
no VM and no port forwarding in the path. But it is the practical reason to prefer runners over
blocks *on a laptop*: runners add no published ports at all.

---

## What this does NOT measure

- **A saturation point.** Every cell is a fixed sub-saturation workload chosen so the axes stay
  comparable. Nothing here says where throughput stops improving.
- **CI behaviour.** 6-CPU Colima numbers. Ratios should carry; seconds will not.
- **Memory.** Peak container COUNT was sampled, not memory or CPU. "2 containers per block" is
  not a memory budget.
- **Beyond B=3 / R=4** on the crossed matrix (R=12 was measured only at B=1).

One caveat on the table itself: the scenario counter under-reports at high concurrency (three
cells show 56/106/128 where the true counts are 64/147/147). That is a known FRAMEWORK issue —
concurrent runners share one stdout and corrupt each other's godog summary lines; see
PILOT-STATUS.md. The authoritative signals are the process exit code and `--- FAIL` count, both
clean here; the `scen/s` column uses true counts from the feature files.

## Reproducing

```
cd tests/framework/suites/it
W="@api-deploy,@dynamic-endpoint,@cel,@model-round-robin,@content-length-guardrail,@word-count-guardrail"

go test -blocks gateway-core/sqlite        -block-parallel 1 -runner-parallel 4 -feature-tags "$W"
go test -blocks gateway-core               -block-parallel 2 -runner-parallel 4 -feature-tags "$W"
go test -blocks gateway-core,gateway-jwt   -block-parallel 3 -runner-parallel 4 -feature-tags "$W"
```

Use `-block-parallel`, never `-parallel`: the latter is a `go test` builtin that caps
`t.Parallel`, and because the engine nests parallel subtests it DEADLOCKS the suite after boot.
See the flag table in the README.
