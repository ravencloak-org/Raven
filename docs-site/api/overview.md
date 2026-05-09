---
title: "API Overview"
---

<BetaBanner />

# API Overview

Raven exposes a REST API from the Go backend. Every operation here is
generated from a single OpenAPI 3.1 spec — the same spec drives the Go
server's typed handlers via `oapi-codegen` (Phase 3, in progress) and the
docs you are reading now.

## Download the spec

The canonical OpenAPI spec is hosted alongside this site:

- [openapi.yaml](/api/openapi.yaml) (canonical)
- [openapi.json](/api/openapi.json) (auto-converted at build time)

Pipe it into Postman, Insomnia, Bruno, `openapi-generator`, or your own
`oapi-codegen` config:

```bash
curl -O https://docs.raven.ravencloak.org/api/openapi.yaml
oapi-codegen -package raven -generate types,client openapi.yaml > raven.gen.go
```

## Servers

| Environment | URL                              |
| ----------- | -------------------------------- |
| Production  | <https://api.ravencloak.org>     |
| Local dev   | <http://localhost:8080>          |

The production URL will move to `https://api.raven.ravencloak.org` when the
domain-migration ticket lands; this page will update automatically because
the URL is read from the same spec the Go server uses.

## Authentication

See [Authentication](/api/authentication) for the bearer-token scheme,
session cookies for the dashboard, and per-workspace API keys for embedding.
