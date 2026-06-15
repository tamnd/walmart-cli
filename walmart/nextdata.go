package walmart

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
)

// nextdata.go reads the __NEXT_DATA__ JSON island Walmart embeds in every page.
// It is a single <script id="__NEXT_DATA__" type="application/json"> block that
// carries the same data the page renders from: on a product page the item under
// props.pageProps.initialData.data.product, and on a search or category page the
// grid under props.pageProps.initialData.searchResult.itemStacks[].items[]. The
// shapes here are deliberately loose, decoding only the fields the records need,
// because Walmart carries far more in the island than this tool reports.

// extractNextData returns the JSON bytes of the __NEXT_DATA__ island, or false
// when the page has none (a wall stub, or an unexpected layout).
func extractNextData(body []byte) ([]byte, bool) {
	marker := []byte(`id="__NEXT_DATA__"`)
	i := bytes.Index(body, marker)
	if i < 0 {
		return nil, false
	}
	// The JSON starts at the first '>' after the marker and ends at the next
	// </script>.
	start := bytes.IndexByte(body[i:], '>')
	if start < 0 {
		return nil, false
	}
	start += i + 1
	end := bytes.Index(body[start:], []byte("</script>"))
	if end < 0 {
		return nil, false
	}
	return body[start : start+end], true
}

// --- island shapes ---

type ndRoot struct {
	Props struct {
		PageProps struct {
			InitialData struct {
				Data struct {
					Product *ndProduct `json:"product"`
					Reviews *struct {
						AverageOverallRating float64 `json:"averageOverallRating"`
						TotalReviewCount     int     `json:"totalReviewCount"`
					} `json:"reviews"`
					Idml *struct {
						LongDescription string `json:"longDescription"`
					} `json:"idml"`
				} `json:"data"`
				SearchResult *struct {
					ItemStacks []struct {
						Items []ndGridItem `json:"items"`
					} `json:"itemStacks"`
				} `json:"searchResult"`
			} `json:"initialData"`
		} `json:"pageProps"`
	} `json:"props"`
}

type ndProduct struct {
	UsItemID         string       `json:"usItemId"`
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Brand            string       `json:"brand"`
	ManufacturerName string       `json:"manufacturerName"`
	Model            string       `json:"model"`
	UPC              string       `json:"upc"`
	ShortDescription string       `json:"shortDescription"`
	Availability     string       `json:"availabilityStatus"`
	SellerName       string       `json:"sellerName"`
	SellerDisplay    string       `json:"sellerDisplayName"`
	AverageRating    float64      `json:"averageRating"`
	NumberOfReviews  int          `json:"numberOfReviews"`
	PriceInfo        *ndPriceInfo `json:"priceInfo"`
	ImageInfo        *ndImageInfo `json:"imageInfo"`
	Category         *struct {
		Path []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"path"`
	} `json:"category"`
	// variantsMap keys each colour/size/configuration sibling by a variant id;
	// each value carries the sibling's own usItemId, the edge a host follows.
	VariantsMap map[string]struct {
		UsItemID string `json:"usItemId"`
	} `json:"variantsMap"`
}

type ndGridItem struct {
	Typename         string          `json:"__typename"`
	UsItemID         string          `json:"usItemId"`
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Brand            string          `json:"brand"`
	CanonicalURL     string          `json:"canonicalUrl"`
	AvailabilityV2   json.RawMessage `json:"availabilityStatusV2"`
	AverageRating    float64         `json:"averageRating"`
	NumberOfReviews  int             `json:"numberOfReviews"`
	SellerName       string          `json:"sellerName"`
	ImageInfo        *ndImageInfo    `json:"imageInfo"`
	PriceInfo        *ndPriceInfo    `json:"priceInfo"`
	SponsoredProduct json.RawMessage `json:"sponsoredProduct"`
}

type ndImageInfo struct {
	ThumbnailURL string `json:"thumbnailUrl"`
	AllImages    []struct {
		URL string `json:"url"`
	} `json:"allImages"`
}

type ndPriceInfo struct {
	CurrentPrice     *ndPrice        `json:"currentPrice"`
	WasPrice         *ndPrice        `json:"wasPrice"`
	ListPrice        *ndPrice        `json:"listPrice"`
	LinePrice        json.RawMessage `json:"linePrice"` // a number on the grid
	LinePriceDisplay string          `json:"linePriceDisplay"`
}

type ndPrice struct {
	Price        float64 `json:"price"`
	CurrencyUnit string  `json:"currencyUnit"`
	PriceString  string  `json:"priceString"`
}

// price returns the current price and currency from a price block, preferring
// the structured currentPrice and falling back to the grid's linePrice number or
// its display string.
func (p *ndPriceInfo) price() (float64, string) {
	if p == nil {
		return 0, ""
	}
	cur := "USD"
	if p.CurrentPrice != nil {
		if p.CurrentPrice.CurrencyUnit != "" {
			cur = p.CurrentPrice.CurrencyUnit
		}
		if p.CurrentPrice.Price > 0 {
			return p.CurrentPrice.Price, cur
		}
	}
	if len(p.LinePrice) > 0 {
		var v float64
		if json.Unmarshal(p.LinePrice, &v) == nil && v > 0 {
			return v, cur
		}
	}
	if p.LinePriceDisplay != "" {
		if v := priceFromDisplay(p.LinePriceDisplay); v > 0 {
			return v, cur
		}
	}
	return 0, cur
}

// was returns the strikethrough price, when the item is marked down.
func (p *ndPriceInfo) was() float64 {
	if p == nil || p.WasPrice == nil {
		return 0
	}
	return p.WasPrice.Price
}

// list returns the manufacturer list price, when present.
func (p *ndPriceInfo) listPrice() float64 {
	if p == nil || p.ListPrice == nil {
		return 0
	}
	return p.ListPrice.Price
}

// ndAvailability reads availabilityStatusV2, which is a string on some pages and
// an object {display, value} on others.
func ndAvailability(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var obj struct {
		Display string `json:"display"`
		Value   string `json:"value"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		if obj.Display != "" {
			return obj.Display
		}
		return obj.Value
	}
	return ""
}

