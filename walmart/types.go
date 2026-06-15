package walmart

// This file holds the exported records the commands emit. Their json tags name
// the fields a reader sees, kit:"id" marks the key the record store upserts on,
// and table:",truncate" keeps wide free text from blowing up a terminal table.
// Each record carries only fields a logged-out reader can actually fill: no
// cart, no order history, no personalized prices, no in-store inventory tied to
// a signed-in account. There is no Rank column either; emit order is the rank,
// and a stable id is a better store key than a position that shifts on every
// refresh. Several records are emitted by more than one surface, and not every
// surface fills every field; omitempty carries the gaps. The per-surface files
// (search.go, product.go, ...) hold the parsing these map from.

// Listing is a summary row in a grid, emitted by search, category browse, and
// trending. Its id is the product's usItemId, so Item carries the same value as
// the graph edge to the full product record: a host walks a search result or a
// category page straight through to walmart://product/<id> and onward.
type Listing struct {
	ID           string  `json:"id" kit:"id"`
	Title        string  `json:"title,omitempty" table:",truncate"`
	Brand        string  `json:"brand,omitempty"`
	Price        float64 `json:"price,omitempty"`
	Currency     string  `json:"currency,omitempty"`
	Was          float64 `json:"was,omitempty"` // strikethrough / was price
	Seller       string  `json:"seller,omitempty"`
	Rating       float64 `json:"rating,omitempty"`  // average star rating
	Reviews      int     `json:"reviews,omitempty"` // review count
	Availability string  `json:"availability,omitempty"`
	Sponsored    bool    `json:"sponsored,omitempty"` // an ad tile rather than an organic result
	Thumbnail    string  `json:"thumbnail,omitempty" table:",truncate"`
	URL          string  `json:"url"`
	Item         string  `json:"item,omitempty" table:"-" kit:"link,kind=walmart/product"` // edge to the full product
}

// Product is the full detail for one item, emitted by product. The fields are
// what the /ip page shows a logged-out visitor, the same fields the Affiliate
// API returns on the fallback path.
type Product struct {
	ID           string   `json:"id" kit:"id"`
	Name         string   `json:"name,omitempty" table:",truncate"`
	Brand        string   `json:"brand,omitempty"`
	Model        string   `json:"model,omitempty"`
	UPC          string   `json:"upc,omitempty"`
	Price        float64  `json:"price,omitempty"`
	Currency     string   `json:"currency,omitempty"`
	Was          float64  `json:"was,omitempty"`  // was price, when the item is marked down
	List         float64  `json:"list,omitempty"` // manufacturer list price / MSRP
	Availability string   `json:"availability,omitempty"`
	Seller       string   `json:"seller,omitempty"`
	Rating       float64  `json:"rating,omitempty"`
	Reviews      int      `json:"reviews,omitempty"`
	Category     string   `json:"category,omitempty"`                                               // the leaf department name
	CategoryID   string   `json:"category_id,omitempty" table:"-" kit:"link,kind=walmart/category"` // edge to the leaf category
	Trail        []string `json:"trail,omitempty" table:"-"`                                        // the full department path, root first, leaf last
	Description  string   `json:"description,omitempty" table:",truncate"`
	Image        string   `json:"image,omitempty" table:",truncate"`
	Images       []string `json:"images,omitempty" table:"-"`                                   // the photo gallery, when more than one is listed
	Variants     []string `json:"variants,omitempty" table:"-" kit:"link,kind=walmart/product"` // edges to colour/size/configuration siblings
	URL          string   `json:"url"`
}

// Store is a Walmart store's public profile, emitted by store show and store
// find.
type Store struct {
	ID       string  `json:"id" kit:"id"` // store number
	Name     string  `json:"name,omitempty"`
	Address  string  `json:"address,omitempty"`
	City     string  `json:"city,omitempty"`
	State    string  `json:"state,omitempty"`
	Zip      string  `json:"zip,omitempty"`
	Phone    string  `json:"phone,omitempty"`
	Lat      float64 `json:"lat,omitempty"`
	Lon      float64 `json:"lon,omitempty"`
	Distance float64 `json:"distance,omitempty"` // miles from the queried ZIP, on store find
	Hours    string  `json:"hours,omitempty"`
	URL      string  `json:"url"`
}

// Category is a category node, emitted by category show and category tree. The
// ParentID and Children edges make the taxonomy walkable in both directions, so
// a host can recurse the department tree from any node to reconstruct it.
type Category struct {
	ID       string   `json:"id" kit:"id"`
	Name     string   `json:"name"`
	Parent   string   `json:"parent,omitempty"`                                               // parent category name (from the trail)
	ParentID string   `json:"parent_id,omitempty" table:"-" kit:"link,kind=walmart/category"` // edge up to the parent node
	Children []string `json:"children,omitempty" table:"-" kit:"link,kind=walmart/category"`  // edges down to the child nodes
	Trail    []string `json:"trail,omitempty" table:"-"`                                      // the full ancestor path, root first, leaf last
	URL      string   `json:"url"`
}

// Deal is a rolled-back or cleared item, emitted by deals.
type Deal struct {
	ID       string  `json:"id" kit:"id"`
	Title    string  `json:"title,omitempty" table:",truncate"`
	Brand    string  `json:"brand,omitempty"`
	Price    float64 `json:"price,omitempty"`
	Was      float64 `json:"was,omitempty"`  // price before the markdown
	Save     float64 `json:"save,omitempty"` // was minus price
	Currency string  `json:"currency,omitempty"`
	Offer    string  `json:"offer,omitempty"` // the offer type, e.g. "Rollback" or "Clearance"
	Rating   float64 `json:"rating,omitempty"`
	Reviews  int     `json:"reviews,omitempty"`
	Image    string  `json:"image,omitempty" table:",truncate"`
	URL      string  `json:"url"`
	Item     string  `json:"item,omitempty" table:"-" kit:"link,kind=walmart/product"` // edge to the full product
}

// Suggestion is one autocomplete term, emitted by suggest.
type Suggestion struct {
	Query string `json:"query"`           // the prefix that was queried
	Term  string `json:"term" kit:"id"`   // a suggested completion
	Image string `json:"image,omitempty"` // a thumbnail the typeahead pairs with some terms
}

// Ref is the result of `walmart ref id`: the canonical (kind, id) a reference
// resolves to, plus the live URL, all without touching the network.
type Ref struct {
	Input string `json:"input"`
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	URL   string `json:"url"`
}
