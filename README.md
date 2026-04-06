# Graph Ingestion

Ingestion pipeline and graph-building experiments for pattern related graph data.

## Sample run

Log(n) merging of 15 bitcoin blocks is intended.
But blocks are now being merged in a chain manner.
TODO: Fix this

### Merge chain

| Step | Action | Edges |
|------|--------|------:|
| — | Subgraph `1` | 3,405 |
| — | Subgraph `2` | 4,177 |
| 1 | Merge `1` + `2` → `3` | 7,582 |
| — | Subgraph `4` | 6,920 |
| 2 | Merge `3` + `4` → `5` | 14,502 |
| — | Subgraph `6` | 7,269 |
| 3 | Merge `5` + `6` → `7` | 21,771 |
| — | Subgraph `8` | 7,142 |
| 4 | Merge `7` + `8` → `9` | 28,913 |
| — | Subgraph `10` | 8,075 |
| 5 | Merge `9` + `10` → `11` | 36,988 |
| — | Subgraph `12` | 7,358 |
| 6 | Merge `11` + `12` → `13` | 44,346 |
| — | Subgraph `14` | 6,553 |
| 7 | Merge `13` + `14` → `15` | 50,899 |
| — | Subgraph `16` | 6,324 |
| 8 | Merge `15` + `16` → `17` | 57,223 |
| — | Subgraph `18` | 8,013 |
| 9 | Merge `17` + `18` → `19` | 65,236 |
| — | Subgraph `20` | 6,795 |
| 10 | Merge `19` + `20` → `21` | 72,031 |
| — | Subgraph `22` | 9,030 |
| 11 | Merge `21` + `22` → `23` | 81,061 |
| — | Subgraph `24` | 7,261 |
| 12 | Merge `23` + `24` → `25` | 88,322 |
| — | Subgraph `26` | 6,334 |
| 13 | Merge `25` + `26` → `27` | 94,656 |
| — | Subgraph `28` | 7,157 |
| 14 | Merge `27` + `28` → `29` | 101,813 |

### Result

- **Final graph:** id `29`, **101,813** edges  
- **Blocks fetched:** 15  
- **Wall time:** ~8.47s  

Raw log lines:

```text
merged graph id=29 edges=101813
fetched 15 blocks
Time taken: 8.468718292s
```
