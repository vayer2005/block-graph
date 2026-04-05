package ingestor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// See https://www.blockchain.com/explorer/api/blockchain_api
const DefaultAPIBase = "https://blockchain.info"

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient() *Client {
	return &Client{
		BaseURL: strings.TrimRight(DefaultAPIBase, "/"),
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) base() string {
	if c.BaseURL == "" {
		return strings.TrimRight(DefaultAPIBase, "/")
	}
	return strings.TrimRight(c.BaseURL, "/")
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) getBytes(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s: %s", path, resp.Status, strings.TrimSpace(string(b)))
	}
	return b, nil
}

// TipHeight returns the current chain tip block height (from GET /latestblock).
func (c *Client) TipHeight(ctx context.Context) (int, error) {
	body, err := c.getBytes(ctx, "/latestblock")
	if err != nil {
		return 0, err
	}
	var out struct {
		Height int `json:"height"`
	}
	err = json.Unmarshal(body, &out);
	if err != nil {
		return 0, err
	}
	return out.Height, nil
}

// BlockHashAtHeight resolves the block hash for a given height (GET /block-height/{height}?format=json).
func (c *Client) BlockHashAtHeight(ctx context.Context, height int) (string, error) {
	path := fmt.Sprintf("/block-height/%d?format=json", height)
	body, err := c.getBytes(ctx, path)
	if err != nil {
		return "", err
	}
	var out struct {
		Blocks []struct {
			Hash string `json:"hash"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if len(out.Blocks) == 0 {
		return "", fmt.Errorf("no block at height %d", height)
	}
	return out.Blocks[0].Hash, nil
}

// BlockByHash fetches the full block including the "tx" array (GET /rawblock/{hash}).
func (c *Client) BlockByHash(ctx context.Context, hash string) (Block, error) {
	path := fmt.Sprintf("/rawblock/%s?format=json", hash)
	body, err := c.getBytes(ctx, path)
	if err != nil {
		return Block{}, err
	}
	var blk Block
	if err := json.Unmarshal(body, &blk); err != nil {
		return Block{}, err
	}
	return blk, nil
}
