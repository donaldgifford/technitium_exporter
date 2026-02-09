# R-005: No Size Limit on Response Body Reads

| Field    | Value                            |
| -------- | -------------------------------- |
| Severity | MEDIUM                           |
| Category | Security / Performance           |
| File     | `pkg/technitium/client.go:46,77` |
| Linter   | Manual review                    |

## Finding

Both `GetStats` and `GetSettings` read the entire response body without a size
limit:

```go
body, err := io.ReadAll(resp.Body)
```

If the Technitium server (or a man-in-the-middle on the network) returns an
unexpectedly large response, `io.ReadAll` will allocate memory until the process
runs out or the OS kills it.

## Impact

In a normal operating environment, the Technitium API responses are small JSON
payloads (a few KB). The risk materializes in:

1. **Compromised server:** An attacker controlling the Technitium server could
   return a multi-GB response, causing the exporter to OOM.
2. **Bug in Technitium:** A server bug could produce an unexpectedly large
   response.
3. **Network MITM:** On unencrypted HTTP (common for local Technitium setups),
   an attacker on the LAN could inject a large response.

For a Prometheus exporter running on a Raspberry Pi with limited RAM, this is
more impactful than on a server with abundant memory.

## Root Cause

`io.ReadAll` is the standard Go pattern for reading HTTP responses, but it's
unbounded by design.

## Proposed Solutions

### Option A: Use `io.LimitReader` (Recommended)

Wrap the response body with a size limit. 1MB is generous for the expected JSON
payloads:

```go
const maxResponseSize = 1 << 20 // 1MB

body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
```

**Effort:** Two-line change (one per method, or once in `doRequest`) **Risk:**
None. The actual responses are a few KB. A 1MB limit has massive headroom.

### Option B: Apply the limit in `doRequest`

Set `http.MaxBytesReader` or apply `io.LimitReader` at the transport level in
`doRequest`, so all callers benefit automatically:

```go
resp.Body = http.MaxBytesReader(nil, resp.Body, maxResponseSize)
```

Note: `http.MaxBytesReader` is designed for server-side use and returns a
specific error type. `io.LimitReader` is simpler for client-side use.

**Effort:** One-line change in `doRequest` **Risk:** None

### Option C: Stream JSON decode

Instead of reading the entire body into memory, decode directly from the reader:

```go
var statsResp StatsResponse
if err := json.NewDecoder(resp.Body).Decode(&statsResp); err != nil {
    return nil, fmt.Errorf("failed to parse stats response: %w", err)
}
```

This avoids the intermediate `[]byte` allocation entirely. However,
`json.NewDecoder` still reads until EOF, so it doesn't solve the unbounded read
problem by itself. Combine with `io.LimitReader` for full protection.

**Effort:** Medium (refactor both methods) **Risk:** `json.NewDecoder` has
subtle behavior with multiple JSON values in a stream, though that's not
relevant here.

## Recommendation

**Option A** applied in both `GetStats` and `GetSettings`, or **Option B**
applied once in `doRequest`. Either is a minimal change with clear defensive
value.
