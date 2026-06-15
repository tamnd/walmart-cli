package walmart

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes walmart as a kit Domain: a driver that a multi-domain host
// (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/walmart-cli/walmart"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then dereferences
// walmart:// URIs by routing to the operations Register installs. The same Domain
// also builds the standalone walmart binary (see cli.NewApp), so the binary and a
// host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the walmart driver. It carries no state; the per-run client is built
// by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against, and
// the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme:   "walmart",
		Hosts:    []string{Host, "walmart.com"},
		Identity: Identity(),
	}
}

// Identity is the fixed description of the walmart CLI, shared by the domain and
// the standalone composition root so help and version read the same everywhere.
func Identity() kit.Identity {
	return kit.Identity{
		Binary: "walmart",
		Short:  "Read public Walmart products, search, categories, stores, and deals into structured records",
		Long: `walmart reads public Walmart data the way a logged-out browser does:
keyword search, a single product, a category and the items in it, the
store locator, the current rollbacks, and search autocomplete. Walmart
fronts its site with a bot manager that walls almost everything from
datacenter IPs, so autocomplete is the only surface that reads from any
network; the product page, search, the category pages, and the store
pages are best-effort and fall back to the Affiliate API when
WALMART_CONSUMER_ID and a private key are set. There is no API key
needed for autocomplete, no login, and nothing to run alongside it. It
returns records as a table, JSON, JSONL, CSV, TSV, or URLs, and serves
the same operations over HTTP and MCP.

walmart is an independent tool and is not affiliated with Walmart.`,
		Site: BaseURL,
		Repo: "https://github.com/tamnd/walmart-cli",
	}
}

// Register installs the client factory and every operation onto app. A resolver
// op (Single) names its own record type and answers `ant get`; a List op
// enumerates a parent resource's members and answers `ant ls`.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)
	app.CommandGroup("read", "Read public Walmart data")
	app.CommandGroup("store", "Read Walmart stores")
	app.CommandGroup("category", "Read a category, its items, and its children")
	app.CommandGroup("ref", "Resolve references to ids and URLs (offline)")

	// Top-level reads.
	kit.Handle(app, kit.OpMeta{
		Name: "search", Group: "read", List: true,
		Summary: "Search products by keyword",
		URIType: "product",
		Args:    []kit.Arg{{Name: "query", Help: "search keywords"}},
	}, search)

	kit.Handle(app, kit.OpMeta{
		Name: "product", Group: "read", Single: true,
		Summary: "Show one product by id",
		URIType: "product", Resolver: true,
		Args: []kit.Arg{{Name: "id", Help: "item id or /ip/ URL"}},
	}, getProduct)

	kit.Handle(app, kit.OpMeta{
		Name: "deals", Group: "read", List: true,
		Summary: "The current rollbacks",
		URIType: "product",
	}, deals)

	kit.Handle(app, kit.OpMeta{
		Name: "trending", Group: "read", List: true,
		Summary: "The trending products",
		URIType: "product",
	}, trending)

	kit.Handle(app, kit.OpMeta{
		Name: "suggest", Group: "read", List: true,
		Summary: "Search autocomplete suggestions",
		Args:    []kit.Arg{{Name: "prefix", Help: "the typed prefix"}},
	}, suggest)

	// Store: a single store, stores near a ZIP.
	kit.Handle(app, kit.OpMeta{
		Name: "show", Parent: "store", Single: true,
		Summary: "Show a store's profile",
		URIType: "store", Resolver: true,
		Args: []kit.Arg{{Name: "id", Help: "store number or /store/ URL"}},
	}, getStore)

	kit.Handle(app, kit.OpMeta{
		Name: "find", Parent: "store", List: true,
		Summary: "Find stores near a ZIP code",
		URIType: "store",
		Args:    []kit.Arg{{Name: "zip", Help: "a US ZIP code"}},
	}, findStores)

	// Category: metadata, items, children.
	kit.Handle(app, kit.OpMeta{
		Name: "show", Parent: "category", Single: true,
		Summary: "Show a category's metadata",
		URIType: "category", Resolver: true,
		Args: []kit.Arg{{Name: "id", Help: "category id or /cp/ URL"}},
	}, getCategory)

	kit.Handle(app, kit.OpMeta{
		Name: "browse", Parent: "category", List: true,
		Summary: "List the items in a category",
		URIType: "product",
		Args:    []kit.Arg{{Name: "id", Help: "category id or /cp/ URL"}},
	}, categoryBrowse)

	kit.Handle(app, kit.OpMeta{
		Name: "tree", Parent: "category", List: true,
		Summary: "List a category's child categories",
		URIType: "category",
		Args:    []kit.Arg{{Name: "id", Help: "category id or /cp/ URL (empty for the top level)", Optional: true}},
	}, categoryTree)

	// Reference tools (offline).
	kit.Handle(app, kit.OpMeta{
		Name: "id", Parent: "ref", Single: true,
		Summary: "Classify a reference into its (kind, id)",
		Args:    []kit.Arg{{Name: "ref", Help: "any Walmart URL, path, or id"}},
	}, classifyRef)

	kit.Handle(app, kit.OpMeta{
		Name: "url", Parent: "ref", Single: true,
		Summary: "Build the canonical URL for a (kind, id)",
		Args: []kit.Arg{
			{Name: "kind", Help: "product, store, or category"},
			{Name: "id", Help: "the id for that kind"},
		},
	}, buildURL)
}

