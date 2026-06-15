---
title: "Quick start"
description: "Run your first walmart commands and shape their output."
weight: 30
---

Once `walmart` is on your `PATH`, complete a search box. `suggest` reads the
typeahead host and answers from any network:

```bash
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

The department tree reads from the homepage, which Walmart serves anonymously:

```bash
walmart category tree --fields id,name
```

By default you get an aligned table on a terminal. Ask for JSON when you want to
pipe it:

```bash
walmart suggest keyboard -o json
```

```json
[
  {
    "query": "keyboard",
    "term": "keyboard and mouse",
    "image": "https://i5.walmartimages.com/asr/..."
  }
]
```

## The best-effort surfaces

The product, search, category, and store pages sit behind Walmart's bot wall and
may exit 4 from a datacenter. See
[what anonymous access reaches](/getting-started/introduction/#what-anonymous-access-reaches).

```bash
walmart product 5037034321           # one product by id (best-effort)
walmart search "air fryer"           # keyword search (best-effort)
walmart category browse 3944         # the items in a category (best-effort)
walmart deals                        # the current rollbacks (best-effort)
walmart store find 94040             # stores near a ZIP (needs credentials)
```

Set `WALMART_CONSUMER_ID` and an Affiliate API private key and these commands
fall back to the documented API on a wall. See
[configuration](/reference/configuration/#opting-in-to-the-affiliate-api).

## Shape the output

The same flags work on every command:

```bash
walmart category tree --fields id,name
walmart product 5037034321 --template '{{.Name}} {{.Price}} {{.Currency}}'
walmart suggest keyboard -o jsonl | jq .term
```

`-o` takes `table`, `json`, `jsonl`, `csv`, `tsv`, `url`, or `raw`. Left to
`auto`, it prints a table to a terminal and JSONL into a pipe, so the same
command reads well by hand and parses cleanly downstream. See
[output formats](/reference/output/) for the full contract.

## Resolve a reference offline

The `ref` commands classify and build Walmart references with no network call:

```bash
walmart ref id "https://www.walmart.com/ip/Apple-iPhone-13/123456789012"
walmart ref url product 5037034321
```

## Serve it instead

The same operations are available over HTTP and to agents over MCP:

```bash
walmart serve --addr :7777 &
curl -s 'localhost:7777/v1/suggest/keyboard'   # NDJSON, one record per line
walmart mcp                                     # MCP over stdio
```

## What to read next

The [guides](/guides/) cover the common jobs, and the
[CLI reference](/reference/cli/) is the full command tree and flag list.
