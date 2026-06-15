---
title: "Troubleshooting"
description: "The handful of things that trip people up, and how to fix each one."
weight: 40
---

Most of these come down to network reality or how Walmart serves its data, not a
bug. Each case maps to an exit code so a script can tell them apart.

## A command exits 4

The product, search, category, and store surfaces sit behind Walmart's bot
manager, which walls them from datacenter IPs. From a home network they usually
answer; from a datacenter or a cloud host they hit the wall, and `walmart`
reports need-auth (exit 4) rather than pretending the result was empty. Two
remedies, and the error message names both: run from a residential network, or
opt in to the Affiliate API by setting `WALMART_CONSUMER_ID` and a private key so
the commands fall back to it. `store find` and `trending` have no anonymous
endpoint and need the credentials regardless of network. See
[configuration](/reference/configuration/#opting-in-to-the-affiliate-api).

The `suggest` and `category tree` surfaces are not affected; they read from any
network.

## Requests start failing or returning 429

Walmart rate-limits like any public site. `walmart` already paces requests and
retries the transient failures, but a hard limit still means backing off, and it
reports rate-limited (exit 5). Raise the delay between requests with `--rate`
(for example `--rate 1s`) and retry later. A burst of 429 or 5xx responses is the
site asking you to slow down, not a defect.

## A reference is not found

An unknown id, a removed product, or a reference `walmart` cannot classify
reports not-found (exit 6). Check that the id is spelled the way Walmart uses it,
and that the product still exists in a private browser window before assuming it
is gone.

## Prices come back without a currency you expected

Walmart serves US pages in USD, but the price and its currency are read together,
so each record carries an explicit `currency` field alongside the number. Read
the two together rather than assuming a currency.

## The binary is not on your PATH

`go install` puts the binary in `$(go env GOPATH)/bin` (usually `~/go/bin`), and
a release archive leaves it wherever you unpacked it. If your shell cannot find
`walmart`, add that directory to your `PATH`. See
[installation](/getting-started/installation/).

## Seeing what walmart actually did

When something behaves unexpectedly, `-v` adds per-request detail so you can see
the URLs it hit and the responses it got. That is usually enough to tell a bot
wall apart from a rate limit apart from a genuinely empty result. Add
`--no-cache` to force a fresh fetch when you suspect a stale cached page.
