// Package cli assembles the walmart command tree from the walmart domain on top
// of the any-cli/kit framework. Every read command is declared once as a kit
// operation in the walmart package, so the CLI, the HTTP API (walmart serve),
// and the MCP server (walmart mcp) all derive from one registry.
package cli

import (
	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/walmart-cli/walmart"
)

// Build metadata, set via -ldflags at release time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// builder holds the domain-global flags while the app is assembled, then folds
// them onto the resolved config in finalize.
type builder struct {
	userAgent string
	zip       string
	cacheTTL  string
	refresh   bool
}

// NewApp assembles the kit App: the walmart domain installs the client factory
// and the operations, this package adds the global flags and the version command,
// and kit provides the CLI, API, and MCP surfaces.
//
// To add a command, declare it in walmart/domain.go with kit.Handle and it
// appears here automatically. Reach for app.AddCommand only for a verb that does
// not fit the emit-records shape, the way version does below.
func NewApp() *kit.App {
	b := &builder{}
	id := walmart.Identity()
	id.Version = Version

	app := kit.New(id, kit.WithDefaults(walmart.Defaults))
	app.GlobalFlags(b.globals)
	app.Finalize(b.finalize)

	walmart.Domain{}.Register(app)
	app.AddCommand(newVersionCmd())
	return app
}

func (b *builder) globals(f *kit.FlagSet) {
	f.StringVar(&b.userAgent, "user-agent", walmart.DefaultUserAgent, "User-Agent sent with each request")
	f.StringVar(&b.zip, "zip", "", "ZIP code context for store and price reads (default WALMART_ZIP)")
	f.StringVar(&b.cacheTTL, "cache-ttl", walmart.DefaultCacheTTL.String(), "how long a cached response stays fresh")
	f.BoolVar(&b.refresh, "refresh", false, "fetch fresh copies and rewrite the cache, ignoring any hit")
}

func (b *builder) finalize(c *kit.Config) {
	if c.Extra == nil {
		c.Extra = map[string]string{}
	}
	if b.userAgent != "" {
		c.Extra["user-agent"] = b.userAgent
	}
	if b.zip != "" {
		c.Extra["zip"] = b.zip
	}
	if b.cacheTTL != "" {
		c.Extra["cache-ttl"] = b.cacheTTL
	}
	if b.refresh {
		c.Extra["refresh"] = "true"
	}
}
