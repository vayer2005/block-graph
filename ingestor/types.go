package ingestor

import "encoding/json"

// Block is the JSON shape returned by blockchain.info GET /rawblock/{hash}?format=json.
// Transactions are in the top-level "tx" array — downstream graph logic should read
// from Block.Tx (see also FetchedBlock).
type Block struct {
	Hash   string            `json:"hash"`
	Height int               `json:"height"`
	Tx     []json.RawMessage `json:"tx"`
}

// FetchedBlock pairs a height with the parsed block body. Transactions are
// stored in Block.Tx (same as the API’s "tx" field).
type FetchedBlock struct {
	Height int
	Block  Block
}
