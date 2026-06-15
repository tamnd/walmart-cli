---
title: "Add a command"
description: "Model a real walmart record and expose it as a command, a route, and a tool at once."
weight: 10
---

A walmart operation is declared once and shows up everywhere: as a CLI
subcommand, as an HTTP route under `serve`, as an MCP tool, and as a
`walmart://` URI a host can dereference. You add one by touching three files,
and every surface updates itself. The `product` command is the worked example
below.

## 1. Model the record

In `walmart/types.go`, a struct describes the thing you fetch. The `kit` and
`table` struct tags decide how a host addresses it and how it prints:

```go
type Product struct {
    ID           string   `json:"id" kit:"id"`              // the URI id
    Name         string   `json:"name"`
    Brand        string   `json:"brand,omitempty"`
    Price        float64  `json:"price,omitempty"`
    Currency     string   `json:"currency,omitempty"`
    Availability string   `json:"availability,omitempty"`
    Description  string   `json:"description,omitempty" kit:"body"` // what cat and Markdown print
    URL          string   `json:"url,omitempty"`
}
```

- `kit:"id"` marks the field that becomes the URI id.
- `kit:"body"` marks the prose that `cat` and the Markdown export render.
- `json:",omitempty"` keeps a record honest: a field Walmart did not serve is
  absent rather than zero.

## 2. Fetch it

In `walmart/product.go`, a client method returns the record. The two-layer
client hides whether the data came from the public page or the Affiliate API
fallback:

```go
func (c *Client) GetProduct(ctx context.Context, ref string) (*Product, error) {
    id := productID(ref) // accept a bare id or an /ip/ URL
    body, err := c.get(ctx, c.BaseURL+"/ip/"+id)
    if err != nil {
        return nil, err // ErrBlocked, ErrRateLimited, ErrNotFound flow up unchanged
    }
    // parse __NEXT_DATA__ into a Product ...
    return p, nil
}
```

## 3. Declare the operation

In `walmart/ops.go`, add an input struct and a handler. The struct tags tell
`kit` what is a positional argument, what is an inherited flag, and where the
client is injected:

```go
type productRef struct {
    ID     string  `kit:"arg" help:"item id or /ip/ URL"`
    Client *Client `kit:"inject"`
}

func getProduct(ctx context.Context, in productRef, emit func(*Product) error) error {
    p, err := in.Client.GetProduct(ctx, in.ID)
    if err != nil {
        return mapErr(err)
    }
    return emit(p)
}
```

Then register it in `Register` in `walmart/domain.go`:

```go
kit.Handle(app, kit.OpMeta{
    Name: "product", Group: "read", Single: true,
    Summary: "Show one product by id",
    URIType: "product", Resolver: true,
    Args: []kit.Arg{{Name: "id", Help: "item id or /ip/ URL"}},
}, getProduct)
```

That is the whole change. `kit.Handle` reflects the input for flags and the
output for the record shape, so the operation immediately becomes:

```bash
walmart product 5037034321              # the command
curl 'localhost:7777/v1/product/5037034321'   # the route, under serve
ant get walmart://product/5037034321    # the URI dereference, via a host
```

## Resolver ops and list ops

Two flags shape how a host treats an operation:

- **`Single: true`** with **`Resolver: true`** marks the canonical one-record
  fetch for a `URIType`. It answers `ant get`. `product`, `store show`, and
  `category show` are the resolvers.
- **`List: true`** marks a member-lister for a parent resource. It answers
  `ant ls`. A list op emits records that are themselves addressable, so every
  member is a URI a host can follow. `search`, `category browse`, and
  `category tree` do this, each tagged with the `URIType` of the members it
  emits (`product` for `search` and `browse`, `category` for `tree`).

## Map errors to exit codes

Return through `mapErr` so every surface reports the same outcome with the same
exit code: the bot wall reads as need-auth (exit 4), a throttle as rate-limited
(exit 5), a missing item as not-found (exit 6):

```go
case errors.Is(err, ErrBlocked):
    return errs.NeedAuth("%s", err.Error())
case errors.Is(err, ErrRateLimited):
    return errs.RateLimited("%s", err.Error())
case errors.Is(err, ErrNotFound):
    return errs.NotFound("%s", err.Error())
```

See [output formats](/reference/output/) for how records render, and
[resource URIs](/guides/resource-uris/) for how a host addresses them.
