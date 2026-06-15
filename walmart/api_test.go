package walmart

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
)

// testKeyPEM generates a throwaway RSA key in the PKCS#8 PEM encoding the
// Affiliate API expects, so the signing path runs against a real key.
func testKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

// itemsJSON is the search/paginated item shape the API returns.
const itemsJSON = `{"items":[
  {"itemId":12345,"name":"Apple iPhone 13","salePrice":499.99,"msrp":599,"brandName":"Apple",
   "stock":"Available","numReviews":10,"customerRating":"4.5","upc":"194252707357",
   "categoryPath":"Electronics/Cell Phones","productUrl":"https://www.walmart.com/ip/12345",
   "thumbnailImage":"https://i5/t.jpg","largeImage":"https://i5/l.jpg"}
]}`

func TestParsePrivateKeyPKCS1(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	p := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}))
	got, err := parsePrivateKey(p)
	if err != nil {
		t.Fatalf("parsePrivateKey(PKCS1): %v", err)
	}
	if got == nil {
		t.Fatal("parsePrivateKey returned a nil key")
	}
}

func TestParsePrivateKeyRejectsGarbage(t *testing.T) {
	if _, err := parsePrivateKey("not a pem block"); err == nil {
		t.Error("parsePrivateKey of non-PEM input should error")
	}
}

func TestAPISign(t *testing.T) {
	a := newAPIClient(Config{ConsumerID: "consumer-1", KeyVersion: "1", PrivateKey: testKeyPEM(t)})
	if a.keyErr != nil {
		t.Fatalf("key did not load: %v", a.keyErr)
	}
	sig, err := a.sign("1700000000000")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		t.Errorf("signature is not valid base64: %v", err)
	}
	if len(raw) == 0 {
		t.Error("signature is empty")
	}
}

func TestAPISearchItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Every signed request carries the four WM_* headers.
		for _, h := range []string{"WM_CONSUMER.ID", "WM_CONSUMER.INTIMESTAMP", "WM_SEC.KEY_VERSION", "WM_SEC.AUTH_SIGNATURE"} {
			if r.Header.Get(h) == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		_, _ = w.Write([]byte(itemsJSON))
	}))
	defer ts.Close()

	a := newAPIClient(Config{ConsumerID: "consumer-1", KeyVersion: "1", PrivateKey: testKeyPEM(t)})
	a.base = ts.URL

	got, err := a.SearchItems(context.Background(), "iphone", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 listing, got %d", len(got))
	}
	l := got[0]
	if l.ID != "12345" || l.Title != "Apple iPhone 13" {
		t.Errorf("listing = %+v", l)
	}
	if l.Price != 499.99 || l.Currency != "USD" {
		t.Errorf("price = %v %q", l.Price, l.Currency)
	}
	if l.Was != 599 {
		t.Errorf("msrp above sale should map to was, got %v", l.Was)
	}
	if l.Availability != "Available" || l.Rating != 4.5 {
		t.Errorf("availability/rating = %q / %v", l.Availability, l.Rating)
	}
}

func TestAPIGetItem(t *testing.T) {
	const itemJSON = `{"itemId":12345,"name":"Apple iPhone 13","salePrice":499.99,"msrp":599,
	  "brandName":"Apple","modelNumber":"MLPF3LL/A","upc":"194252707357","stock":"Available",
	  "categoryPath":"Electronics/Cell Phones","shortDescription":"<p>A great phone.</p>",
	  "largeImage":"https://i5/l.jpg","numReviews":10,"customerRating":4.5}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(itemJSON))
	}))
	defer ts.Close()

	a := newAPIClient(Config{ConsumerID: "c", KeyVersion: "1", PrivateKey: testKeyPEM(t)})
	a.base = ts.URL

	p, err := a.GetItem(context.Background(), "12345")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "12345" || p.Model != "MLPF3LL/A" {
		t.Errorf("product = %+v", p)
	}
	if p.Price != 499.99 || p.Was != 599 || p.List != 599 {
		t.Errorf("price/was/list = %v / %v / %v", p.Price, p.Was, p.List)
	}
	if p.Category != "Cell Phones" {
		t.Errorf("category leaf = %q", p.Category)
	}
	if p.Description != "A great phone." {
		t.Errorf("description = %q", p.Description)
	}
}

func TestAPIBadCredentialsAreBlocked(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	a := newAPIClient(Config{ConsumerID: "c", KeyVersion: "1", PrivateKey: testKeyPEM(t)})
	a.base = ts.URL

	if _, err := a.SearchItems(context.Background(), "iphone", 5); err != ErrBlocked {
		t.Errorf("401 should read as ErrBlocked, got %v", err)
	}
}

// TestSearchFallsBackToAPI proves the walled web search routes to the Affiliate
// backend when credentials are configured: the page host returns the bot wall,
// the API host returns results.
func TestSearchFallsBackToAPI(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(itemsJSON))
	}))
	defer api.Close()
	wall := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer wall.Close()

	c := NewClient(Config{
		BaseURL:    wall.URL,
		ConsumerID: "c",
		KeyVersion: "1",
		PrivateKey: testKeyPEM(t),
		Delay:      0,
		Retries:    0,
		NoCache:    true,
	})
	if c.api == nil {
		t.Fatal("credentials should have built the API backend")
	}
	c.api.base = api.URL

	got, err := c.Search(context.Background(), "iphone", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "12345" {
		t.Fatalf("fallback results = %+v", got)
	}
}

// TestStoresFallback confirms the store locator returns API records when
// credentials are set.
func TestStoresFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"no":2280,"name":"Mountain View Supercenter","streetAddress":"600 Showers Dr",
		  "city":"Mountain View","stateProvCode":"CA","zip":"94040","phoneNumber":"650-555-1212",
		  "lat":37.39,"lon":-122.09,"distance":1.2}]`))
	}))
	defer ts.Close()

	c := NewClient(Config{
		ConsumerID: "c",
		KeyVersion: "1",
		PrivateKey: testKeyPEM(t),
		NoCache:    true,
	})
	c.api.base = ts.URL

	got, err := c.FindStores(context.Background(), "94040", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 store, got %d", len(got))
	}
	s := got[0]
	if s.ID != "2280" || s.City != "Mountain View" || s.State != "CA" {
		t.Errorf("store = %+v", s)
	}
	if s.Distance != 1.2 || s.Zip != "94040" {
		t.Errorf("distance/zip = %v / %q", s.Distance, s.Zip)
	}
}
