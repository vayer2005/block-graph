# Graph patterns and risk-style interpretation

This document lists common **graph topologies** used in financial forensics and AML-style analysis—often summarized as “patterns in the graph can yield risk insights.” Each pattern is a **structural** signal: useful for triage and explanation, **not** proof of illicit activity on its own.

Sources in the literature often tie these shapes to **Anti-Money Laundering** systems and **synthetic transaction** datasets used to train or evaluate detection models.

---

## 1. Long hop chains

**Shape:** A long directed path—many nodes in sequence, one following another (often drawn as a winding line).

**Interpretation (AML context):** Associated with **layering**: moving funds through many intermediaries to obscure origin. The **length** of the path (hop count) is the main signal.

**Transaction-ID graph (`txid` nodes):** **Yes.** An edge **A → B** means *B spends an output of A*. A long path **A → B → C → …** is exactly a **chain of spends**.  
**Caveat:** Legitimate custody, payments, and wallet behavior also create chains; **length vs. baseline** matters.

---

## 2. Fan-out

**Shape:** One source node with many outgoing edges to distinct recipients.

**Interpretation:** Often discussed alongside **structuring** or **smurfing**: splitting flow into many branches (many destinations or many small outputs).

**Transaction-ID graph:** **Yes**, in two senses:
- **One transaction → many child transactions:** many outputs of **Tx S** are spent by **different** next transactions (high out-degree from **S** in the spend graph).
- **One transaction with many outputs** (value split) even before the next hop—visible as **fan-out** at the **tx** level.

**Caveat:** Batch payouts, mining pool rewards, and change-heavy transactions can look like fan-out.

---

## 3. Fan-in

**Shape:** Many nodes pointing **into** one destination node.

**Interpretation:** **Consolidation**: many sources feeding one sink (gathering funds).

**Transaction-ID graph:** **Yes.** One **Tx T** with **many distinct input transactions** (many parents in the spend graph) is **fan-in** to **T**.

**Caveat:** Exchange deposits and wallet sweeps consolidate many inputs; **high fan-in** is common at service hubs.

---

## 4. Gather–scatter (hourglass / bowtie)

**Shape:** Multiple nodes on the left → one **central hub** → multiple nodes on the right (many-to-one-to-many).

**Interpretation:** **Pass-through** or **mule-like** behavior: aggregate in, then redistribute out (sometimes described as gather-scatter around a hub).

**Transaction-ID graph:** **Yes**, if the **hub is a transaction** (or a small set of txs): many parents → **Tx H** → many children.  
**Caveat:** Payment processors and mixers can mimic **hub** shapes; **context** (amounts, timing, known entities) matters.

---

## 5. Scatter–gather

**Shape:** One or few sources → several **intermediate** nodes → one **final** sink (split, then reconverge).

**Interpretation:** **Layered consolidation**: distance from sources to sink with intermediate hops (sometimes to obfuscate).

**Transaction-ID graph:** **Partially / yes** for the **topology** if you see **branching then merging** in the directed spend graph (e.g. diamond-like or multi-path to one sink). Algorithms may need **path enumeration** or **flow** summaries, not only single-parent trees.

**Caveat:** Normal business flows can also branch and merge.

---

## 6. Cycles (closed loops)

**Shape:** Directed edges that form a **closed loop** (A → B → … → A).

**Interpretation (AML context):** **Round-tripping** or **wash trading**: funds moving in a circuit so the path returns to a familiar origin.

**Transaction-ID graph (Bitcoin UTXO spend graph):** **No — not as a true cycle.**  
Spending is **forward in time**: a transaction only spends **existing** outputs. The graph of **tx → tx** (spend edges) is a **DAG** (no directed cycles). You **cannot** have **A → … → A** in the strict sense.

