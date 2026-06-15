package walmart

import "time"

// Host is the Walmart hostname this client builds page URLs from and the host
// the URI driver in domain.go claims.
const Host = "www.walmart.com"

// BaseURL is the root every product, store, and category URL is built from.
const BaseURL = "https://" + Host

// typeaheadURL is the autocomplete host the search box calls as you type. It is
// a small JSON endpoint, not behind the bot wall, so `walmart suggest` is the
// most reliable command in the tool.
const typeaheadURL = BaseURL + "/typeahead/v2/complete"

// The opt-in Affiliate API. Walmart walls the product page, search, the category
// pages, and the store pages from datacenter IPs, so when the affiliate
// credentials are set the walled commands fall back to this documented backend
// instead of failing. Unlike eBay's Browse API, it signs each request with an
// RSA key rather than exchanging an OAuth token.
const (
	apiBase           = "https://developer.api.walmart.com/api-proxy/service/affil/product/v2"
	defaultKeyVersion = "1"
)

// DefaultUserAgent is sent with every page request. Walmart serves its public
// pages to a normal browser; a browser User-Agent is what keeps a logged-out
// reader looking like one. Override it with --user-agent.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// Defaults for the polite client.
const (
	// DefaultDelay is the minimum gap between requests. Walmart is touchy, so a
	// one-second pace reads steadily without leaning on it.
	DefaultDelay    = 1 * time.Second
	DefaultRetries  = 3
	DefaultTimeout  = 30 * time.Second
	DefaultCacheTTL = 24 * time.Hour

	// defaultPageSize is the items asked for per listing page. 40 is the size the
	// web grids use.
	defaultPageSize = 40
)

// Config carries the knobs the client reads. It is built from the kit framework
// config in ClientFromConfig, so a --rate or --timeout on the command line and
// the same value resolved by a host both land here.
type Config struct {
	UserAgent string

	// Zip is the ZIP context for the store locator and the default-store price on
	// the API path. Empty leaves it to Walmart's default store.
	Zip string

	// The affiliate credentials enable the API fallback for the walled surfaces.
	// ConsumerID with one of PrivateKey or PrivateKeyFile turns it on; empty
	// leaves the fallback off, and the walled commands then report the bot wall
	// rather than guessing.
	ConsumerID     string
	PrivateKey     string // PEM contents
	PrivateKeyFile string // path to a PEM file, used when PrivateKey is empty
	KeyVersion     string
	PublisherID    string

	// Delay is the minimum gap between requests. Zero means no pacing.
	Delay   time.Duration
	Retries int
	Timeout time.Duration

	// BaseURL is the site root. Empty uses the public site; tests point it at an
	// httptest server.
	BaseURL string

	// CacheDir is where responses are cached. Empty disables the cache, as does
	// NoCache.
	CacheDir string
	CacheTTL time.Duration
	NoCache  bool
	// Refresh fetches fresh copies and rewrites the cache, ignoring any hit.
	Refresh bool
}

// DefaultConfig returns the baseline configuration: a browser User-Agent, a
// one-second pace, three retries, a 30s timeout, the "1" key version, and a
// one-day cache.
func DefaultConfig() Config {
	return Config{
		UserAgent:  DefaultUserAgent,
		KeyVersion: defaultKeyVersion,
		Delay:      DefaultDelay,
		Retries:    DefaultRetries,
		Timeout:    DefaultTimeout,
		BaseURL:    BaseURL,
		CacheTTL:   DefaultCacheTTL,
	}
}
