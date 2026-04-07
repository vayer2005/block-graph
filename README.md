# Graph fraud: ingestion, merge, and patterns

**Source:** [YouTube ~5:52](https://www.youtube.com/watch?v=szLUNcUwbVE&t=352s) (AML-style framing: patterns in the transaction graph and what they can suggest for risk triage).

**This lecture was the project's inspiration.**

Graph signals from the merged transaction graph (degrees, path-style and hub-style summaries) surface in the anomaly dashboard; the screenshot below is an example of that view.

![Graph signals: anomaly dashboard screenshot](gifs/Screenshot%202026-04-06%20at%205.31.41%E2%80%AFPM.png)

---

This repository is an ingestion pipeline and graph-building experiment for Bitcoin **transaction-ID graphs** (nodes are `txid`s, directed edges are spends). Recent blocks are fetched, each block is turned into a subgraph of edges, subgraphs are merged into one graph, and downstream code computes structural “anomaly” style metrics for a small dashboard.

Pipeline / dashboard render

---

## How ingestion and the graph are structured

The spine is three ideas: **parallel block fetch**, **per-block subgraph construction**, and **pairwise merging** until one edge list remains. The intended shape is that merging happens in rounds so that the **depth** of combine steps scales like **log n** in the number of initial subgraphs, instead of repeatedly merging one ever-growing graph with the next block (a **chain**), which grows total merge work in an unbalanced way.

### Block ingestion

Ingestion is driven from a fixed window of recent **confirmed** blocks: the CLI asks for the last *N* blocks from the chain tip (`ingestor.RunWithHandler` with `BlockCount`). Each block needs a full **block body** (transactions), not just headers.

The fetch layer uses **Go worker goroutines** and a **buffered channel of heights** (`jobs`). Every worker loops on `height := range jobs`, resolves the block hash at that height, then loads the block JSON. Work is spread across `cfg.Workers` goroutines so **HTTP latency overlaps**: while one goroutine waits on the network, others keep pulling heights from the queue.

### Block cache

If `DATABASE_URL` is set, a **Postgres-backed cache** stores blocks by height and hash. Before hitting the wire, a worker checks the cache; on a hash match it **skips the HTTP fetch** and deserializes from disk. That makes retries, overlapping runs, and crashes cheaper and gives a single place to see what has already been pulled for an analysis window.

### Parallel HTTP and parallel graph building (specifics)

Fetch and subgraph extraction are **pipelined**, not strictly sequential:

1. **Fetch workers** (`cfg.Workers` goroutines) share the same `jobs` channel and each perform: `BlockHashAtHeight` → optional cache read → `BlockByHash` → write `FetchedBlock` into a `byHeight` map (mutex-protected).
2. When a block is available, the fetch goroutine does **not** call the block handler inline. It starts **another goroutine** with `go func(f FetchedBlock) { handler(ctx, &f) }(fb)` and tracks it with `handlerWg`. So **subgraph construction runs concurrently with further HTTP**: other workers keep fetching while `ProcessBlock` walks transactions, resolves `prev_out` links, and aggregates `(from_txid, to_txid)` edges into a `Subgraph`.
3. `fetchWg.Wait()` waits for all heights to be consumed from the network/cache path; `handlerWg.Wait()` waits until **every** `ProcessBlock` has returned. Errors from either path are collected under a mutex.

In short: **parallel requests** saturate I/O, and **parallel handlers** overlap CPU work for parsing and edge aggregation with ongoing fetches.

### Subgraph building

Each completed block is passed to `features.ProcessBlock`, which builds one subgraph: for each transaction, inputs point to funding transactions; edges are deduplicated and **amounts summed** per `(From, To)` pair. The subgraph is registered under a new id, and its **(id, edge count)** is pushed into the merge coordinator’s heap.

### Merging

`MergeSubgraphs` unions two subgraphs in memory (again summing amounts on duplicate keys). The coordinator runs **merge worker goroutines** that pop **two** entries from a **min-heap ordered by edge count** (then id), merge those ids, delete the parents from the registry, insert the merged graph under a new id (id is a strictly increasing atomic counter), and push the result back until ingest has finished and only one graph remains. This ensures that subgraph constructions happen in a parallel manner rather than requiring a lock to update a main graph. 

### Output

The result is one consolidated; downstream, the code reports degrees (fan-in / fan-out), and other graph anomalies commonly used in fraud detection algorithms (see youtube link below), and related metrics into JSON for the dashboard.

---

## Limitation: chain-style merging vs a balanced log-depth merge

The **design goal** for the merge phase is a **balanced** combine tree: pairwise rounds so that no single intermediate graph absorbs the entire history before others have been merged, so merge **depth** stays **O(log n)** in the number of subgraphs and work stays better balanced across rounds.

**What actually happens today** is visible in a sample run (15 blocks, 15 workers, ~8.5 s wall time): merges follow a **chain** in terms of the growing result (e.g. combine subgraphs `1` and `2` into `3`, then combine that accumulated graph with `4` into `5`, and so on), rather than a tournament where disjoint pairs of similar size merge in each round. That **chaining** means one side of each merge keeps growing through the whole sequence; it **prevents** the balanced **log (n)** merge schedule the doc above describes.

### Observed merge chain (example) -> sequential events from an actual run


| Step | Action                   | Edges   |
| ---- | ------------------------ | ------- |
| -    | Subgraph `1`             | 3,405   |
| -    | Subgraph `2`             | 4,177   |
| 1    | Merge `1` + `2` → `3`    | 7,582   |
| -    | Subgraph `4`             | 6,920   |
| 2    | Merge `3` + `4` → `5`    | 14,502  |
| -    | Subgraph `6`             | 7,269   |
| 3    | Merge `5` + `6` → `7`    | 21,771  |
| -    | Subgraph `8`             | 7,142   |
| 4    | Merge `7` + `8` → `9`    | 28,913  |
| -    | Subgraph `10`            | 8,075   |
| 5    | Merge `9` + `10` → `11`  | 36,988  |
| -    | Subgraph `12`            | 7,358   |
| 6    | Merge `11` + `12` → `13` | 44,346  |
| -    | Subgraph `14`            | 6,553   |
| 7    | Merge `13` + `14` → `15` | 50,899  |
| -    | Subgraph `16`            | 6,324   |
| 8    | Merge `15` + `16` → `17` | 57,223  |
| -    | Subgraph `18`            | 8,013   |
| 9    | Merge `17` + `18` → `19` | 65,236  |
| -    | Subgraph `20`            | 6,795   |
| 10   | Merge `19` + `20` → `21` | 72,031  |
| -    | Subgraph `22`            | 9,030   |
| 11   | Merge `21` + `22` → `23` | 81,061  |
| -    | Subgraph `24`            | 7,261   |
| 12   | Merge `23` + `24` → `25` | 88,322  |
| -    | Subgraph `26`            | 6,334   |
| 13   | Merge `25` + `26` → `27` | 94,656  |
| -    | Subgraph `28`            | 7,157   |
| 14   | Merge `27` + `28` → `29` | 101,813 |


**Result:** final graph id (from atomic counter) `29`, **101,813** edges; **15** blocks fetched.

```text
merged graph id=29 edges=101813
fetched 15 blocks
Time taken: 8.468718292s
```

---

## Graph patterns (and how to read them for risk)

**Important note:** Because this project has no labeled fraud dataset at scale, I’m not training a graph neural network on these features; many production systems instead combine rich graph features with classical models, and larger teams sometimes add deep graph models where data and infrastructure allow.

The mapping below is for a **transaction-ID graph** (`txid` nodes, spend edges).

### Long hop chains

Long directed path A → B → C → … is the classic “layering” slide: money stepping through intermediaries. The measurable quantity is **hop count / path length**. In a txid graph, an edge A → B means B spends an output of A, so a long path is a chain of spends. Custody moves, normal payments, and unusual flows all create chains; **length vs a baseline** matters more than “long = bad.”

### Fan-out

One node with many outgoing edges to different destinations, often discussed next to structuring / smurfing. On tx→tx this appears as (1) one transaction with many **child** transactions, or (2) one tx with many outputs spent separately. Batch payouts, pools, and messy change can look like fan-out; treat as **context-dependent**.

### Fan-in

Many nodes into one: **consolidation**. Here, “many distinct input transactions feeding one tx T.” Exchanges and wallet sweeps do this routinely.

### Gather-scatter (hourglass / bowtie)

Many → one hub → many: pass-through / “mule” narratives. If the hub is a transaction (or a tight cluster), the graph fits: many parents, hub H, many children. Processors and mixers can look similar; amounts, timing, and entity knowledge dominate.

### Scatter-gather

Few sources → branch through intermediates → one sink: split then reconverge. You see branch-then-merge structure (diamond-like routes). May need path enumeration or flow-style summaries; legitimate business graphs also branch and merge.

### Cycles

Slides often tie directed cycles to round-tripping or wash trading. On **Bitcoin tx→tx** spend graphs you effectively get a **DAG**: spends only consume existing outputs, so you do not get a true directed cycle on transaction nodes alone.

Cycles matter more on **address** or **entity** graphs (A pays B, later B pays A). The “cycle” story is usually at that level, not raw txid semantics.

### Random / unstructured

Weak motifs: use as a **baseline** for comparing path length, degree, and hub participation against “typical” in the same window.

### Bipartite-ish

Two groups with edges mostly between groups. Pure tx nodes do not split into two sides without extra labels (counterparty type, cluster, service tags). Natural on enriched or user↔merchant models.

### Stack (layered mesh)

Several layers of nodes with dense forward flow. On tx graphs you still get multi-layer **forward** flow; a dense mesh suggests many parallel paths (layered metrics, k-core-style ideas, betweenness on subgraphs, often expensive). Busy exchange periods also go dense; compare to a baseline.


**BlockGraph usage here:** the main view is transactions as nodes and spends as directed edges, good for chains, fan-in/out, gather-scatter, and scatter-gather with the caveats above. **Structure ≠ intent:** exchanges, mixers, and ordinary commerce all produce similar shapes.

---

## References

The primary video link is at the top of this README under **Source**.

---

## Running the CLI

From the repo root (see `ingestor/cmd/main.go` for flags):

```bash
go run ./ingestor/cmd/
```

Optional Postgres cache: set `DATABASE_URL` or pass `-database-url`. The dashboard JSON path defaults to `anomaly-dashboard.json`.