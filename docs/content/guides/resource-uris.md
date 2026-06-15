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

## Walking the graph

`ls` lists the members of a collection, and every member is itself an
addressable URI, so a host can follow the graph and write it to disk:

```bash
ant ls     walmart://category/3944          # the items in the category, as product URIs
ant export walmart://category/3944 --follow 1 --to ./data
```

The list operations (`search`, `category browse`, `category tree`, `deals`,
`trending`) emit records that are themselves addressable, so each member is a
`walmart://product/` or `walmart://category/` URI a host can fetch in turn.

## Why this is the same code

The driver and the binary share one definition per operation. A resolver op
answers both `walmart product` on the command line and
`ant get walmart://product/...` through a host, from the same handler and the
same client. There is no second implementation to keep in step.
