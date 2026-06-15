package walmart_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tamnd/walmart-cli/walmart"
)

// newClient points a client at a fake server with no pacing or retries and no
// affiliate credentials, so the walled surfaces report the bot wall rather than
// falling back. The API fallback path is covered in api_test.go.
func newClient(ts *httptest.Server) *walmart.Client {
	cfg := walmart.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Delay = 0
	cfg.Retries = 0
	cfg.NoCache = true
	return walmart.NewClient(cfg)
}

// serve returns a server that replies with body for every request.
func serve(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
}

// status returns a server that replies with a bare status code for every request.
func status(code int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	}))
}

// productPage mirrors a real /ip/ page: the item under
// props.pageProps.initialData.data.product, with the structured currentPrice,
// the strikethrough wasPrice and the manufacturer listPrice, a category trail, a
// short description in HTML, and a two-image gallery.
const productPage = `<html><head>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"initialData":{"data":{
  "product":{"usItemId":"123456789012","name":"Apple iPhone 13 128GB","brand":"Apple","model":"MLPF3LL/A","upc":"194252707357",
    "availabilityStatus":"IN_STOCK","sellerDisplayName":"Walmart.com","averageRating":4.5,"numberOfReviews":1234,
    "shortDescription":"<p>A great phone.</p>",
    "priceInfo":{"currentPrice":{"price":549.99,"currencyUnit":"USD"},"wasPrice":{"price":699},"listPrice":{"price":799}},
    "imageInfo":{"thumbnailUrl":"https://i5.walmartimages.com/a.jpg","allImages":[{"url":"https://i5.walmartimages.com/a.jpg"},{"url":"https://i5.walmartimages.com/b.jpg"}]},
    "category":{"path":[{"name":"Electronics","url":"/cp/electronics/3944"},{"name":"Cell Phones","url":"/cp/cell-phones/1105910"}]},
    "variantsMap":{"v1":{"usItemId":"123456789012"},"v2":{"usItemId":"999000111222"},"v3":{"usItemId":"888000111222"}}},
  "reviews":{"averageOverallRating":4.5,"totalReviewCount":1234}
}}}}}
</script></head><body>iPhone</body></html>`

func TestGetProduct(t *testing.T) {
	ts := serve(productPage)
	defer ts.Close()

	p, err := newClient(ts).GetProduct(context.Background(), "123456789012")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "123456789012" || p.Name != "Apple iPhone 13 128GB" {
		t.Errorf("product = %+v", p)
	}
	if p.Brand != "Apple" || p.Model != "MLPF3LL/A" || p.UPC != "194252707357" {
		t.Errorf("brand/model/upc = %q / %q / %q", p.Brand, p.Model, p.UPC)
	}
	if p.Price != 549.99 || p.Currency != "USD" {
		t.Errorf("price = %v %q", p.Price, p.Currency)
	}
	if p.Was != 699 || p.List != 799 {
		t.Errorf("was/list = %v / %v", p.Was, p.List)
	}
	if p.Availability != "IN_STOCK" || p.Seller != "Walmart.com" {
		t.Errorf("availability/seller = %q / %q", p.Availability, p.Seller)
	}
	if p.Rating != 4.5 || p.Reviews != 1234 {
		t.Errorf("rating/reviews = %v / %d", p.Rating, p.Reviews)
	}
	if p.Category != "Cell Phones" {
		t.Errorf("category leaf = %q", p.Category)
	}
	// The leaf url carries the category id, so the product links to its category
	// for BFS; the trail keeps the human path.
	if p.CategoryID != "1105910" {
		t.Errorf("category id (edge) = %q", p.CategoryID)
	}
	if len(p.Trail) != 2 || p.Trail[0] != "Electronics" || p.Trail[1] != "Cell Phones" {
		t.Errorf("trail = %v", p.Trail)
	}
	// The variantsMap names sibling item ids; self is dropped and the rest sorted,
	// so the product links to its other variants.
	if len(p.Variants) != 2 || p.Variants[0] != "888000111222" || p.Variants[1] != "999000111222" {
		t.Errorf("variants (edges) = %v", p.Variants)
	}
	if p.Description != "A great phone." {
		t.Errorf("description = %q", p.Description)
	}
	if len(p.Images) != 2 || p.Image != "https://i5.walmartimages.com/a.jpg" {
		t.Errorf("image gallery = %v (image %q)", p.Images, p.Image)
	}
	if p.URL != "https://www.walmart.com/ip/123456789012" {
		t.Errorf("url = %q", p.URL)
	}
}

// searchPage mirrors a real /search grid: an organic Product tile with a
// canonical URL and the object form of availabilityStatusV2, an ad tile flagged
// by its __typename and priced by a bare linePrice number, and a layout cell with
// no item id that is dropped.
const searchPage = `<html><head>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"initialData":{"searchResult":{"itemStacks":[{"items":[
  {"__typename":"Product","usItemId":"111111111","name":"Item One","brand":"BrandA","canonicalUrl":"/ip/Item-One/111111111",
   "availabilityStatusV2":{"display":"In stock","value":"IN_STOCK"},"averageRating":4.2,"numberOfReviews":10,"sellerName":"Walmart.com",
   "priceInfo":{"currentPrice":{"price":12.99,"currencyUnit":"USD"},"wasPrice":{"price":19.99}},
   "imageInfo":{"thumbnailUrl":"https://i5/t1.jpg"}},
  {"__typename":"Ad","usItemId":"222222222","name":"Sponsored Item","priceInfo":{"linePrice":8},"imageInfo":{"thumbnailUrl":"https://i5/t2.jpg"}},
  {"__typename":"Tile"}
]}]}}}}}
</script></head><body>results</body></html>`

