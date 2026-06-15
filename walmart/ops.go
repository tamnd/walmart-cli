package walmart

import (
	"context"

	"github.com/tamnd/any-cli/kit/errs"
)

// ops.go holds the handler for every operation declared in domain.go. kit
// reflects each input struct into CLI flags, HTTP query params, and MCP tool
// arguments: kit:"arg" is a positional, kit:"flag,inherit" binds the shared
// --limit, and kit:"inject" receives the client newClient builds. The reference
// ops (id, url) take no client; they run offline.

// --- top-level reads ---

type queryIn struct {
	Query  string  `kit:"arg" help:"search keywords"`
	Limit  int     `kit:"flag,inherit"`
	Client *Client `kit:"inject"`
}

func search(ctx context.Context, in queryIn, emit func(*Listing) error) error {
	items, err := in.Client.Search(ctx, in.Query, limitOr(in.Limit, defaultPageSize))
	if err != nil {
		return mapErr(err)
	}
	return emitAll(items, emit)
}

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

type listIn struct {
	Limit  int     `kit:"flag,inherit"`
	Client *Client `kit:"inject"`
}

func deals(ctx context.Context, in listIn, emit func(*Deal) error) error {
	ds, err := in.Client.Deals(ctx, limitOr(in.Limit, 50))
	if err != nil {
		return mapErr(err)
	}
	return emitAll(ds, emit)
}

func trending(ctx context.Context, in listIn, emit func(*Listing) error) error {
	items, err := in.Client.Trending(ctx, limitOr(in.Limit, 50))
	if err != nil {
		return mapErr(err)
	}
	return emitAll(items, emit)
}

type prefixIn struct {
	Prefix string  `kit:"arg" help:"the typed prefix"`
	Limit  int     `kit:"flag,inherit"`
	Client *Client `kit:"inject"`
}

func suggest(ctx context.Context, in prefixIn, emit func(*Suggestion) error) error {
	ss, err := in.Client.Suggest(ctx, in.Prefix, limitOr(in.Limit, 0))
	if err != nil {
		return mapErr(err)
	}
	return emitAll(ss, emit)
}

// --- store ---

type storeRef struct {
	ID     string  `kit:"arg" help:"store number or /store/ URL"`
	Client *Client `kit:"inject"`
}

type storeFindIn struct {
	Zip    string  `kit:"arg" help:"a US ZIP code"`
	Limit  int     `kit:"flag,inherit"`
	Client *Client `kit:"inject"`
}

func getStore(ctx context.Context, in storeRef, emit func(*Store) error) error {
	s, err := in.Client.GetStore(ctx, in.ID)
	if err != nil {
		return mapErr(err)
	}
	return emit(s)
}

func findStores(ctx context.Context, in storeFindIn, emit func(*Store) error) error {
	stores, err := in.Client.FindStores(ctx, in.Zip, limitOr(in.Limit, defaultPageSize))
	if err != nil {
		return mapErr(err)
	}
	return emitAll(stores, emit)
}

// --- category ---

type categoryRef struct {
	ID     string  `kit:"arg" help:"category id or /cp/ URL"`
	Client *Client `kit:"inject"`
}

type categoryListIn struct {
	ID     string  `kit:"arg" help:"category id or /cp/ URL"`
	Limit  int     `kit:"flag,inherit"`
	Client *Client `kit:"inject"`
}

func getCategory(ctx context.Context, in categoryRef, emit func(*Category) error) error {
	cat, err := in.Client.GetCategory(ctx, in.ID)
	if err != nil {
		return mapErr(err)
	}
	return emit(cat)
}

func categoryBrowse(ctx context.Context, in categoryListIn, emit func(*Listing) error) error {
	items, err := in.Client.CategoryBrowse(ctx, in.ID, limitOr(in.Limit, defaultPageSize))
	if err != nil {
		return mapErr(err)
	}
	return emitAll(items, emit)
}

func categoryTree(ctx context.Context, in categoryListIn, emit func(*Category) error) error {
	cats, err := in.Client.CategoryTree(ctx, in.ID, limitOr(in.Limit, 0))
	if err != nil {
		return mapErr(err)
	}
	return emitAll(cats, emit)
}

// --- reference tools (offline) ---

type refIn struct {
	Ref string `kit:"arg" help:"any Walmart URL, path, or id"`
}

func classifyRef(_ context.Context, in refIn, emit func(*Ref) error) error {
	r := Classify(in.Ref)
	if r.Kind == "unknown" {
		return errs.Usage("unrecognized walmart reference: %q", in.Ref)
	}
	return emit(&r)
}

type urlIn struct {
	Kind string `kit:"arg" help:"product, store, or category"`
	ID   string `kit:"arg" help:"the id for that kind"`
}

func buildURL(_ context.Context, in urlIn, emit func(*Ref) error) error {
	u := URLFor(in.Kind, in.ID)
	if u == "" {
		return errs.Usage("walmart has no resource type %q", in.Kind)
	}
	return emit(&Ref{Input: in.Kind + "/" + in.ID, Kind: in.Kind, ID: in.ID, URL: u})
}

// emitAll streams a slice of records through emit.
func emitAll[T any](items []*T, emit func(*T) error) error {
	for _, it := range items {
		if err := emit(it); err != nil {
			return err
		}
	}
	return nil
}
