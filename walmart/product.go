package walmart

import (
	"context"
	"errors"
)

// product.go reads one item by id. The /ip/ page is walled from datacenter IPs
// by PerimeterX, so this is best-effort: the client GETs the page and reads its
// __NEXT_DATA__ island, and when the bot wall is up (ErrBlocked) and Affiliate
// API credentials are set, falls back to the documented API for the same record.
// With no credentials the wall propagates as exit 4 with a message that names
// both remedies.

// GetProduct returns one item by id (or /ip/ URL).
func (c *Client) GetProduct(ctx context.Context, ref string) (*Product, error) {
	id := productID(ref)
	if id == "" {
		return nil, ErrNotFound
	}
	body, err := c.get(ctx, c.BaseURL+"/ip/"+id)
	if err != nil {
		if errors.Is(err, ErrBlocked) && c.api != nil {
			return c.api.GetItem(ctx, id)
		}
		return nil, err
	}
	nd, ok := extractNextData(body)
	if ok {
		if p := parseProduct(nd, id); p != nil {
			return p, nil
		}
	}
	// No island, or an island without a product: treat it as the wall and fall
	// back to the API when configured.
	if c.api != nil {
		return c.api.GetItem(ctx, id)
	}
	return nil, ErrBlocked
}
