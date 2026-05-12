---
title: "Embed the Chat Widget"
---

# Embed the Chat Widget

Raven ships an embeddable chat widget as a standards-based **Web Component**
called `<raven-chat>`. Drop the script tag and the element on any HTML page —
your marketing site, docs portal, customer dashboard, or a static landing page —
point it at your Raven workspace, and your visitors can chat with your
knowledge base out of the box.

This page walks through embedding the widget end-to-end: creating an API key,
adding the two lines of HTML, customising the look, and locking the widget down
to the origins you trust.

## What you'll need

Before you start, make sure you have:

- A running **Raven workspace** (self-hosted or hosted) reachable over HTTPS.
- A **knowledge base** with at least one ingested document. See
  [First Knowledge Base](/get-started/first-knowledge-base) if you haven't
  ingested anything yet.
- The hostname of the site that will embed the widget (e.g.
  `https://www.example.com`).
- An admin or member role on the knowledge base — you'll need it to mint an
  API key.

## Create a widget API key

API keys in Raven are **scoped to a single knowledge base**, optionally
restricted to a list of allowed origins, and optionally rate-limited. The full
key value is shown **once at creation** and stored hashed thereafter; treat it
like a secret you can rotate, not a password you can recover.

### From the dashboard

1. Open the Raven dashboard and switch to the organisation and workspace that
   owns the knowledge base.
2. Navigate to **Knowledge Bases → _your KB_ → API Keys**.
3. Click **New API Key**.
4. Give it a descriptive name (for example `marketing-site-widget`).
5. Under **Allowed domains**, list every origin that will embed the widget,
   one per line, with scheme:
   ```
   https://www.example.com
   https://example.com
   ```
6. Optionally set a **rate limit** (requests per minute per key). Leave blank
   for the workspace default.
7. Click **Create**, then **copy the key value** from the confirmation dialog.
   You will not be able to view it again — if you lose it, revoke and create a
   new one.

### Via the API

The dashboard form is a thin wrapper around the REST API. The endpoint lives
under the knowledge-base path:

```
POST /api/v1/orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/api-keys
```

Authenticate as a workspace member and send a `CreateAPIKeyRequest` body:

```bash
curl -X POST \
  "https://app.example.com/api/v1/orgs/$ORG_ID/workspaces/$WS_ID/knowledge-bases/$KB_ID/api-keys" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SESSION_TOKEN" \
  -d '{
    "name": "marketing-site-widget",
    "allowed_domains": [
      "https://www.example.com",
      "https://example.com"
    ],
    "rate_limit": 60
  }'
```

The response contains the metadata plus a one-time `raw_key`:

```json
{
  "id": "ak_01HXYZ…",
  "knowledge_base_id": "kb_01HABC…",
  "name": "marketing-site-widget",
  "key_prefix": "rvn_…",
  "allowed_domains": ["https://www.example.com", "https://example.com"],
  "rate_limit": 60,
  "status": "active",
  "raw_key": "rvn_live_…",
  "created_at": "2026-05-12T10:42:00Z"
}
```

Capture `raw_key` in your secret store immediately. Only the `key_prefix` is
ever returned on subsequent `GET` calls.

## Minimal embed

Two tags is the whole story. Paste them anywhere inside `<body>`:

```html
<raven-chat
  api-key="rvn_live_your_key_here"
  api-url="https://app.example.com"
></raven-chat>
<script type="module" src="https://app.example.com/chat-widget.js"></script>
```

That's it. `<raven-chat>` mounts a floating launcher button (bottom-right by
default). Clicking it expands the chat panel and streams responses from your
knowledge base over SSE.

- **`api-key`** — the `raw_key` value you copied above.
- **`api-url`** — the base URL of your Raven API (no trailing slash, no path).
- **Script source** — the same origin as `api-url`. The widget bundle is built
  from `frontend/src/components/chat-widget/index.ts` and served as
  `chat-widget.js`. It registers the `<raven-chat>` custom element on first
  load and guards against double-registration if the script ends up included
  twice.

## Attributes

The component reads its configuration from HTML attributes. Every attribute
listed below is observed — change it at runtime via `setAttribute()` and the
widget updates without a reload.