// newClient builds the client from the host-resolved config, so a host and the
// standalone binary pace and identify themselves the same way.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	return ClientFromConfig(cfg), nil
}

// ClientFromConfig maps the framework config onto a walmart.Config and returns a
// client. The affiliate credentials are read from the environment, not flags, so
// they never land in shell history.
func ClientFromConfig(cfg kit.Config) *Client {
	wc := DefaultConfig()
	if cfg.Rate > 0 {
		wc.Delay = cfg.Rate
	}
	if cfg.Retries >= 0 {
		wc.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		wc.Timeout = cfg.Timeout
	}
	if ua := cfg.Extra["user-agent"]; ua != "" {
		wc.UserAgent = ua
	} else if cfg.UserAgent != "" {
		wc.UserAgent = cfg.UserAgent
	}
	if z := cfg.Extra["zip"]; z != "" {
		wc.Zip = z
	} else if z := os.Getenv("WALMART_ZIP"); z != "" {
		wc.Zip = z
	}
	wc.ConsumerID = os.Getenv("WALMART_CONSUMER_ID")
	wc.PrivateKey = os.Getenv("WALMART_PRIVATE_KEY")
	wc.PrivateKeyFile = os.Getenv("WALMART_PRIVATE_KEY_FILE")
	if kv := os.Getenv("WALMART_KEY_VERSION"); kv != "" {
		wc.KeyVersion = kv
	}
	wc.PublisherID = os.Getenv("WALMART_PUBLISHER_ID")
	wc.CacheDir = cfg.CacheDir
	wc.NoCache = cfg.NoCache
	if ttl := cfg.Extra["cache-ttl"]; ttl != "" {
		if d, err := time.ParseDuration(ttl); err == nil {
			wc.CacheTTL = d
		}
	}
	wc.Refresh = cfg.Extra["refresh"] == "true"
	return NewClient(wc)
}

// Defaults seeds the framework baseline with walmart's own values, so an unset
// --rate or --timeout uses the walmart default rather than the generic kit one.
// It is passed to kit.New via kit.WithDefaults.
func Defaults(c *kit.Config) {
	def := DefaultConfig()
	c.Rate = def.Delay
	c.Retries = def.Retries
	c.Timeout = def.Timeout
	c.UserAgent = def.UserAgent
}

// Classify turns any accepted input into the canonical (type, id), so `ant
// resolve` and `ant url` touch no network.
func (Domain) Classify(input string) (uriType, id string, err error) {
	r := Classify(input)
	if r.Kind == "unknown" {
		return "", "", errs.Usage("unrecognized walmart reference: %q", input)
	}
	return r.Kind, r.ID, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	u := URLFor(uriType, id)
	if u == "" {
		return "", errs.Usage("walmart has no resource type %q", uriType)
	}
	return u, nil
}

// mapErr translates a library error into a kit error so the exit code matches the
// rest of the fleet: a missing entity reads as "not found" (exit 6), a throttle
// as "rate limited" (exit 5), and the bot wall as "need auth" (exit 4).
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return errs.NotFound("%s", err.Error())
	case errors.Is(err, ErrRateLimited):
		return errs.RateLimited("%s", err.Error())
	case errors.Is(err, ErrBlocked):
		return errs.NeedAuth("%s", err.Error())
	default:
		return err
	}
}

// limitOr returns the operator's --limit when set, else the command's own
// default fetch count.
func limitOr(limit, def int) int {
	if limit > 0 {
		return limit
	}
	return def
}
