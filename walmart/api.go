package walmart

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// api.go is the opt-in Affiliate API backend. Walmart publishes a REST product
// API that returns item, search, taxonomy, deals, trends, and store data as
// clean JSON. It is not anonymous in the keyless sense: it needs a free
// developer account's consumer id and RSA private key. Unlike eBay's Browse API,
// there is no token to exchange; each request is signed in place with the key,
// so the signature is computed per call from the consumer id, a millisecond
// timestamp, and the key version. This CLI never requires it, but the product
// page, search, the category pages, and the store pages are exactly the surfaces
// Walmart walls from datacenter networks, so the walled commands fall back here
// when the operator has set WALMART_CONSUMER_ID and a key.

type apiClient struct {
	HTTP        *http.Client
	ConsumerID  string
	KeyVersion  string
	PublisherID string

	key    *rsa.PrivateKey
	keyErr error

	// base is the API root. It defaults to the public host; tests point it at an
	// httptest server.
	base string
}

func newAPIClient(cfg Config) *apiClient {
	a := &apiClient{
		HTTP:        &http.Client{Timeout: cfg.Timeout},
		ConsumerID:  cfg.ConsumerID,
		KeyVersion:  cfg.KeyVersion,
		PublisherID: cfg.PublisherID,
		base:        apiBase,
	}
	if a.KeyVersion == "" {
		a.KeyVersion = defaultKeyVersion
	}
	pemData := cfg.PrivateKey
	if pemData == "" && cfg.PrivateKeyFile != "" {
		b, err := os.ReadFile(cfg.PrivateKeyFile)
		if err != nil {
			a.keyErr = err
			return a
		}
		pemData = string(b)
	}
	a.key, a.keyErr = parsePrivateKey(pemData)
	return a
}

