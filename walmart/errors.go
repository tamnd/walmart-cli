package walmart

import "errors"

// The library reports its outcomes as a few sentinel errors. domain.go's mapErr
// translates each into the kit error kind that carries the matching exit code,
// so the standalone binary and a host agree on what a wall, a throttle, and a
// miss mean.
var (
	// ErrNotFound is a missing entity: an unknown item id, an unknown store, or a
	// bad category. Walmart serves these as a 404 or a not-found shell page. Exit
	// code 6.
	ErrNotFound = errors.New("not found")

	// ErrRateLimited is a sustained HTTP 429 after the client's own retries. Slow
	// down with --rate. Exit code 5.
	ErrRateLimited = errors.New("rate limited")

	// ErrBlocked is the PerimeterX bot wall: a 403 or 412 interstitial, the
	// "Robot or human?" challenge page served with a 200, or an implausibly small
	// body on a page that should be large. Walmart walls the product page, search,
	// the category pages, and the store pages from datacenter IPs, so this is
	// expected there. The message names the two remedies (a residential IP, or
	// Affiliate API credentials). Exit code 4.
	ErrBlocked = errors.New("blocked by Walmart's bot wall (retry from a residential network, " +
		"or set WALMART_CONSUMER_ID and WALMART_PRIVATE_KEY to use the Affiliate API)")
)
