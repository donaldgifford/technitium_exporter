# R-009: Test Helpers Don't Accept `*testing.T`

| Field    | Value                                             |
| -------- | ------------------------------------------------- |
| Severity | LOW                                               |
| Category | Testing                                           |
| File     | `collector/collector_test.go:22-24,102-114`       |
| Linter   | Manual review (thelper enabled but not triggered) |

## Finding

The test helper functions `newTestLogger()` and `newTestServer()` don't accept
`*testing.T`:

```go
func newTestLogger() *slog.Logger {
    return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestServer(statsJSON, settingsJSON string, statsCode int) *httptest.Server {
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // ...
    }))
}
```

If the handler inside `newTestServer` ever needs to call `t.Errorf` or `t.Fatal`
(e.g., for unexpected request paths), it can't. More importantly, if the helper
itself fails, the test failure would report the line inside the helper rather
than the line in the calling test.

## Impact

Currently low. The helpers are simple factory functions that don't fail. But if
assertions are added to the test server handler (e.g., verifying request
parameters), the `thelper` linter would require `t.Helper()` calls, which
requires `*testing.T` to be passed in.

For comparison, the `pkg/technitium/client_test.go` test server handlers already
use `t.Errorf` for path validation (line 73-74), but those are defined inline
within each test.

## Root Cause

The helpers were designed as pure factory functions without test assertions.
This is valid for their current use, but limits future extensibility.

## Proposed Solutions

### Option A: Add `*testing.T` parameter (Recommended)

```go
func newTestServer(t *testing.T, statsJSON, settingsJSON string, statsCode int) *httptest.Server {
    t.Helper()
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Can now use t.Errorf if needed
        switch {
        case strings.Contains(r.URL.Path, "/api/dashboard/stats/get"):
            w.WriteHeader(statsCode)
            _, _ = w.Write([]byte(statsJSON))
        case strings.Contains(r.URL.Path, "/api/settings/get"):
            _, _ = w.Write([]byte(settingsJSON))
        default:
            t.Errorf("unexpected request path: %s", r.URL.Path)
            w.WriteHeader(http.StatusNotFound)
        }
    }))
}
```

Also use `t.Cleanup` to automatically close the server:

```go
func newTestServer(t *testing.T, statsJSON, settingsJSON string, statsCode int) *httptest.Server {
    t.Helper()
    server := httptest.NewServer(...)
    t.Cleanup(server.Close)
    return server
}
```

This removes the need for `defer server.Close()` in every test.

**Effort:** Low (update helper + all call sites) **Risk:** None. Pure mechanical
refactor.

### Option B: Leave as-is

The helpers work as-is. The `thelper` linter doesn't flag them because they
don't take `*testing.T`. No functional issue exists today.

**Effort:** None **Risk:** None

## Recommendation

**Option A** if you're making changes to the test file anyway. The `t.Cleanup`
pattern is particularly nice as it eliminates `defer server.Close()` boilerplate
from every test. Otherwise **Option B** is fine -- this is not urgent.
