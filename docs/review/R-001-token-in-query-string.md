# R-001: Token Passed in URL Query String

| Field    | Value                         |
| -------- | ----------------------------- |
| Severity | HIGH                          |
| Category | Security                      |
| File     | `pkg/technitium/client.go:96` |
| Linter   | Manual review                 |

## Finding

The Technitium API token is passed as a URL query parameter:

```go
params.Set("token", c.token)
// ...
reqURL := c.baseURL + endpoint + "?" + params.Encode()
```

This results in requests like:

```
GET /api/dashboard/stats/get?token=SECRET&type=LastHour
```

Tokens in query strings appear in:

- HTTP server access logs
- Reverse proxy logs (nginx, caddy, traefik)
- Network monitoring tools
- Browser history (if accessed via browser)
- Referrer headers on redirects

## Impact

If the Technitium server sits behind a reverse proxy or any logging
infrastructure, the admin API token is recorded in plain text in log files.
Anyone with access to those logs gains full admin API access.

In practice, most Technitium deployments are on local networks (Raspberry Pi,
home lab), and the exporter communicates directly with the server over localhost
or LAN. The risk is real but bounded by deployment context.

## Root Cause

This is a Technitium API design constraint. The API only supports token-based
auth via query parameters. The exporter has no choice but to comply.

## Proposed Solutions

### Option A: Document the limitation (Recommended)

Add a note to the README and CLAUDE.md acknowledging the token-in-URL pattern.
No code changes needed since the exporter can't change how the Technitium API
works.

**Effort:** Minimal **Risk:** None

### Option B: Support header-based auth if/when available

Watch for Technitium API updates that add `Authorization` header support. If
added, implement header-based auth as the default with query parameter as
fallback.

```go
req.Header.Set("Authorization", "Bearer "+c.token)
```

**Effort:** Low (when API supports it) **Risk:** None

### Option C: Add HTTP-only transport flag

Add a config flag to restrict the client to HTTP-only (no HTTPS redirect
following) to prevent the token from leaking over TLS renegotiation or
cross-origin redirects. This is defense-in-depth.

**Effort:** Low **Risk:** Could break setups using HTTPS

## Recommendation

Go with **Option A** for now. This is a known limitation of the upstream API,
not a bug in the exporter. Document it so users can make informed deployment
decisions (e.g., restrict network access to the exporter, avoid exposing the
Technitium API through public reverse proxies).
