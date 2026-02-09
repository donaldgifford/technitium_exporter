# R-002: Direct Error Comparison Instead of `errors.Is`

| Field    | Value                                                    |
| -------- | -------------------------------------------------------- |
| Severity | HIGH                                                     |
| Category | Idioms                                                   |
| File     | `cmd/technitium_exporter/main.go:120`                    |
| Linter   | `errorlint` (configured but not triggering on `main.go`) |

## Finding

The `ListenAndServe` error is compared directly using `!=` instead of
`errors.Is`:

```go
if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
    logger.Error("Error starting HTTP server", "err", err)
    os.Exit(1)
}
```

Per Go 1.13+ conventions and the project's own `errorlint` config
(`comparison: true`), error comparisons should use `errors.Is` to handle wrapped
errors correctly:

```go
if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
```

## Impact

If `net/http` ever wraps `ErrServerClosed` (or if a middleware/listener wrapper
is introduced that wraps errors), the direct comparison would fail to match,
causing the exporter to log a spurious error and exit with code 1 on every
graceful shutdown.

Currently `ListenAndServe` returns the sentinel directly, so this works today.
But it's fragile and inconsistent with the project's own lint rules.

## Root Cause

Likely written before the `errorlint` config was added, or the linter isn't
catching it because `main.go` is excluded via the path rule for `gochecknoinits`
(though that rule is scoped to a different linter).

## Proposed Solutions

### Option A: Use `errors.Is` (Recommended)

```go
if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
    logger.Error("Error starting HTTP server", "err", err)
    os.Exit(1)
}
```

Requires adding `"errors"` to the import block.

**Effort:** One-line change **Risk:** None

### Option B: Leave as-is with a lint suppression

Add a `//nolint:errorlint` directive with explanation. Not recommended since the
fix is trivial.

**Effort:** One-line change **Risk:** Masks a real issue

## Recommendation

**Option A.** This is a trivial fix that aligns the code with the project's own
lint configuration and Go best practices.