// parseProduct maps the product island to a Product, or nil when the island
// carries no product.
func parseProduct(nd []byte, id string) *Product {
	var root ndRoot
	if json.Unmarshal(nd, &root) != nil {
		return nil
	}
	pp := root.Props.PageProps.InitialData.Data.Product
	if pp == nil {
		return nil
	}
	pid := pp.UsItemID
	if pid == "" {
		pid = id
	}
	p := &Product{ID: pid, URL: BaseURL + "/ip/" + pid}
	p.Name = pp.Name
	p.Brand = pp.Brand
	if p.Brand == "" {
		p.Brand = pp.ManufacturerName
	}
	p.Model = pp.Model
	p.UPC = pp.UPC
	p.Price, p.Currency = pp.PriceInfo.price()
	p.Was = pp.PriceInfo.was()
	p.List = pp.PriceInfo.listPrice()
	p.Availability = pp.Availability
	p.Seller = pp.SellerDisplay
	if p.Seller == "" {
		p.Seller = pp.SellerName
	}
	p.Rating = pp.AverageRating
	p.Reviews = pp.NumberOfReviews
	if rv := root.Props.PageProps.InitialData.Data.Reviews; rv != nil {
		if p.Rating == 0 {
			p.Rating = rv.AverageOverallRating
		}
		if p.Reviews == 0 {
			p.Reviews = rv.TotalReviewCount
		}
	}
	if pp.Category != nil && len(pp.Category.Path) > 0 {
		path := pp.Category.Path
		for _, node := range path {
			if node.Name != "" {
				p.Trail = append(p.Trail, node.Name)
			}
		}
		leaf := path[len(path)-1]
		p.Category = leaf.Name
		p.CategoryID = catIDFromURL(leaf.URL)
	}
	seenVar := map[string]bool{}
	for _, v := range pp.VariantsMap {
		if v.UsItemID == "" || v.UsItemID == p.ID || seenVar[v.UsItemID] {
			continue
		}
		seenVar[v.UsItemID] = true
		p.Variants = append(p.Variants, v.UsItemID)
	}
	// variantsMap iterates in random order; sort so the edge list is stable.
	sort.Strings(p.Variants)
	p.Description = stripHTML(pp.ShortDescription)
	if p.Description == "" {
		if idml := root.Props.PageProps.InitialData.Data.Idml; idml != nil {
			p.Description = stripHTML(idml.LongDescription)
		}
	}
	if pp.ImageInfo != nil {
		p.Image = pp.ImageInfo.ThumbnailURL
		seen := map[string]bool{}
		for _, img := range pp.ImageInfo.AllImages {
			if img.URL == "" || seen[img.URL] {
				continue
			}
			seen[img.URL] = true
			p.Images = append(p.Images, img.URL)
		}
		if p.Image == "" && len(p.Images) > 0 {
			p.Image = p.Images[0]
		}
	}
	return p
}

// parseListings maps the search/category grid island to Listing records, capped
// to limit (limit <= 0 means all). It keeps organic Product tiles and the ad
// tiles Walmart marks, flagging the latter as sponsored, and dedupes by id.
func parseListings(nd []byte, limit int) []*Listing {
	var root ndRoot
	if json.Unmarshal(nd, &root) != nil {
		return nil
	}
	sr := root.Props.PageProps.InitialData.SearchResult
	if sr == nil {
		return nil
	}
	var out []*Listing
	seen := map[string]bool{}
	for _, stack := range sr.ItemStacks {
		for _, it := range stack.Items {
			l := it.toListing()
			if l == nil || seen[l.ID] {
				continue
			}
			seen[l.ID] = true
			out = append(out, l)
			if limit > 0 && len(out) >= limit {
				return out
			}
		}
	}
	return out
}

// toListing maps one grid item, or nil when the tile carries no item id (a
// banner or a layout cell).
func (it ndGridItem) toListing() *Listing {
	id := it.UsItemID
	if id == "" {
		id = it.ID
	}
	if id == "" || !isDigits(id) {
		return nil
	}
	l := &Listing{ID: id, Item: id, URL: BaseURL + "/ip/" + id}
	if it.CanonicalURL != "" {
		l.URL = BaseURL + it.CanonicalURL
	}
	l.Title = it.Name
	l.Brand = it.Brand
	l.Price, l.Currency = it.PriceInfo.price()
	l.Was = it.PriceInfo.was()
	l.Seller = it.SellerName
	l.Rating = it.AverageRating
	l.Reviews = it.NumberOfReviews
	l.Availability = ndAvailability(it.AvailabilityV2)
	l.Sponsored = it.Typename != "Product" || len(it.SponsoredProduct) > 0
	if it.ImageInfo != nil {
		l.Thumbnail = it.ImageInfo.ThumbnailURL
	}
	return l
}

// jsonNumString renders a json.Number-or-number-or-string id field as a string.
func jsonNumString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	return ""
}

// rawFloat reads a JSON value that may be a number or a quoted number.
func rawFloat(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return f
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		v, _ := strconv.ParseFloat(s, 64)
		return v
	}
	return 0
}