func TestSearch(t *testing.T) {
	ts := serve(searchPage)
	defer ts.Close()

	got, err := newClient(ts).Search(context.Background(), "iphone", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 listings (the layout cell dropped), got %d", len(got))
	}
	a := got[0]
	if a.ID != "111111111" || a.Title != "Item One" || a.Brand != "BrandA" {
		t.Errorf("listing 0 = %+v", a)
	}
	// Every listing links to its full product for BFS, so the grid item id is also
	// its product edge.
	if a.Item != a.ID {
		t.Errorf("listing item edge = %q, want %q", a.Item, a.ID)
	}
	if a.Price != 12.99 || a.Was != 19.99 || a.Currency != "USD" {
		t.Errorf("price = %v was %v %q", a.Price, a.Was, a.Currency)
	}
	if a.Availability != "In stock" {
		t.Errorf("availability (object form) = %q", a.Availability)
	}
	if a.Sponsored {
		t.Error("organic Product tile should not be sponsored")
	}
	if a.URL != "https://www.walmart.com/ip/Item-One/111111111" {
		t.Errorf("url from canonicalUrl = %q", a.URL)
	}
	b := got[1]
	if b.ID != "222222222" || !b.Sponsored {
		t.Errorf("ad tile = %+v", b)
	}
	if b.Price != 8 {
		t.Errorf("linePrice number = %v", b.Price)
	}
}

func TestSuggest(t *testing.T) {
	ts := serve(`{"queries":[{"displayName":"iphone 13","imageUrl":"https://i5/s.jpg"},{"displayName":"iphone case"}]}`)
	defer ts.Close()

	c := newClient(ts)
	c.SuggestURL = ts.URL // the typeahead host, redirected to the fake

	got, err := c.Suggest(context.Background(), "iphone", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 suggestions, got %d", len(got))
	}
	if got[0].Term != "iphone 13" || got[0].Query != "iphone" || got[0].Image == "" {
		t.Errorf("suggestion 0 = %+v", got[0])
	}
	if got[1].Term != "iphone case" {
		t.Errorf("suggestion 1 = %+v", got[1])
	}
}

// homePage mirrors the department links Walmart serves anonymously on the
// homepage: two distinct /cp/ links and a repeat that the scan dedupes.
const homePage = `<html><body>
<a href="/cp/electronics/3944">Electronics</a>
<a href="/cp/grocery/976759">Grocery</a>
<a href="/cp/electronics/3944">Electronics again</a>
</body></html>`

func TestCategoryTree(t *testing.T) {
	ts := serve(homePage)
	defer ts.Close()

	got, err := newClient(ts).CategoryTree(context.Background(), "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 deduped categories, got %d", len(got))
	}
	if got[0].ID != "3944" || got[0].Name != "Electronics" {
		t.Errorf("category 0 = %+v", got[0])
	}
	if got[1].ID != "976759" || got[1].Name != "Grocery" {
		t.Errorf("category 1 = %+v", got[1])
	}
}

func TestSearchWallForbidden(t *testing.T) {
	ts := status(http.StatusForbidden)
	defer ts.Close()

	_, err := newClient(ts).Search(context.Background(), "iphone", 10)
	if !errors.Is(err, walmart.ErrBlocked) {
		t.Errorf("403 without credentials should be ErrBlocked, got %v", err)
	}
}

func TestSearchWallPreconditionFailed(t *testing.T) {
	ts := status(http.StatusPreconditionFailed)
	defer ts.Close()

	_, err := newClient(ts).Search(context.Background(), "iphone", 10)
	if !errors.Is(err, walmart.ErrBlocked) {
		t.Errorf("412 without credentials should be ErrBlocked, got %v", err)
	}
}

func TestProductInterstitialIsBlocked(t *testing.T) {
	// PerimeterX serves the challenge with a 200 status as often as a 403, so the
	// body is what gives it away.
	ts := serve(`<html><head><title>Robot or human?</title></head><body>px-captcha</body></html>`)
	defer ts.Close()

	_, err := newClient(ts).GetProduct(context.Background(), "123456789012")
	if !errors.Is(err, walmart.ErrBlocked) {
		t.Errorf("interstitial body should be ErrBlocked, got %v", err)
	}
}

func TestProductNotFound(t *testing.T) {
	ts := status(http.StatusNotFound)
	defer ts.Close()

	_, err := newClient(ts).GetProduct(context.Background(), "123456789012")
	if !errors.Is(err, walmart.ErrNotFound) {
		t.Errorf("404 should be ErrNotFound, got %v", err)
	}
}

func TestRateLimited(t *testing.T) {
	ts := status(http.StatusTooManyRequests)
	defer ts.Close()

	_, err := newClient(ts).Search(context.Background(), "iphone", 10)
	if !errors.Is(err, walmart.ErrRateLimited) {
		t.Errorf("429 should be ErrRateLimited, got %v", err)
	}
}

// TestFindStoresWithoutCredentialsIsBlocked confirms the store locator, which has
// no anonymous endpoint, reports the wall rather than guessing when no affiliate
// credentials are set.
func TestFindStoresWithoutCredentialsIsBlocked(t *testing.T) {
	ts := serve("unused")
	defer ts.Close()

	_, err := newClient(ts).FindStores(context.Background(), "94043", 5)
	if !errors.Is(err, walmart.ErrBlocked) {
		t.Errorf("store find without credentials should be ErrBlocked, got %v", err)
	}
}
