# BlockGraph: how I’m structuring ingestion and the merge

This note is the shape of the core pipeline. I pull recent Bitcoin blocks, turn them into a transaction graph inside a fixed window, then run the pattern logic on that graph. The spine is parallel ingestion, worker subroutines that each build a piece of the graph, and pairwise merges in rounds so the depth of the combine step is logarithmic in the number of workers rather than linear.

###  Block ingestion
Ingestion is the outer loop. I fix a window, usually the last twenty-four hours of confirmed blocks once I know the chain tip and can define a height range, and I need the full block bodies for every height in that range. I use Go and hit my node or a public API with several workers in parallel, each responsible for a sub-range of heights or for pulling from a shared queue of heights still missing. The point is to overlap network latency, saturating I/O while the window is still downloading.

###  Block Cache 
I keep a Postgres-backed record of which blocks I have already ingested: height, hash, and the raw payload or a pointer to it. Before a worker goes to the wire it checks that store. If the block is there it skips the fetch and reads from disk. That way retries, overlapping runs, and crashes do not re-download the same work, and I have one place to answer what we have for this analysis.

### Subgraph Building
Subroutines are how I split the graph build. After blocks are available I partition the transactions or the block heights across n workers, however many goroutines or processes I configure. Each worker runs the same routine. For every transaction it owns I walk its inputs; for each input I resolve the funding txid; \s. The subgraph that worker produces is really just its multiset of edges and implicitly the nodes that touch those edges.

### Subgraph Merging

Merging is uses log(n) based subgraph merge structure. I start with n subgraphs, one per worker. Each merge step takes two bundles and returns one: the union of edges, with deduplication on from_txid and to_txid so the same logical edge never appears twice if I ever overlap work by mistake. I schedule merges in rounds. After each round the number of bundles drops by about half until a single bundle remains. The number of merge rounds is on the order of log n in the number of initial workers. The merge operation itself is cheap relative to parsing blocks. It is mostly set union over edge records in memory or streamed through Postgres if memory gets tight.

### Output
What comes out is one consolidated edge list for the run: the full directed graph over transactions in the window. Downstream I run the BlockGraph metrics and extractions on that graph, degrees for fan-in and fan-out, longest path style passes in topological order for chains and hub motifs, top K subgraphs for the UI, without caring how many merge rounds built the edge list, only that it is complete and deduplicated.

That is the whole structure I am aiming for: blockchain ingestion as the entry, subroutines that each emit a disjoint edge set, log depth pairwise merge to one graph, then pattern algorithms on top.