| Attribute       | Required | Default     | Purpose                                                                 |
|-----------------|----------|-------------|-------------------------------------------------------------------------|
| `api-key`       | yes      | —           | The widget API key minted above.                                        |
| `api-url`       | yes      | —           | Base URL of the Raven API (e.g. `https://app.example.com`).             |
| `theme-color`   | no       | `#6366f1`   | Primary brand colour. Drives the launcher, header, and send button.     |
| `avatar-url`    | no       | (none)      | URL for the assistant avatar shown in the panel header.                 |
| `welcome-text`  | no       | `Hi! How can I help?` | First-line greeting shown above the message list.            |
| `position`      | no       | `bottom-right` | Launcher anchor. Accepts `bottom-right` or `bottom-left`.           |
| `voice-enabled` | no       | `false`     | Set to `"true"` to expose a voice-call button (requires `livekit-url`). |
| `livekit-url`   | no       | —           | LiveKit server URL used for the voice-call session.                     |

A fully decked-out embed looks like:

```html
<raven-chat
  api-key="rvn_live_your_key_here"
  api-url="https://app.example.com"
  theme-color="#0ea5e9"
  avatar-url="https://www.example.com/img/support-avatar.png"
  welcome-text="Hi there — ask me anything about Example."
  position="bottom-right"
  voice-enabled="true"
  livekit-url="wss://livekit.example.com"
></raven-chat>
```

## Styling

The widget renders inside an **open Shadow DOM**, so your site's stylesheet
cannot accidentally leak into it (and vice-versa). To tweak the appearance,
either:

1. **Use `theme-color`** for the simplest case — it derives a matching hover
   shade automatically.
2. **Override CSS custom properties** on the host element. Because the shadow
   root is open, inherited custom properties flow through. Add a global rule
   on the host:

   ```css
   raven-chat {
     --rc-primary: #0ea5e9;
     --rc-bg: #0b1220;
     --rc-bg-secondary: #111a2d;
     --rc-text: #e6edf3;
     --rc-text-muted: #94a3b8;
     --rc-border: #1f2937;
     --rc-radius: 16px;
     --rc-shadow: 0 10px 30px rgba(0, 0, 0, 0.35);
   }
   ```

The variables read by the shadow stylesheet are: `--rc-primary`,
`--rc-primary-hover`, `--rc-bg`, `--rc-bg-secondary`, `--rc-text`,
`--rc-text-muted`, `--rc-border`, `--rc-shadow`, `--rc-radius`, and `--rc-font`.

The layout is mobile-first: on viewports narrower than the panel width, the
launcher anchors to the screen edge and the expanded panel docks full-width.
You do not need to add any media queries.

## Security

### Public, not secret

The widget API key is a **public credential** — it lives in your page's HTML
and ships to every visitor. It is safe to commit it to your website's source
because it is constrained two ways:

- It is **scoped to one knowledge base**. It cannot read other KBs, list
  documents, manage settings, or touch any admin surface.
- It is **revocable**. Hit
  `DELETE /api/v1/orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/api-keys/{key_id}`
  (or click **Revoke** in the dashboard) and the key stops working immediately.

If you need to call Raven server-to-server with broader privileges, do not use
a widget key — use a workspace session token issued by SuperTokens or a
service account.

### Domain allowlist

The `allowed_domains` array you provided at creation is enforced server-side.
A request whose `Origin` header is not in the list is rejected before the
chat handler runs.

The check runs in two complementary places:

1. **CORS middleware** (browser-facing). When a request carries
   `X-API-Key`, the global CORS layer (`internal/middleware/cors.go`)
   compares the request's `Origin` against that key's `allowed_domains`
   in addition to the workspace-level `RAVEN_CORS_ALLOWED_ORIGINS` list.
   If the key has a non-empty `allowed_domains`, the server-wide allowlist
   does **not** widen it — a widget key bound to `example.com` is unusable
   from any other origin even if the workspace allowlist is broader. If the
   key's `allowed_domains` is empty, the workspace-level list applies as
   before.
2. **API-key auth** (`internal/middleware/apikey.go`). After CORS, the
   API-key middleware re-runs the same check against `Origin` (falling
   back to `Referer` for non-browser callers) and returns
   `403 domain_not_allowed` if the host is not in the per-key list.

### Wildcard syntax

