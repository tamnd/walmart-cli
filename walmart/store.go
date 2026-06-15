package walmart

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/PuerkitoBio/goquery"
)

// store.go reads the store surfaces. Finding stores near a ZIP runs through the
// Affiliate API, the documented way to enumerate them; without credentials there
// is no anonymous store-locator endpoint, so it reports the wall. A single store
// page (/store/<id>) is walled too, so `store show` is best-effort web with no
// API equivalent.

// FindStores returns the stores near a ZIP code.
func (c *Client) FindStores(ctx context.Context, zip string, limit int) ([]*Store, error) {
	if c.api != nil {
		return c.api.Stores(ctx, zip, limit)
	}
	return nil, ErrBlocked
}

// GetStore returns one store's profile by id (or /store/ URL), read from the
// page when Walmart serves it.
func (c *Client) GetStore(ctx context.Context, ref string) (*Store, error) {
	id := storeID(ref)
	if id == "" {
		return nil, ErrNotFound
	}
	body, err := c.get(ctx, c.BaseURL+"/store/"+id)
	if err != nil {
		return nil, err
	}
	s := parseStore(body, id)
	if s == nil {
		return nil, ErrBlocked
	}
	return s, nil
}

// parseStore reads a store's fields from the page's __NEXT_DATA__ island, or nil
// when the island carries no store (a wall stub, or an unexpected layout).
func parseStore(body []byte, id string) *Store {
	nd, ok := extractNextData(body)
	if ok {
		if s := storeFromIsland(nd, id); s != nil {
			return s
		}
	}
	// Fall back to the social-card meta for a name, so a served-but-unparsed page
	// still yields something addressable.
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	name := metaContent(doc, "og:title")
	if name == "" {
		return nil
	}
	return &Store{ID: id, Name: name, URL: BaseURL + "/store/" + id}
}

// storeFromIsland decodes the store object Walmart nests under the page island.
func storeFromIsland(nd []byte, id string) *Store {
	var root struct {
		Props struct {
			PageProps struct {
				InitialData struct {
					Data struct {
						Store *struct {
							ID          json.Number `json:"id"`
							DisplayName string      `json:"displayName"`
							Name        string      `json:"name"`
							Address     *struct {
								Address     string `json:"address"`
								AddressLine string `json:"addressLineOne"`
								City        string `json:"city"`
								State       string `json:"state"`
								PostalCode  string `json:"postalCode"`
							} `json:"address"`
							Phone       string `json:"phone"`
							PhoneNumber string `json:"phoneNumber"`
							Geo         *struct {
								Latitude  float64 `json:"latitude"`
								Longitude float64 `json:"longitude"`
							} `json:"geoPoint"`
						} `json:"store"`
					} `json:"data"`
				} `json:"initialData"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if json.Unmarshal(nd, &root) != nil {
		return nil
	}
	st := root.Props.PageProps.InitialData.Data.Store
	if st == nil {
		return nil
	}
	sid := st.ID.String()
	if sid == "" {
		sid = id
	}
	s := &Store{ID: sid, URL: BaseURL + "/store/" + sid}
	s.Name = firstNonEmpty(st.DisplayName, st.Name)
	s.Phone = firstNonEmpty(st.Phone, st.PhoneNumber)
	if a := st.Address; a != nil {
		s.Address = firstNonEmpty(a.AddressLine, a.Address)
		s.City = a.City
		s.State = a.State
		s.Zip = a.PostalCode
	}
	if g := st.Geo; g != nil {
		s.Lat = g.Latitude
		s.Lon = g.Longitude
	}
	if s.Name == "" {
		return nil
	}
	return s
}
