# R-006: URL Built via String Concatenation

| Field    | Value                         |
| -------- | ----------------------------- |
| Severity | MEDIUM                        |
| Category | Idioms / Robustness           |
| File     | `pkg/technitium/client.go:96` |
| Linter   | Manual review                 |

## Finding

The request URL is assembled via string concatenation:

```go
reqURL := c.baseURL + endpoint + "?" + params.Encode()
```

This is fragile if `baseURL` contains a trailing slash or unexpected path
components:

| `baseURL`                   | Result                                                                      |
| --------------------------- | --------------------------------------------------------------------------- |
| `http://localhost:5380`     | `http://localhost:5380/api/dashboard/stats/get?...` (correct)               |
| `http://localhost:5380/`    | `http://localhost:5380//api/dashboard/stats/get?...` (double slash)         |
| `http://localhost:5380/dns` | `http://localhost:5380/dns/api/dashboard/stats/get?...` (works but fragile) |

## Impact

A double slash (`//`) in the path is typically handled correctly by most HTTP
servers (they normalize it), so this is unlikely to cause a real failure.
However, it's not idiomatic Go and could cause issues with strict URL parsers or
proxies.

## Root Cause

Simple string concatenation was used for brevity. The `net/url` package provides
proper URL manipulation but adds verbosity.

## Proposed Solutions

### Option A: Use `url.JoinPath` (Recommended)

Go 1.19+ provides `url.JoinPath` which handles trailing slashes:

```go
func (c *Client) doRequest(ctx context.Context, endpoint string, params url.Values) (*http.Response, error) {
    reqURL, err := url.JoinPath(c.baseURL, endpoint)
    if err != nil {
        return nil, fmt.Errorf("failed to build URL: %w", err)
    }
    reqURL += "?" + params.Encode()
    // ...
}
```

**Effort:** Low **Risk:** None. `url.JoinPath` has been stable since Go 1.19.

### Option B: Parse and construct with `net/url`

```go
u, err := url.Parse(c.baseURL)
if err != nil {
    return nil, fmt.Errorf("invalid base URL: %w", err)
}
u.Path = path.Join(u.Path, endpoint)
u.RawQuery = params.Encode()
reqURL := u.String()
```

**Effort:** Medium **Risk:** `path.Join` can strip trailing slashes, which may
matter for some APIs (not Technitium).

### Option C: Normalize baseURL in NewClient

Strip trailing slashes from `baseURL` at construction time:

```go
func NewClient(baseURL, token string, timeout time.Duration) *Client {
    return &Client{
        baseURL: strings.TrimRight(baseURL, "/"),
        // ...
    }
}
```

This is the simplest fix but only addresses the trailing slash case, not
malformed URLs.

**Effort:** One-line change **Risk:** None

## Recommendation

**Option A** is the most idiomatic and handles edge cases properly. **Option C**
is acceptable as a minimal fix if you want to keep the change small.
