---
title: "Resource URIs"
description: "Use walmart as a database/sql-style driver so a host program can address Walmart as walmart:// URIs."
weight: 20
---

`walmart` is a command line, but the `walmart` Go package is also a small driver
that makes Walmart addressable as a resource URI. A host program registers it the
way a program registers a database driver with `database/sql`, then dereferences
`walmart://` URIs without knowing anything about how Walmart is fetched.

The host that does this today is [ant](https://github.com/tamnd/ant), a single
binary that puts one URI namespace over a family of site tools. The examples
below use `ant`; any program that links the package gets the same behaviour.

## Mounting the driver

A host enables the driver with one blank import, exactly like
`import _ "github.com/lib/pq"`:

```go
import _ "github.com/tamnd/walmart-cli/walmart"
```

The package's `init` registers a domain with the scheme `walmart` for the hosts
`www.walmart.com` and `walmart.com`. The standalone `walmart` binary does not
change.

## Addressing records

A URI is `scheme://authority/id`. The resolver types are:

| URI                          | What it is                          |
| ---------------------------- | ----------------------------------- |
| `walmart://product/<id>`     | one product, keyed by its item id   |
| `walmart://store/<id>`       | a store's public profile            |
| `walmart://category/<id>`    | a category, keyed by its numeric id |

```bash
ant get walmart://product/5037034321        # the product record
ant get walmart://category/3944             # the category record
ant url walmart://product/5037034321        # the live https URL
ant resolve https://www.walmart.com/cp/electronics/3944  # a pasted link, back to its URI
```

`product`, `store`, and `category` are best-effort: from a datacenter they may
hit Walmart's bot wall and report need-auth, the same as the matching commands.
See [what anonymous access reaches](/getting-started/introduction/#what-anonymous-access-reaches).

## Collections

`ls` lists the members of a collection. Each list operation has its own
authority, so they never shadow one another:

| URI                          | What it lists                          |
| ---------------------------- | -------------------------------------- |
| `walmart://search/<query>`   | products matching a keyword            |
| `walmart://category/<id>`    | the items in a category                |
| `walmart://categories/<id>`  | a category's child categories          |
| `walmart://stores/<zip>`     | stores near a ZIP code                 |
| `walmart://deals`            | the current rollbacks                  |
| `walmart://trending`         | the trending products                  |

```bash
ant ls walmart://search/cordless%20drill     # products matching the keyword
ant ls walmart://category/3944               # the items in the category
ant ls walmart://categories/3944             # the child categories
```

## Walking the graph

Every record carries explicit edges to the records it points at, so a host can
breadth-first crawl the site and write it to disk without scraping URLs out of
free text. The edges are:

| From       | Field        | Edge to                          |
| ---------- | ------------ | -------------------------------- |
| `Listing`  | `item`       | `walmart://product/<id>`         |
| `Deal`     | `item`       | `walmart://product/<id>`         |
| `Product`  | `category_id`| `walmart://category/<id>`        |
| `Product`  | `variants`   | `walmart://product/<id>` (each sibling) |
| `Category` | `parent_id`  | `walmart://category/<id>` (up)   |
| `Category` | `children`   | `walmart://category/<id>` (down, each) |

A search result or a category page links straight through to the full product;
a product links up to its leaf category and across to its colour, size, and
configuration siblings; a category links both up to its parent and down to its
children. Starting from any node, `--follow` walks these edges:

```bash
ant export walmart://categories/ --follow 3 --to ./data   # crawl the taxonomy down three levels
ant export walmart://product/5037034321 --follow 1 --to ./data  # a product, its category, and its variants
```

Each record is written under its minted URI with its edges intact, so the saved
set reconstructs the slice of the site that was reached: the category tree, the
products in each category, and the variant clusters that tie products together.

These edge fields stay out of the table and CSV views (they would be noise in a
terminal) but are always present in the JSON and JSONL a host reads.

## Why this is the same code

The driver and the binary share one definition per operation. A resolver op
answers both `walmart product` on the command line and
`ant get walmart://product/...` through a host, from the same handler and the
same client. There is no second implementation to keep in step.
