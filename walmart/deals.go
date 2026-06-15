package walmart

import "context"

// deals.go reads the rollback and clearance items, and the trending grid. Both
// the deals hub (/shop/deals) and the trending shelf are walled from datacenter
// IPs, and neither has a __NEXT_DATA__ grid that survives the wall, so these run
// through the Affiliate API. Without credentials they report the wall.

// Deals returns the current rollbacks.
func (c *Client) Deals(ctx context.Context, limit int) ([]*Deal, error) {
	if c.api != nil {
		return c.api.Deals(ctx, limit)
	}
	// Best-effort web read of the deals hub for the rare network that is served.
	body, err := c.get(ctx, c.BaseURL+"/shop/deals")
	if err != nil {
		return nil, err
	}
	nd, ok := extractNextData(body)
	if !ok {
		return nil, ErrBlocked
	}
	listings := parseListings(nd, limit)
	if len(listings) == 0 {
		return nil, ErrBlocked
	}
	return dealsFromListings(listings), nil
}

// Trending returns the trending items.
func (c *Client) Trending(ctx context.Context, limit int) ([]*Listing, error) {
	if c.api != nil {
		return c.api.Trending(ctx, limit)
	}
	return nil, ErrBlocked
}

// dealsFromListings turns the grid listings the deals hub renders into Deal
// records, carrying the markdown the listing already knows about.
func dealsFromListings(listings []*Listing) []*Deal {
	out := make([]*Deal, 0, len(listings))
	for _, l := range listings {
		d := &Deal{
			ID:       l.ID,
			Title:    l.Title,
			Brand:    l.Brand,
			Price:    l.Price,
			Was:      l.Was,
			Currency: l.Currency,
			Rating:   l.Rating,
			Reviews:  l.Reviews,
			Image:    l.Thumbnail,
			URL:      l.URL,
		}
		if l.Was > l.Price {
			d.Save = l.Was - l.Price
		}
		out = append(out, d)
	}
	return out
}
