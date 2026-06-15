package walmart

import (
	"errors"
	"testing"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring (mint, resolve), which need no network. The client's HTTP
// behaviour is covered in walmart_test.go and api_test.go.

func TestClassify(t *testing.T) {
	cases := []struct {
		in   string
		kind string
		id   string
		url  string
	}{
		{"123456789012", "product", "123456789012", "https://www.walmart.com/ip/123456789012"},
		{"5037034321", "product", "5037034321", "https://www.walmart.com/ip/5037034321"},
		{"https://www.walmart.com/ip/Apple-iPhone-13/123456789012", "product", "123456789012", "https://www.walmart.com/ip/123456789012"},
		{"/ip/5037034321", "product", "5037034321", "https://www.walmart.com/ip/5037034321"},
		{"https://www.walmart.com/store/2280", "store", "2280", "https://www.walmart.com/store/2280"},
		{"/store/Mountain-View/2280", "store", "2280", "https://www.walmart.com/store/2280"},
		{"https://www.walmart.com/cp/electronics/3944", "category", "3944", "https://www.walmart.com/cp/3944"},
		{"https://www.walmart.com/browse/cell-phones/1105910", "category", "1105910", "https://www.walmart.com/cp/1105910"},
		{"https://www.walmart.com/browse/electronics?cat_id=976759", "category", "976759", "https://www.walmart.com/cp/976759"},
		{"", "unknown", "", ""},
		{"12345", "unknown", "", ""}, // too short for an item id
		{"https://www.walmart.com/help/center", "unknown", "", ""},
	}
	for _, c := range cases {
		got := Classify(c.in)
		if got.Kind != c.kind || got.ID != c.id || got.URL != c.url {
			t.Errorf("Classify(%q) = {%q %q %q}, want {%q %q %q}",
				c.in, got.Kind, got.ID, got.URL, c.kind, c.id, c.url)
		}
	}
}

// TestURLForRoundTrip checks that re-classifying a built URL recovers the same
// (kind, id) for the kinds that have a canonical URL.
func TestURLForRoundTrip(t *testing.T) {
	cases := []struct{ kind, id string }{
		{"product", "123456789012"},
		{"store", "2280"},
		{"category", "3944"},
	}
	for _, c := range cases {
		u := URLFor(c.kind, c.id)
		if u == "" {
			t.Errorf("URLFor(%q,%q) empty", c.kind, c.id)
			continue
		}
		got := Classify(u)
		if got.Kind != c.kind || got.ID != c.id {
			t.Errorf("round-trip %q: Classify(%q) = {%q %q}, want {%q %q}",
				c.kind, u, got.Kind, got.ID, c.kind, c.id)
		}
	}
}

func TestURLForUnknown(t *testing.T) {
	if u := URLFor("nonsense", "x"); u != "" {
		t.Errorf("URLFor of an unknown kind should be empty, got %q", u)
	}
}

func TestMapErr(t *testing.T) {
	cases := []struct {
		err  error
		kind errs.Kind
	}{
		{ErrNotFound, errs.KindNotFound},
		{ErrRateLimited, errs.KindRateLimited},
		{ErrBlocked, errs.KindNeedAuth},
	}
	for _, c := range cases {
		got := mapErr(c.err)
		if errs.KindOf(got) != c.kind {
			t.Errorf("mapErr(%v) kind = %v, want %v", c.err, errs.KindOf(got), c.kind)
		}
	}
	if mapErr(nil) != nil {
		t.Error("mapErr(nil) should be nil")
	}
	plain := errors.New("boom")
	if got := mapErr(plain); got != plain {
		t.Errorf("mapErr passes an unmapped error through unchanged, got %v", got)
	}
}

func TestDomainClassify(t *testing.T) {
	d := Domain{}
	kind, id, err := d.Classify("https://www.walmart.com/ip/Apple-iPhone-13/123456789012")
	if err != nil {
		t.Fatal(err)
	}
	if kind != "product" || id != "123456789012" {
		t.Errorf("Domain.Classify = {%q %q}", kind, id)
	}
	if _, _, err := d.Classify("https://www.walmart.com/help/center"); err == nil {
		t.Error("Domain.Classify of an unknown reference should error")
	}
}

func TestDomainLocate(t *testing.T) {
	d := Domain{}
	u, err := d.Locate("category", "3944")
	if err != nil {
		t.Fatal(err)
	}
	if u != "https://www.walmart.com/cp/3944" {
		t.Errorf("Locate = %q", u)
	}
	if _, err := d.Locate("nonsense", "x"); err == nil {
		t.Error("Locate of an unknown kind should error")
	}
}

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "walmart" {
		t.Errorf("scheme = %q", info.Scheme)
	}
	if info.Identity.Binary != "walmart" {
		t.Errorf("identity binary = %q", info.Identity.Binary)
	}
	want := map[string]bool{"www.walmart.com": true, "walmart.com": true}
	for _, h := range info.Hosts {
		delete(want, h)
	}
	if len(want) != 0 {
		t.Errorf("missing hosts: %v", want)
	}
}

// TestHostWiring mounts the driver in a kit Host (the runtime ant drives) and
// checks the round trip: a product record mints to its URI, and a bare id
// resolves back to the same URI. The init in domain.go registers the domain, so
// kit.Open finds it.
func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	p := &Product{ID: "123456789012", URL: BaseURL + "/ip/123456789012", Name: "Apple iPhone 13"}
	u, err := h.Mint(p)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if want := "walmart://product/123456789012"; u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}

	got, err := h.ResolveOn("walmart", "123456789012")
	if err != nil || got.String() != "walmart://product/123456789012" {
		t.Errorf("ResolveOn = (%q, %v), want walmart://product/123456789012", got.String(), err)
	}
}
