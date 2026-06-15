package walmart

import (
	"context"
	"errors"
	"net/url"
)

// search.go reads keyword search. The /search page is walled from datacenter IPs
// by PerimeterX, so this is best-effort: the client GETs the page and reads its
// __NEXT_DATA__ island, and when the bot wall is up and Affiliate API
// credentials are set it falls back to the documented API for the same records.
// With no credentials the wall propagates as exit 4.

// Search returns listings matching a keyword query.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]*Listing, error) {
	u := c.BaseURL + "/search?q=" + url.QueryEscape(query)
	body, err := c.get(ctx, u)
	if err != nil {
		if errors.Is(err, ErrBlocked) && c.api != nil {
			return c.api.SearchItems(ctx, query, limit)
		}
		return nil, err
	}
	nd, ok := extractNextData(body)
	if !ok {
		if c.api != nil {
			return c.api.SearchItems(ctx, query, limit)
		}
		return nil, ErrBlocked
	}
	listings := parseListings(nd, limit)
	if len(listings) == 0 && c.api != nil {
		return c.api.SearchItems(ctx, query, limit)
	}
	return listings, nil
}
