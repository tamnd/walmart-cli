---
title: "walmart"
description: "walmart reads public Walmart data (autocomplete, departments, and best-effort products, search, categories, stores, and deals) into structured records over a CLI, an HTTP server, and an MCP tool set."
heroTitle: "walmart, from the command line"
heroLead: "Complete the search box, list the departments, resolve any Walmart reference offline, and best-effort open a product, run a search, browse a category, find a store, or read the rollbacks. One pure-Go binary, no API key, output that pipes into the rest of your tools, and a resource-URI driver other programs can address."
heroPrimaryURL: "/getting-started/quick-start/"
heroPrimaryText: "Get started"
---

`walmart` reads the public Walmart pages a logged-out browser sees, lifts the
data out of the `__NEXT_DATA__` island they embed, and gets out of your way.

```bash
walmart suggest keyboard          # search-box autocomplete
walmart category tree             # the top-level departments
walmart product 5037034321        # one product by id (best-effort)
walmart search "air fryer"        # keyword search (best-effort)
walmart serve --addr :7777        # the same operations over HTTP
```

There is no API key for the core, no login, and nothing to run alongside it.
Output adapts to where it goes: an aligned table on your terminal, JSONL the
moment you pipe it somewhere.

## Honest about what is reachable

Walmart fronts its site with a strict bot manager that does not treat every
surface the same. `walmart` is explicit about the line. Autocomplete reads from
any network, and the department tree reads from the homepage. The product,
search, category, and store pages are walled from datacenter IPs, so they are
best-effort and fall back to Walmart's documented Affiliate API when you opt in
with a free developer account's credentials. See
[what anonymous access reaches](/getting-started/introduction/#what-anonymous-access-reaches).

## Two ways to use it

- **As a command** for reading Walmart by hand or in a script. Start with the
  [quick start](/getting-started/quick-start/).
- **As a resource-URI driver** so a host like
  [ant](https://github.com/tamnd/ant) can address Walmart as `walmart://` URIs
  and follow links across sites. See [resource URIs](/guides/resource-uris/).

Both are the same code: one operation, declared once, is a CLI command, an HTTP
route, an MCP tool, and a URI dereference.

## Where to go next

- New here? Read the [introduction](/getting-started/introduction/), then the
  [quick start](/getting-started/quick-start/).
- Installing? See [installation](/getting-started/installation/).
- Doing a specific job? The [guides](/guides/) are task-first.
- Need every flag? The [CLI reference](/reference/cli/) is the full surface.