**Where cycles *can* appear:**
- **Address graph** (or **entity** graph): if **address A** pays **B** in one tx and later **B** pays **A** in another, you can get a **directed cycle** in **address ↔ address** flows (different transactions).
- **Temporal** “cycle” in behavior (same entity, multiple hops) is often modeled with **addresses** or **clusters**, not raw `txid` DAGs.

**Summary:** **Cycles** in the infographic are **natural** for **account/address**-style graphs; **not** for pure **transaction DAG** semantics on Bitcoin.

---

## 7. Random (unstructured)

**Shape:** Sparse edges, no strong motif—no long path, no hub, no obvious fan-in/out.

**Interpretation:** **Baseline** or “typical” activity: used as a **control** or comparison class when building or evaluating detectors.

**Transaction-ID graph:** **Yes** as a **reference distribution**: compare **path length**, **degree**, **hub participation** in a window to **typical** subgraphs.

---

## 8. Bipartite-like structure

**Shape:** Two groups of nodes; edges only **between** groups, not **within** a group (two-sided matching).

**Interpretation:** Sometimes associated with **coordinated** or **two-sided** patterns (e.g. one class of entities interacting only with another class).

**Transaction-ID graph:** **Awkward.** Transactions are **not** naturally partitioned into two “sides” without **extra labels** (counterparty type, cluster, service tag).  
**Better:** **Address–address** or **entity–entity** graphs with **metadata**, or **bipartite** **user ↔ merchant** if you have off-chain labels.

**Summary:** **Possible** with heavy **enrichment**; **not** the default on raw `txid` graphs alone.

---

## 9. Stack (layered mesh)

**Shape:** Multiple **layers** (rows) of nodes; dense flow left-to-right through several tiers; **sandwich** or **stacked** appearance.

**Interpretation:** **Organized layering** at scale: systematic routing through successive tiers (sometimes associated with complex laundering typologies in training materials).

**Transaction-ID graph:** **Partially.** You can see **multi-layer** **forward** flow (tiers of **tx**), but **dense** “mesh” often requires **many parallel paths**—detectable with **layered** **path** or **flow** metrics, **k-core**-style analysis, or **betweenness** on **subgraphs** (can be heavy).  
**Caveat:** Busy exchange or settlement periods also create **dense** graphs—**contrast** with **random** baseline.

---

## Can `transaction_id` nodes detect these patterns?

| Pattern | Works well with `txid` nodes? | Notes |
|--------|------------------------|-------|
| Long hop chains | **Yes** | Directed path length in the spend graph. |
| Fan-out | **Yes** | Many children from one tx / many outputs spent separately. |
| Fan-in | **Yes** | Many parents into one tx. |
| Gather–scatter | **Yes** | Hub as a **transaction** (many→one→many). |
| Scatter–gather | **Mostly** | Branch/merge topology; may need path-level or flow summaries. |
| Cycles | **No** (strict DAG) | Use **address/entity** graphs or off-chain identity for **round-trip** loops. |
| Random baseline | **Yes** | Compare metrics to distribution in the same window. |
| Bipartite | **Rarely** (raw) | Needs **labels** or **address/entity** modeling. |
| Stack / layered mesh | **Partially** | Layered **forward** flow yes; **dense mesh** needs careful metrics and baselines. |

---

## Practical takeaway for BlockGraph

- **Primary graph:** **transactions** as nodes, **spends** as directed edges—strongly supports **chains**, **fan-in**, **fan-out**, **gather–scatter**, and **scatter–gather** (with caveats).
- **Do not** expect **directed cycles** on raw **Bitcoin** **tx→tx** graphs; treat **cycles** as **address/entity**-level or **off-chain** concepts unless you change the graph definition.
- Always document: **structural** ≠ **illicit**; exchanges, mixers, and normal commerce create **similar shapes**.

---

## References (context)

Patterns are often discussed in the context of **AML systems** and **synthetic financial transaction** datasets used to train or benchmark models. Exact citations depend on your course materials; the infographic title is: *“Patterns in the graph can yield risk insights.”*
