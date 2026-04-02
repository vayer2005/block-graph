# Graph patterns and risk-style interpretation

This document lists common **graph topologies** used in financial forensics and AML-style analysis. In training and infographics these are often summarized as “patterns in the graph can yield risk insights.” Each pattern is a **structural** signal. It is useful for triage and explanation, but it is **not** on its own proof of illicit activity.

Sources in the literature often tie these shapes to **Anti-Money Laundering** systems and **synthetic transaction** datasets used to train or evaluate detection models.

---

## 1. Long hop chains

A **long hop chain** is a long directed path: many nodes in sequence, one following another, often drawn as a winding line. In AML discussions this shape is associated with **layering**, moving funds through many intermediaries to obscure origin. The **length** of the path (hop count) is the main signal.

For a **transaction-ID graph** (`txid` nodes), this pattern maps well. An edge **A → B** means that *B spends an output of A*, so a long path **A → B → C → …** is exactly a **chain of spends**. Legitimate custody, payments, and wallet behavior also create chains, so **length relative to a baseline** matters.

---

## 2. Fan-out

**Fan-out** means one source node with many outgoing edges to distinct recipients. It is often discussed next to **structuring** or **smurfing**: splitting flow into many branches, whether many destinations or many small outputs.

On the **transaction-ID graph** the idea applies in two ways. First, **one transaction → many child transactions**: many outputs of **Tx S** may be spent by **different** next transactions (high out-degree from **S** in the spend graph). Second, **one transaction with many outputs** (value split) can show **fan-out** at the **tx** level even before the next hop. Batch payouts, mining pool rewards, and change-heavy transactions can look like fan-out, so context matters.

---

## 3. Fan-in

**Fan-in** is the opposite shape: many nodes pointing **into** one destination. The usual reading is **consolidation**, many sources feeding one sink.

On the **transaction-ID graph**, one **Tx T** with **many distinct input transactions** (many parents in the spend graph) is **fan-in** to **T**. Exchange deposits and wallet sweeps consolidate many inputs, so **high fan-in** is common at service hubs.

---

## 4. Gather-scatter (hourglass / bowtie)

**Gather-scatter** looks like multiple nodes on the left, one **central hub**, then multiple nodes on the right: many-to-one-to-many. It suggests **pass-through** or **mule-like** behavior: aggregate in, then redistribute out.

If the **hub is a transaction** (or a small set of txs), the pattern fits the **transaction-ID graph**: many parents → **Tx H** → many children. Payment processors and mixers can mimic **hub** shapes, so **context** (amounts, timing, known entities) matters.

---

## 5. Scatter-gather

**Scatter-gather** is one or few sources → several **intermediate** nodes → one **final** sink: split, then reconverge. It is read as **layered consolidation**, sometimes with intermediate hops to obfuscate the path from sources to sink.

The **transaction-ID graph** supports this **topology** when you see **branching then merging** in the directed spend graph (for example diamond-like or multi-path routes to one sink). Algorithms may need **path enumeration** or **flow** summaries, not only single-parent trees. Normal business flows can also branch and merge.

---

## 6. Cycles (closed loops)

A **cycle** is a set of directed edges that form a **closed loop** (A → B → … → A). In AML material this is tied to **round-tripping** or **wash trading**: funds move in a circuit so the path returns toward a familiar origin.

For the **Bitcoin UTXO spend graph** with **tx → tx** edges, you do **not** get a true directed cycle in the strict sense. Spending moves **forward in time**: a transaction only spends **existing** outputs, so the spend graph is a **DAG**. You **cannot** have **A → … → A** along transaction nodes alone.

Cycles **can** appear on an **address** or **entity** graph: if **address A** pays **B** in one tx and later **B** pays **A** in another, you can see a **directed cycle** in **address-to-address** flows across different transactions. Behavioral “cycles” are usually modeled with **addresses** or **clusters**, not raw `txid` DAGs.

In short, **cycles** in the infographic are natural for **account/address**-style graphs; they are **not** what you get from pure **transaction DAG** semantics on Bitcoin.

---

## 7. Random (unstructured)

The **random** or unstructured case has sparse edges and no strong motif: no long path, no hub, no obvious fan-in or fan-out. It serves as a **baseline** or “typical” activity class, a **control** or comparison when you build or evaluate detectors.

On the **transaction-ID graph** you can use it as a **reference distribution**: compare **path length**, **degree**, and **hub participation** in a window to **typical** subgraphs in the same setting.

---

## 8. Bipartite-like structure

A **bipartite-like** shape has two groups of nodes; edges run **between** groups, not **within** a group (two-sided matching). It is sometimes linked to **coordinated** or **two-sided** patterns, for example one class of entities interacting mainly with another class.

Raw **transaction** nodes do **not** split cleanly into two “sides” without **extra labels** (counterparty type, cluster, service tag). **Address-to-address** or **entity-to-entity** graphs with **metadata**, or **bipartite** **user ↔ merchant** models when you have off-chain labels, are usually a better fit. The pattern is **possible** with heavy **enrichment**; it is **not** the default on raw `txid` graphs alone.

---

## 9. Stack (layered mesh)

A **stack** shows multiple **layers** (rows) of nodes, dense flow left-to-right through several tiers, a **sandwich** or **stacked** look. It suggests **organized layering** at scale: systematic routing through successive tiers, sometimes tied to complex laundering typologies in training materials.

On the **transaction-ID graph** you can see **multi-layer** **forward** flow (tiers of **tx**). A **dense** “mesh” often implies **many parallel paths**. That structure can be studied with **layered** **path** or **flow** metrics, **k-core**-style analysis, or **betweenness** on **subgraphs** (which can be heavy). Busy exchange or settlement periods also create **dense** graphs, so **contrast** with a **random** baseline.

---

## Can `transaction_id` nodes detect these patterns?

| Pattern | Works well with `txid` nodes? | Notes |
|--------|------------------------|-------|
| Long hop chains | **Yes** | Directed path length in the spend graph. |
| Fan-out | **Yes** | Many children from one tx / many outputs spent separately. |
| Fan-in | **Yes** | Many parents into one tx. |
| Gather-scatter | **Yes** | Hub as a **transaction** (many→one→many). |
| Scatter-gather | **Mostly** | Branch/merge topology; may need path-level or flow summaries. |
| Cycles | **No** (strict DAG) | Use **address/entity** graphs or off-chain identity for **round-trip** loops. |
| Random baseline | **Yes** | Compare metrics to distribution in the same window. |
| Bipartite | **Rarely** (raw) | Needs **labels** or **address/entity** modeling. |
| Stack / layered mesh | **Partially** | Layered **forward** flow yes; **dense mesh** needs careful metrics and baselines. |

---

## Practical takeaway for BlockGraph

The **primary** BlockGraph view uses **transactions** as nodes and **spends** as directed edges. That representation strongly supports **chains**, **fan-in**, **fan-out**, **gather-scatter**, and **scatter-gather**, subject to the caveats above.

You should **not** expect **directed cycles** on raw **Bitcoin** **tx→tx** graphs. Treat **cycles** as **address/entity**-level or **off-chain** ideas unless you change the graph definition.

Always document that **structural** similarity is not the same as **illicit** intent: exchanges, mixers, and ordinary commerce create **similar shapes**.

---

## References (context)

Patterns are often discussed in the context of **AML systems** and **synthetic financial transaction** datasets used to train or benchmark models. Exact citations depend on your course materials. The infographic title is *“Patterns in the graph can yield risk insights.”*

[YouTube](https://www.youtube.com/watch?v=szLUNcUwbVE&t=352s) (relevant segment from about 5:52).
