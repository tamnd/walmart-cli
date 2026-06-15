package walmart

import (
	"context"
	"encoding/json"
	"net/url"
)

// suggest.go reads the typeahead host, the most reliable surface in the tool: a
// small JSON host the search box calls as you type, not behind the bot wall. The
// reply is an object with a queries array, each carrying a display name and
// sometimes a thumbnail.

// Suggest returns search-autocomplete terms for a typed prefix.
func (c *Client) Suggest(ctx context.Context, prefix string, limit int) ([]*Suggestion, error) {
	base := c.SuggestURL
	if base == "" {
		base = typeaheadURL
	}
	u := base + "?term=" + url.QueryEscape(prefix) + "&cat=0&prg=desktop"
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Queries []struct {
			DisplayName string `json:"displayName"`
			ImageURL    string `json:"imageUrl"`
		} `json:"queries"`
	}
	if err := json.Unmarshal(body, &resp); err == nil && len(resp.Queries) > 0 {
		var out []*Suggestion
		for _, q := range resp.Queries {
			if q.DisplayName == "" {
				continue
			}
			out = append(out, &Suggestion{Query: prefix, Term: q.DisplayName, Image: q.ImageURL})
			if limit > 0 && len(out) >= limit {
				break
			}
		}
		return out, nil
	}

	// Some variants return a flat array of terms; fall back to that shape.
	var terms []string
	if err := json.Unmarshal(body, &terms); err != nil {
		return nil, nil
	}
	var out []*Suggestion
	for _, t := range terms {
		if t == "" {
			continue
		}
		out = append(out, &Suggestion{Query: prefix, Term: t})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}