Each entry is matched against the **host** part of the `Origin` URL.
Exact hostnames (`example.com`, `app.example.com`) match literally.
A single leading-wildcard form is supported:

| Entry           | Matches                                              |
|-----------------|------------------------------------------------------|
| `example.com`   | `example.com` only                                   |
| `*.example.com` | `example.com` (apex), `www.example.com`, `a.b.example.com` (any depth) |

Wildcards in any other position (e.g. `foo.*.example.com`,
`example.*`) are **not** supported — the entry is treated as a literal
hostname and will never match.

### Rate limiting

Set `rate_limit` (requests per minute) on the key to cap how chatty a single
embed can be. This is enforced per key, not per visitor.

## CORS configuration on the server

When the widget runs on `https://www.example.com` and calls
`https://app.example.com`, the browser issues a cross-origin request. Your
Raven API must include the embedding origin in its CORS allowlist or the
browser will block the response.

Set the env var on the API container:

```bash
RAVEN_CORS_ALLOWED_ORIGINS=https://www.example.com,https://example.com
```

You can pass multiple origins as a comma-separated list. The middleware
matches **exactly** — protocol, host, and (non-default) port all have to
match. `https://example.com` does not cover `https://www.example.com`; list
both explicitly. There is no wildcard support.

The same value can be set via config file under `cors.allowed_origins` in
`raven.yaml`.

## Local development

For end-to-end development on your laptop:

1. Start the Raven API on port `8080`.
2. Start the frontend dev server (`npm --prefix frontend run dev`) on port
   `3000`. The Vite proxy forwards `/api` to `http://localhost:8080`.
3. Open a sandbox HTML page — see
   `frontend/e2e/chat-widget/widget-sandbox.html` for the exact shape:

   ```html
   <raven-chat
     api-key="test-api-key-from-env"
     api-url="http://localhost:8080"
   ></raven-chat>
   <script src="http://localhost:5173/chat-widget.js"></script>
   ```

4. Add the page's origin (`http://localhost:8081`, `file://`, or wherever you
   are serving it) to `RAVEN_CORS_ALLOWED_ORIGINS` before booting the API.

The Playwright integration tests in `frontend/e2e/chat-widget/widget.spec.ts`
load the sandbox HTML, type into the shadow-DOM input, and assert that the
assistant streams a response. Use them as a living reference if your embed
behaves differently from what you see in the dashboard preview.

## Troubleshooting

| Symptom                                                              | Likely cause                                                                                                       |
|----------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------|
| Launcher never appears; console shows `404` on `chat-widget.js`      | Wrong `src` on the `<script>` tag. The bundle is served from your Raven API origin, not from your marketing site. |
| Launcher appears but panel shows the **error state** on first send   | Invalid, revoked, or wrong-knowledge-base API key. Check `key_prefix` in the dashboard against your embed.        |
| Browser console: `CORS policy: No 'Access-Control-Allow-Origin'`     | The embedding origin is not in `RAVEN_CORS_ALLOWED_ORIGINS`. Add it and restart the API.                          |
| Replies say "no relevant context" no matter what you ask             | The KB has no documents, or ingestion hasn't completed. Check **Knowledge Bases → Documents** for green status.   |
| Widget renders but styles look unbranded                             | Custom property overrides are scoped too narrowly. Apply them directly to `raven-chat` at the root, not inside `:host {}` from your page CSS — `:host` is a shadow-internal selector.       |
| Voice call button missing                                            | `voice-enabled="true"` and `livekit-url="wss://…"` are both required, and the workspace must have LiveKit configured. See [Try the Voice Agent](/get-started/try-the-voice-agent). |

If `<raven-chat>` element appears in the DOM but is empty, the script
registered the element after it was parsed and a re-attach is needed. In
practice this only happens if the script tag is loaded `async` and parsed
late; mark it `type="module"` (which is `defer` by default) or place it after
the element.

## What's next

- [API Overview](/api/overview) — the full REST surface, including the API key
  management endpoints.
- [Workspaces & Tenancy](/guides/workspaces-and-tenancy) — how organisations,
  workspaces, and knowledge bases relate to API keys and CORS scoping.
- [Try the Voice Agent](/get-started/try-the-voice-agent) — unlock the
  `voice-enabled` attribute with a LiveKit-backed voice call.
