---
title: "Configuration"
description: "Environment variables, the Affiliate API credentials, defaults, and the data directory."
weight: 20
---

`walmart` needs almost no configuration: the core surfaces run anonymously
against public data out of the box. The settings below let you tune politeness,
opt in to the Affiliate API, and choose where data lands.

## Defaults

| Setting | Default | Flag |
|---|---|---|
| Requests | paced and retried on 429/5xx | `--rate`, `--retries` |
| Per-request timeout | 30s | `--timeout` |
| On-disk cache | under the data directory | `--no-cache` to bypass |

## Opting in to the Affiliate API

The product, search, category, and store surfaces are walled from datacenter
IPs, and the store locator has no anonymous endpoint at all. When you set a
consumer id and a private key, the walled commands fall back to Walmart's
documented Affiliate API, and `store find` and `trending` use it directly:

```bash
export WALMART_CONSUMER_ID=...               # the consumer id
export WALMART_PRIVATE_KEY_FILE=~/wm_key.pem # path to the RSA private key (PEM)
# or paste the key itself instead of pointing at a file:
export WALMART_PRIVATE_KEY="$(cat ~/wm_key.pem)"
export WALMART_KEY_VERSION=1                  # the key version (default 1)
```

The credentials come from a free
[Walmart developer account](https://developer.walmart.com). Unlike an OAuth
backend there is no token to exchange: `walmart` signs each request in place with
the RSA key, computing the signature from the consumer id, a millisecond
timestamp, and the key version. It reads the credentials from the environment
rather than flags so they never land in your shell history. The reliable core
surfaces (`suggest`, `category tree`) never touch the API.

`WALMART_PUBLISHER_ID` and `WALMART_ZIP` are optional and forwarded when present;
`WALMART_ZIP` also sets the ZIP context for the store and price reads, the same
as `--zip`.

## The data directory

Caches and any record store live under one data directory, chosen in this order:

1. `--data-dir`
2. `WALMART_DATA_DIR`
3. `$XDG_DATA_HOME/walmart`
4. `~/.local/share/walmart`

## Environment variables

Every flag has an environment fallback, prefixed `WALMART_` in upper case with
dashes as underscores. For example:

```bash
export WALMART_RATE=1s        # same as --rate 1s
export WALMART_DATA_DIR=~/data/walmart
```

Flags win over environment variables, which win over the built-in defaults.

## Sending records to a store

`--db` tees every emitted record into a store as a side effect of reading, so a
session fills a local database without a separate import step:

```bash
walmart category browse 3944 --db out.db        # SQLite file
walmart category browse 3944 --db 'postgres://...'
```
