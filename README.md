# walmart

A command line for [Walmart](https://www.walmart.com). One binary that completes
the search box, lists the departments, resolves any Walmart reference offline,
and best-effort opens a product, runs a keyword search, browses a category, finds
a store, or reads the rollbacks. No API key, no login, nothing to run alongside
it.

```
walmart suggest keyboard -n 6 --fields term
```

```
╭─────────────────────────────╮
│ TERM                        │
├─────────────────────────────┤
│ keyboard                    │
│ keyboard and mouse          │
│ keyboard and mouse wireless │
│ keyboard piano              │
│ keyboards for computers     │
│ keyboard wireless           │
╰─────────────────────────────╯
```

On a terminal the table header and JSON values are colorized; piped to a file or
another program the output drops to plain text so it parses cleanly. Use
`--color always` to keep color through a pipe, or `--color never` to drop it.

Full documentation: [walmart-cli.tamnd.com](https://walmart-cli.tamnd.com).

## Why

Reading Walmart programmatically usually means a developer account and the
Affiliate API, all to see things a logged-out browser shows for free. `walmart`
reads the same public pages a visitor does, lifts the data out of the
`__NEXT_DATA__` island Walmart embeds in every page, and shapes each surface into
a clean record with real output formats and pipelines that compose.

Walmart fronts its site with a strict bot manager, and this tool is honest about
what that leaves reachable. Autocomplete answers from any network, and the
department tree reads from the homepage. The product, search, category, and store
pages sit behind the wall, which hard-walls them from datacenter IPs; those are
best-effort, and they fall back to Walmart's documented Affiliate API when you
opt in with a free developer account's credentials. See
[what anonymous access reaches](#what-anonymous-access-reaches).

## Install

```sh
go install github.com/tamnd/walmart-cli/cmd/walmart@latest
```

Or grab a prebuilt binary from the [releases page](https://github.com/tamnd/walmart-cli/releases).
The binary is pure Go with no runtime dependencies. You can also run the
container image:

```sh
docker run --rm ghcr.io/tamnd/walmart:latest --help
```

Build from source:

```sh
git clone https://github.com/tamnd/walmart-cli
cd walmart-cli
make build      # produces ./bin/walmart
```

## Quick start

```sh
walmart suggest keyboard              # search-box autocomplete
walmart category tree                 # the top-level departments
walmart product 5037034321            # one product by id (best-effort, see below)
walmart search "air fryer"            # keyword search (best-effort, see below)
walmart category browse 3944          # the items in a category (best-effort)
walmart deals                         # the current rollbacks (best-effort)
walmart store find 94040              # stores near a ZIP (needs credentials)
walmart store show 2280               # a store's profile (best-effort)
```

Most commands accept a bare id, an `/ip/`, `/store/`, `/cp/`, or `/browse/`
path, or a full Walmart URL wherever they take a reference. The `ref` commands
resolve those offline, with no network call:

```sh
walmart ref id "https://www.walmart.com/ip/Apple-iPhone-13/123456789012" -o json
```

```json
[
  {
    "input": "https://www.walmart.com/ip/Apple-iPhone-13/123456789012",
    "kind": "product",
    "id": "123456789012",
    "url": "https://www.walmart.com/ip/123456789012"
  }
]
```

## How it works

Walmart renders its public pages server-side and ships the data as a
`__NEXT_DATA__` JSON island. `walmart` GETs a page, reads that island, and maps
the product or the search grid onto a clean record. The autocomplete host answers
plain JSON. It paces and caches requests and retries the transient failures, and
it sends a browser user-agent because that is what a logged-out reader looks
like. No API key, no token.

Prices are read in whatever currency Walmart serves the page in, so each record
carries an explicit `currency` field alongside the number.

## What anonymous access reaches

Walmart fronts its site with a bot manager that serves its "Robot or human?"
challenge to traffic it does not trust, and it does not treat every surface the
same. This tool sorts the surfaces into what it can read reliably and what it
cannot, and never pretends the line is elsewhere.

Read reliably from any network:

- `suggest` (the typeahead host)
- `category tree` (the department links on the homepage)

Walled from datacenter IPs, best-effort:

- `product` (the `/ip/` page)
- `search` (the `/search` results)
- `category browse`, `category show` (the `/cp/` and `/browse/` pages)
- `store show` (the `/store/` page)

API-only without a residential network:

- `store find` (no anonymous store-locator endpoint exists)
- `deals`, `trending` (best-effort web, then the API)

From a home network the best-effort surfaces usually answer; from a datacenter
they hit the bot wall and exit 4. When that happens you have two remedies, and
the error message names both: run from a residential network, or opt in to
Walmart's documented Affiliate API by setting `WALMART_CONSUMER_ID` and a private
key (a free developer account, each request signed with the key, no user login),
and the walled commands fall back to it automatically.

Records carry only fields a logged-out reader can fill. There is no cart, no
order history, no personalized price, and no in-store inventory tied to an
account, because none of that exists without one. A product shows the price, the
strikethrough was-price, the manufacturer list price, the seller, the rating, and
the photo gallery a visitor sees; a field a page does not show is left empty
rather than guessed.

When something is genuinely missing the exit code says which, so a script can
tell the cases apart:

| Exit | Meaning |
| --- | --- |
| 0 | ok |
| 2 | usage error |
| 3 | no results (the resource is genuinely empty) |
| 4 | need auth, or the bot wall |
| 5 | rate limited (raise `--rate`) |
| 6 | not found (unknown id, removed product, bad reference) |
| 8 | network error |

## Affiliate API fallback

The fallback is opt-in and reads its credentials from the environment, never from
a flag, so they stay out of shell history. Sign up for a free
[Walmart developer account](https://developer.walmart.com), then set:

```sh
export WALMART_CONSUMER_ID=...               # the consumer id
export WALMART_PRIVATE_KEY_FILE=~/wm_key.pem # path to the RSA private key (PEM)
# or paste the key itself:
export WALMART_PRIVATE_KEY="$(cat ~/wm_key.pem)"
export WALMART_KEY_VERSION=1                  # the key version (default 1)
```

With those set, every walled command tries the page first and falls back to the
signed API on a wall, and `store find` and `trending` use the API directly.
`WALMART_PUBLISHER_ID` and `WALMART_ZIP` are optional and forwarded when present.

## Commands

| Command | What it does |
| --- | --- |
| `search <query>` | Keyword search (best-effort, see above) |
| `product <id>` | One product by id (best-effort, see above) |
| `deals` | The current rollbacks |
| `trending` | The trending products (needs credentials) |
| `suggest <prefix>` | Search-box autocomplete suggestions |
| `store show <id>` | A store's public profile |
| `store find <zip>` | Stores near a ZIP code (needs credentials) |
| `category show <id>` | A category's metadata |
| `category browse <id>` | The items in a category |
| `category tree [id]` | A category's child categories (empty for the top level) |
| `ref id <ref>` | Classify a reference into its (kind, id), offline |
| `ref url <kind> <id>` | Build the canonical URL for a (kind, id), offline |
| `serve` | Serve the same operations over HTTP as NDJSON |
| `mcp` | Serve the same operations to an agent over MCP |
| `version` | Print version, commit, and build date |

A product is addressed by its numeric item id, like `5037034321`, or an `/ip/`
URL; a category by its id, like `3944`, or a `/cp/` URL. Run
`walmart <command> --help` for the full flag list on any command.

## Output

Every command shares one output contract. The default adapts to where output
goes, a table on a terminal and JSONL in a pipe, so the same command reads well
by hand and parses cleanly downstream.

```sh
walmart suggest keyboard -n 4 --fields term,image
```

Pick the format with `-o table|markdown|json|jsonl|csv|tsv|url|raw`, choose
columns with `--fields a,b,c`, render a custom line with `--template`, drop the
header with `--no-header`, and cap results with `-n/--limit`. The `url` format
prints just the canonical URL of each record, which is handy for piping into
another tool.

## Recipes

Autocomplete terms for a prefix, one per line, for a downstream job:

```sh
walmart suggest "air fryer" -n 20 -o jsonl > terms.jsonl
```

The top-level departments with their ids:

```sh
walmart category tree --fields id,name
```

A product as JSON, piped to jq (best-effort or with credentials):

```sh
walmart product 5037034321 -o json | jq '{name, price, was, currency, rating}'
```

The canonical URLs of a search, one per line:

```sh
walmart search "air fryer" -n 50 -o url
```

Tee a category into a local SQLite store, keyed by each item's id, then query it:

```sh
walmart category browse 3944 -n 200 --db walmart.db
```

## Serve it

The same operations are available over HTTP and as an MCP tool set for agents,
with no extra code:

```sh
walmart serve --addr :7777    # GET /v1/... returns NDJSON
walmart mcp                   # speak MCP over stdio
```

## Use it as a resource-URI driver

`walmart` registers a `walmart` domain the way a program registers a database
driver with `database/sql`. A host enables it with one blank import:

```go
import _ "github.com/tamnd/walmart-cli/walmart"
```

Then [ant](https://github.com/tamnd/ant) (or any program that links the package)
dereferences `walmart://` URIs without knowing anything about Walmart:

```sh
ant get walmart://product/<id>        # fetch a product
ant get walmart://store/<id>          # fetch a store
ant get walmart://category/<id>       # fetch a category
ant url walmart://category/<id>       # the live https URL
```

Records carry explicit edges, so a host can breadth-first crawl the graph and
write it to disk: a listing or deal links to its product, a product links to its
category and its variant siblings, and a category links to its parent and
children. `ant export <uri> --follow N` walks those edges. See the
[resource-URI guide](https://walmart-cli.tamnd.com/guides/resource-uris/) for the
full edge map.

## Development

```
cmd/walmart/  thin main: hands cli.NewApp to kit.Run
cli/          assembles the kit App from the walmart domain
walmart/      the library: web client, Affiliate API backend, the __NEXT_DATA__
              parser, data models, and domain.go (the driver)
docs/         tago documentation site
```

```sh
make build      # ./bin/walmart
make test       # go test ./...
make vet        # go vet ./...
```

Every read command is declared once as a kit operation in `walmart/domain.go`.
That single declaration becomes the CLI subcommand, the HTTP route, and the MCP
tool, so the three surfaces never drift.

## Releasing

Push a version tag and GitHub Actions runs GoReleaser, which builds the archives,
Linux packages, the multi-arch GHCR image, checksums, SBOMs, and a cosign
signature:

```sh
git tag v0.1.0
git push --tags
```

The Homebrew and Scoop steps self-disable until their tokens exist, so the first
release works with no extra secrets.

## License

`walmart` is an independent tool and is not affiliated with Walmart. Apache-2.0,
see [LICENSE](LICENSE).
