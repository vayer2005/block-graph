# Graph patterns (and how I read them for risk)

AML training and those infographics always boil this down to “patterns in the graph → risk insights.” Fair enough, but worth being explicit: these are **structural** signals. Good for triage and explaining *why* something looks weird, not proof of anything by itself.

I’m mostly thinking in terms of what shows up in the literature around AML systems and synthetic transaction data for models. Below is my own map from those shapes to what we’re doing with a **transaction-ID graph** (`txid` nodes, spend edges).

---

### Long hop chains

Long directed path — A → B → C → … — the winding line you see in slides. People associate that with **layering** (money stepping through intermediaries). The thing you actually measure is hop count / path length.

On a txid graph this is pretty natural: an edge A → B means B spends an output of A, so a long path is literally a chain of spends. Custody moves, normal payments, and weird stuff all make chains, so **length vs some baseline** is the useful part, not “long = bad.”

---

### Fan-out

One node, lots of outgoing edges to different places. Often mentioned next to structuring / smurfing — splitting flow into many branches.

Two ways this shows up on tx→tx: (1) one transaction with many **child** transactions (high out-degree from that tx in the spend graph), and (2) one tx with many outputs that get spent separately — fan-out at the tx level before you even go another hop. Batch payouts, mining pools, messy change patterns can all look like fan-out, so I treat it as “needs context,” not a smoking gun.

---

### Fan-in

Flip it: many nodes into one. Read as **consolidation**.

Here that’s “many distinct input transactions feeding one tx T” — lots of parents in the spend graph. Exchanges and wallet sweeps do this constantly, so high fan-in at a service hub is often normal.

---

### Gather-scatter (hourglass / bowtie)

Many → one hub → many. Pass-through / mule-y narrative: aggregate, then redistribute.

If the hub is a transaction (or a tight cluster of txs), the tx graph fits: many parents, hub tx H, many children. Processors, mixers, anything hub-shaped can mimic it — amounts, timing, and whether you know the entity matter more than the silhouette.

---

### Scatter-gather

Few sources → branch through intermediates → one sink. Split then reconverge; layered consolidation, sometimes with extra hops to muddy the trail from sources to sink.

You see this as branch-then-merge in the spend graph (diamond-ish routes into one sink). Single-parent tree tricks aren’t always enough; you might need path enumeration or flow-style summaries. Normal business graphs branch and merge too.

---

### Cycles

Closed directed loop A → … → A. Slides tie that to round-tripping / wash trading.

Important nuance for **Bitcoin UTXO spend as tx → tx**: you basically get a **DAG**. Spending only consumes existing outputs, so you don’t get a true directed cycle on transaction nodes alone — no A → … → A along txids in the strict sense.

Cycles *do* show up if you build **address** or **entity** graphs: A pays B in one tx, later B pays A, etc. So the infographic “cycle” story is really address/cluster behavior, not raw txid DAG semantics.

---

### Random / unstructured

No strong motif — not a long path, not a hub, not obvious fan-in/out. I use this mentally as a **baseline**: what does “typical” look like in the same window when you’re comparing path length, degree, hub participation.

---

### Bipartite-ish

Two groups, edges mostly between groups, not inside. Coordinated two-sided activity in some writeups.

Pure tx nodes don’t fall into two sides without extra labels (counterparty type, cluster, service tags). Address/entity graphs with metadata, or explicit user↔merchant bipartite models when you have off-chain labels, are the natural home. Possible on enriched data; not something I assume from raw txid-only graphs.

---

### Stack (layered mesh)

Several “layers” of nodes, dense forward flow — sandwich / tiered look. Training materials sometimes tie big versions to organized layering typologies.

On tx graphs you still get multi-layer **forward** flow. A dense mesh means lots of parallel paths — layered path metrics, k-core-ish stuff, betweenness on subgraphs (expensive). Busy exchange periods also go dense, so again: compare to a random-ish baseline in the same slice.

---

### Does txid-as-node actually see these?

| Pattern | txid-friendly? | Quick note |
|--------|----------------|------------|
| Long chains | yes | path length in spend graph |
| Fan-out | yes | many children / many outputs spent apart |
| Fan-in | yes | many parents into one tx |
| Gather-scatter | yes | hub is a tx: many→one→many |
| Scatter-gather | mostly | branch/merge; may need paths or flow view |
| Cycles | no (strict DAG) | use address/entity or identity for “round trip” |
| Random baseline | yes | compare metrics to local typical |
| Bipartite | rarely raw | needs labels or different graph |
| Stack / mesh | partial | layered forward yes; dense mesh needs care + baseline |

---

### BlockGraph (how I’m using this)

Our main view is transactions as nodes, spends as directed edges. That’s a good fit for chains, fan-in/out, gather-scatter, scatter-gather, with the caveats above.

I’m not looking for directed cycles on raw Bitcoin tx→tx — if we care about “cycles,” that’s address/entity level or we change the graph.

And I always want it in writing somewhere that **structure ≠ intent**: exchanges, mixers, and boring commerce all produce the same shapes.

---

### References

Same AML / synthetic-transaction context as most course slides. Infographic line I keep seeing: *“Patterns in the graph can yield risk insights.”*

Video I had open for the relevant bit: [YouTube ~5:52](https://www.youtube.com/watch?v=szLUNcUwbVE&t=352s).
