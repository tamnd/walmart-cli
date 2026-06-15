---
title: "Introduction"
description: "What walmart is, how it is put together, and which surfaces it reads."
weight: 10
---

`walmart` reads public Walmart data the way a logged-out browser does: search-box
autocomplete, the department tree, a single product, a keyword search, a category
and the items in it, the store locator, and the current rollbacks. It is a single
binary. It speaks to Walmart over plain HTTPS, lifts the data out of the
`__NEXT_DATA__` island Walmart embeds in every page, and gets out of your way.
There is no API key for the core, no login, and nothing to run alongside it.

## How it is built

- A **library package** (`walmart`) holds the HTTP client, the Affiliate API
  backend, and the typed data models. It paces requests, sends a browser
  User-Agent because that is what a logged-out reader looks like, caches on disk,
  and retries the transient failures any public site throws under load.
- A **domain** (`walmart/domain.go`) declares each operation once on the
  [any-cli/kit](https://github.com/tamnd/any-cli) framework. That single
  declaration becomes a CLI command, an HTTP route, an MCP tool, and a
  resource-URI dereference. It is the one place you add to the tool.
- A thin **`cmd/walmart`** hands the assembled app to `kit.Run`, which builds the
  command tree and the serve and mcp surfaces.

## One operation, four surfaces

Because an operation is surface-neutral, the same `product` you run on the
command line is also a route and a tool:

```bash
walmart product 5037034321           # the command
walmart serve --addr :7777           # GET /v1/product/5037034321
walmart mcp                          # the product tool, over stdio
ant get walmart://product/5037034321 # the URI dereference (via a host)
```

## What anonymous access reaches

Walmart fronts its site with a bot manager that serves its "Robot or human?"
challenge to traffic it does not trust, and it does not treat every surface the
same. `walmart` sorts the surfaces into what it reads reliably and what it
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
and the walled commands fall back to it automatically. See
[configuration](/reference/configuration/#opting-in-to-the-affiliate-api).

Records carry only fields a logged-out reader can fill. There is no cart, no
order history, no personalized price, and no in-store inventory tied to an
account, because none of that exists without one. A product shows the price, the
strikethrough was-price, the manufacturer list price, the seller, the rating, and
the photo gallery a visitor sees; a field a page does not show is left empty
rather than guessed.

## Scope

`walmart` is a read-only client over data Walmart already serves publicly. It
reads that data and shapes it for you. That narrow scope keeps it a single small
binary with no database, no daemon, and no setup.

`walmart` is an independent tool and is not affiliated with Walmart.

Next: [install it](/getting-started/installation/), then take the
[quick start](/getting-started/quick-start/).
