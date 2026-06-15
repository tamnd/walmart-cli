package walmart

import (
	"bytes"
	"context"
	"errors"

	"github.com/PuerkitoBio/goquery"
)

// category.go reads the category pages. /cp/ and /browse are walled from
// datacenter IPs, so the item grid is best-effort with the same API fallback as
// search. The category tree is gentler: its top level can be read from the
// homepage, which Walmart serves anonymously, by scanning the department links.

// CategoryBrowse returns the items in a category.
func (c *Client) CategoryBrowse(ctx context.Context, ref string, limit int) ([]*Listing, error) {
	id := categoryID(ref)
	if id == "" {
		return nil, ErrNotFound
	}
	body, err := c.get(ctx, c.BaseURL+"/cp/"+id)
	if err != nil {
		if errors.Is(err, ErrBlocked) && c.api != nil {
			return c.api.CategoryItems(ctx, id, limit)
		}
		return nil, err
	}
	nd, ok := extractNextData(body)
	if ok {
		if listings := parseListings(nd, limit); len(listings) > 0 {
			return listings, nil
		}
	}
	if c.api != nil {
		return c.api.CategoryItems(ctx, id, limit)
	}
	return nil, ErrBlocked
}

// GetCategory returns a category's metadata: its name and its trail. It reads
// the API taxonomy when configured, since the web page is walled.
func (c *Client) GetCategory(ctx context.Context, ref string) (*Category, error) {
	id := categoryID(ref)
	if id == "" {
		return nil, ErrNotFound
	}
	if c.api != nil {
		return c.api.GetCategory(ctx, id)
	}
	// Best-effort web read: the breadcrumb in the page meta, when the page is not
	// walled.
	body, err := c.get(ctx, c.BaseURL+"/cp/"+id)
	if err != nil {
		return nil, err
	}
	doc, derr := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if derr != nil {
		return nil, derr
	}
	cat := &Category{ID: id, URL: BaseURL + "/cp/" + id}
	cat.Name = squish(firstText(doc.Selection, "h1"))
	if cat.Name == "" {
		cat.Name = metaContent(doc, "og:title")
	}
	if cat.Name == "" {
		return nil, ErrBlocked
	}
	return cat, nil
}

// CategoryTree returns the child categories of a node. With API credentials it
// reads the taxonomy; without them it scans the department links on the homepage
// (for the top level) or the category page, which works when that page is served.
func (c *Client) CategoryTree(ctx context.Context, ref string, limit int) ([]*Category, error) {
	id := categoryID(ref)
	if c.api != nil {
		cats, err := c.api.Taxonomy(ctx, id, limit)
		if err == nil && len(cats) > 0 {
			return cats, nil
		}
		if err != nil && !errors.Is(err, ErrBlocked) {
			return nil, err
		}
	}
	page := c.BaseURL + "/"
	if id != "" {
		page = c.BaseURL + "/cp/" + id
	}
	body, err := c.get(ctx, page)
	if err != nil {
		return nil, err
	}
	doc, derr := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if derr != nil {
		return nil, derr
	}
	return parseCategoryLinks(doc, limit), nil
}

// parseCategoryLinks scans the /cp/ links on a page into Category stubs, deduped
// by id, in document order.
func parseCategoryLinks(doc *goquery.Document, limit int) []*Category {
	var out []*Category
	seen := map[string]bool{}
	doc.Find(`a[href*="/cp/"]`).EachWithBreak(func(_ int, a *goquery.Selection) bool {
		href, _ := a.Attr("href")
		id := categoryIDFrom(href)
		if id == "" || seen[id] {
			return true
		}
		name := squish(a.Text())
		if name == "" {
			return true
		}
		seen[id] = true
		out = append(out, &Category{ID: id, Name: name, URL: BaseURL + "/cp/" + id})
		return limit <= 0 || len(out) < limit
	})
	return out
}

// categoryIDFrom extracts the numeric category id from a /cp/ href.
func categoryIDFrom(href string) string {
	segs := splitSegs(refPath(href))
	if len(segs) == 0 || segs[0] != "cp" {
		return ""
	}
	return lastDigits(segs[1:])
}