// parsePrivateKey decodes a PEM RSA key, accepting both the PKCS#8 and PKCS#1
// encodings a developer might paste.
func parsePrivateKey(pemData string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, errors.New("walmart: private key is not valid PEM")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rk, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("walmart: private key is not an RSA key")
		}
		return rk, nil
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// sign builds the per-request signature. The signed message is the consumer id,
// the timestamp in epoch milliseconds, and the key version, each followed by a
// newline; the signature is the base64 of the RSA-SHA256 over that message.
func (a *apiClient) sign(timestampMillis string) (string, error) {
	if a.keyErr != nil {
		return "", a.keyErr
	}
	msg := a.ConsumerID + "\n" + timestampMillis + "\n" + a.KeyVersion + "\n"
	sum := sha256.Sum256([]byte(msg))
	sig, err := rsa.SignPKCS1v15(rand.Reader, a.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// getJSON runs one signed GET against the API and decodes it.
func (a *apiClient) getJSON(ctx context.Context, path string, params url.Values, out any) error {
	if a.PublisherID != "" {
		if params == nil {
			params = url.Values{}
		}
		params.Set("publisherId", a.PublisherID)
	}
	u := a.base + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	sig, err := a.sign(ts)
	if err != nil {
		// A bad or missing key reads as the same wall: the operator's opt-in did
		// not take, so the surface stays blocked.
		return ErrBlocked
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("WM_CONSUMER.ID", a.ConsumerID)
	req.Header.Set("WM_CONSUMER.INTIMESTAMP", ts)
	req.Header.Set("WM_SEC.KEY_VERSION", a.KeyVersion)
	req.Header.Set("WM_SEC.AUTH_SIGNATURE", sig)
	req.Header.Set("Accept", "application/json")

	resp, err := a.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	switch resp.StatusCode {
	case http.StatusOK:
		return json.Unmarshal(b, out)
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusTooManyRequests:
		return ErrRateLimited
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrBlocked
	default:
		return fmt.Errorf("affiliate api: http %d", resp.StatusCode)
	}
}

// --- API wire shapes ---

// apiItem is the item shape the items, search, trends, and paginated endpoints
// all return.
type apiItem struct {
	ItemID           json.RawMessage `json:"itemId"`
	Name             string          `json:"name"`
	SalePrice        float64         `json:"salePrice"`
	Msrp             float64         `json:"msrp"`
	UPC              string          `json:"upc"`
	BrandName        string          `json:"brandName"`
	ModelNumber      string          `json:"modelNumber"`
	CategoryPath     string          `json:"categoryPath"`
	ShortDescription string          `json:"shortDescription"`
	LongDescription  string          `json:"longDescription"`
	ThumbnailImage   string          `json:"thumbnailImage"`
	MediumImage      string          `json:"mediumImage"`
	LargeImage       string          `json:"largeImage"`
	ProductURL       string          `json:"productUrl"`
	Stock            string          `json:"stock"`
	AvailableOnline  bool            `json:"availableOnline"`
	CustomerRating   json.RawMessage `json:"customerRating"`
	NumReviews       int             `json:"numReviews"`
	SellerInfo       string          `json:"sellerInfo"`
	ImageEntities    []struct {
		LargeImage     string `json:"largeImage"`
		ThumbnailImage string `json:"thumbnailImage"`
	} `json:"imageEntities"`
}

func (it apiItem) id() string       { return jsonNumString(it.ItemID) }
func (it apiItem) currency() string { return "USD" }

func (it apiItem) availability() string {
	if it.Stock != "" {
		return it.Stock
	}
	if it.AvailableOnline {
		return "In stock"
	}
	return "Out of stock"
}

func (it apiItem) url() string {
	if it.ProductURL != "" {
		return it.ProductURL
	}
	if id := it.id(); id != "" {
		return BaseURL + "/ip/" + id
	}
	return ""
}

func (it apiItem) toListing() *Listing {
	l := &Listing{
		ID:           it.id(),
		Title:        it.Name,
		Brand:        it.BrandName,
		Price:        it.SalePrice,
		Currency:     it.currency(),
		Seller:       it.SellerInfo,
		Rating:       rawFloat(it.CustomerRating),
		Reviews:      it.NumReviews,
		Availability: it.availability(),
		Thumbnail:    it.ThumbnailImage,
		URL:          it.url(),
	}
	if it.Msrp > it.SalePrice {
		l.Was = it.Msrp
	}
	return l
}

func (it apiItem) toProduct() *Product {
	p := &Product{
		ID:           it.id(),
		Name:         it.Name,
		Brand:        it.BrandName,
		Model:        it.ModelNumber,
		UPC:          it.UPC,
		Price:        it.SalePrice,
		Currency:     it.currency(),
		List:         it.Msrp,
		Availability: it.availability(),
		Seller:       it.SellerInfo,
		Rating:       rawFloat(it.CustomerRating),
		Reviews:      it.NumReviews,
		Category:     lastPath(it.CategoryPath),
		Description:  stripHTML(firstNonEmpty(it.ShortDescription, it.LongDescription)),
		URL:          it.url(),
	}
	if it.Msrp > it.SalePrice {
		p.Was = it.Msrp
	}
	p.Image = firstNonEmpty(it.LargeImage, it.MediumImage, it.ThumbnailImage)
	seen := map[string]bool{}
	for _, img := range append([]string{p.Image}, imageEntityURLs(it)...) {
		if img == "" || seen[img] {
			continue
		}
		seen[img] = true
		p.Images = append(p.Images, img)
	}
	return p
}

func (it apiItem) toDeal() *Deal {
	d := &Deal{
		ID:       it.id(),
		Title:    it.Name,
		Brand:    it.BrandName,
		Price:    it.SalePrice,
		Currency: it.currency(),
		Rating:   rawFloat(it.CustomerRating),
		Reviews:  it.NumReviews,
		Image:    firstNonEmpty(it.LargeImage, it.MediumImage, it.ThumbnailImage),
		URL:      it.url(),
		Offer:    "Rollback",
	}
	if it.Msrp > it.SalePrice {
		d.Was = it.Msrp
		d.Save = it.Msrp - it.SalePrice
	}
	return d
}

func imageEntityURLs(it apiItem) []string {
	var out []string
	for _, e := range it.ImageEntities {
		if e.LargeImage != "" {
			out = append(out, e.LargeImage)
		}
	}
	return out
}

// GetItem fetches one item by id and maps it to a Product.
func (a *apiClient) GetItem(ctx context.Context, id string) (*Product, error) {
	var it apiItem
	if err := a.getJSON(ctx, "/items/"+url.PathEscape(id), nil, &it); err != nil {
		return nil, err
	}
	if it.id() == "" {
		return nil, ErrNotFound
	}
	return it.toProduct(), nil
}

// SearchItems runs a keyword search and maps the results to Listing records.
func (a *apiClient) SearchItems(ctx context.Context, query string, limit int) ([]*Listing, error) {
	if limit <= 0 || limit > 100 {
		limit = defaultPageSize
	}
	params := url.Values{"query": {query}, "numItems": {strconv.Itoa(limit)}}
	var resp struct {
		Items []apiItem `json:"items"`
	}
	if err := a.getJSON(ctx, "/search", params, &resp); err != nil {
		return nil, err
	}
	return listingsFrom(resp.Items, limit), nil
}

// CategoryItems lists the items in a category.
func (a *apiClient) CategoryItems(ctx context.Context, catID string, limit int) ([]*Listing, error) {
	params := url.Values{"category": {catID}}
	var resp struct {
		Items []apiItem `json:"items"`
	}
	if err := a.getJSON(ctx, "/paginated/items", params, &resp); err != nil {
		return nil, err
	}
	return listingsFrom(resp.Items, limit), nil
}

// Deals lists the current rollbacks.
func (a *apiClient) Deals(ctx context.Context, limit int) ([]*Deal, error) {
	params := url.Values{"specialOffer": {"rollback"}}
	var resp struct {
		Items []apiItem `json:"items"`
	}
	if err := a.getJSON(ctx, "/paginated/items", params, &resp); err != nil {
		return nil, err
	}
	var out []*Deal
	for _, it := range resp.Items {
		if it.id() == "" {
			continue
		}
		out = append(out, it.toDeal())
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// Trending lists the trending items.
func (a *apiClient) Trending(ctx context.Context, limit int) ([]*Listing, error) {
	var resp struct {
		Items []apiItem `json:"items"`
	}
	if err := a.getJSON(ctx, "/trends", nil, &resp); err != nil {
		return nil, err
	}
	return listingsFrom(resp.Items, limit), nil
}

// apiCategory is the taxonomy node shape.
type apiCategory struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Path     string        `json:"path"`
	Children []apiCategory `json:"children"`
}

// Taxonomy returns a category node's children: the top-level departments when id
// is empty, or the children of the matching node otherwise.
func (a *apiClient) Taxonomy(ctx context.Context, id string, limit int) ([]*Category, error) {
	var resp struct {
		Categories []apiCategory `json:"categories"`
	}
	if err := a.getJSON(ctx, "/taxonomy", nil, &resp); err != nil {
		return nil, err
	}
	nodes := resp.Categories
	parent := ""
	if id != "" {
		n := findCategory(resp.Categories, id)
		if n == nil {
			return nil, ErrNotFound
		}
		nodes, parent = n.Children, n.Name
	}
	var out []*Category
	for _, n := range nodes {
		out = append(out, &Category{
			ID:     n.ID,
			Name:   n.Name,
			Parent: parent,
			URL:    BaseURL + "/cp/" + n.ID,
		})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// GetCategory returns one category node's metadata by id.
func (a *apiClient) GetCategory(ctx context.Context, id string) (*Category, error) {
	var resp struct {
		Categories []apiCategory `json:"categories"`
	}
	if err := a.getJSON(ctx, "/taxonomy", nil, &resp); err != nil {
		return nil, err
	}
	n := findCategory(resp.Categories, id)
	if n == nil {
		return nil, ErrNotFound
	}
	cat := &Category{ID: n.ID, Name: n.Name, URL: BaseURL + "/cp/" + n.ID}
	if n.Path != "" {
		cat.Trail = strings.Split(n.Path, "/")
		if len(cat.Trail) > 1 {
			cat.Parent = cat.Trail[len(cat.Trail)-2]
		}
	}
	return cat, nil
}

func findCategory(nodes []apiCategory, id string) *apiCategory {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
		if c := findCategory(nodes[i].Children, id); c != nil {
			return c
		}
	}
	return nil
}

// apiStore is the store shape the stores endpoint returns.
type apiStore struct {
	No            json.RawMessage `json:"no"`
	Name          string          `json:"name"`
	StreetAddress string          `json:"streetAddress"`
	City          string          `json:"city"`
	StateProvCode string          `json:"stateProvCode"`
	Zip           string          `json:"zip"`
	PhoneNumber   string          `json:"phoneNumber"`
	Lat           float64         `json:"lat"`
	Lon           float64         `json:"lon"`
	Distance      float64         `json:"distance"`
}

func (s apiStore) toStore() *Store {
	id := jsonNumString(s.No)
	return &Store{
		ID:       id,
		Name:     s.Name,
		Address:  s.StreetAddress,
		City:     s.City,
		State:    s.StateProvCode,
		Zip:      s.Zip,
		Phone:    s.PhoneNumber,
		Lat:      s.Lat,
		Lon:      s.Lon,
		Distance: s.Distance,
		URL:      BaseURL + "/store/" + id,
	}
}

// Stores lists the stores near a ZIP code.
func (a *apiClient) Stores(ctx context.Context, zip string, limit int) ([]*Store, error) {
	params := url.Values{"zip": {zip}}
	var stores []apiStore
	if err := a.getJSON(ctx, "/stores", params, &stores); err != nil {
		return nil, err
	}
	var out []*Store
	for _, s := range stores {
		out = append(out, s.toStore())
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// listingsFrom maps a slice of API items to Listings, skipping idless entries
// and capping to limit.
func listingsFrom(items []apiItem, limit int) []*Listing {
	var out []*Listing
	for _, it := range items {
		if it.id() == "" {
			continue
		}
		out = append(out, it.toListing())
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// lastPath returns the leaf of an "A/B/C" category path.
func lastPath(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "/")
	return strings.TrimSpace(parts[len(parts)-1])
}

// firstNonEmpty returns the first non-empty string among its arguments.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
