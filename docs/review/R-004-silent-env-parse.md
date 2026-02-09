# R-004: Silent Failure on Invalid SCRAPE_TIMEOUT

| Field    | Value                    |
| -------- | ------------------------ |
| Severity | MEDIUM                   |
| Category | Error Handling           |
| File     | `config/config.go:63-66` |
| Linter   | Manual review            |

## Finding

When `SCRAPE_TIMEOUT` is set to an invalid duration string, the error is
silently discarded:

```go
if timeout := os.Getenv("SCRAPE_TIMEOUT"); timeout != "" {
    if d, err := time.ParseDuration(timeout); err == nil {
        c.ScrapeTimeout = d
    }
}
```

If a user sets `SCRAPE_TIMEOUT=30` (missing unit) or `SCRAPE_TIMEOUT=banana`,
the exporter silently falls back to the default `10s` from the kingpin flag. The
user has no indication their configuration was ignored.

## Impact

A user debugging scrape timeout issues would have no feedback that their
environment variable is malformed. This is especially confusing because the
other environment variables (`TECHNITIUM_URL`, `TECHNITIUM_TOKEN`) are validated
via `Validate()` but `SCRAPE_TIMEOUT` is not.

Common mistake: `SCRAPE_TIMEOUT=30` instead of `SCRAPE_TIMEOUT=30s`. Go's
`time.ParseDuration` requires a unit suffix.

## Root Cause

The `ApplyEnvironment` method doesn't return errors, so parse failures have
nowhere to go. The method was designed as a fire-and-forget override step.

## Proposed Solutions

### Option A: Return an error from ApplyEnvironment (Recommended)

Change `ApplyEnvironment` to return `error`:

```go
func (c *Config) ApplyEnvironment() error {
    // ... other env vars ...
    if timeout := os.Getenv("SCRAPE_TIMEOUT"); timeout != "" {
        d, err := time.ParseDuration(timeout)
        if err != nil {
            return fmt.Errorf("invalid SCRAPE_TIMEOUT %q: %w", timeout, err)
        }
        c.ScrapeTimeout = d
    }
    return nil
}
```

Update `main.go` to check the error:

```go
if err := cfg.ApplyEnvironment(); err != nil {
    logger.Error("Configuration error", "err", err)
    os.Exit(1)
}
```

**Effort:** Low (two files) **Risk:** Breaking change if anyone calls
`ApplyEnvironment` without checking the error. Since it's only called in
`main.go`, this is safe.

### Option B: Log a warning

Keep the current signature but log when parsing fails:

```go
if timeout := os.Getenv("SCRAPE_TIMEOUT"); timeout != "" {
    if d, err := time.ParseDuration(timeout); err == nil {
        c.ScrapeTimeout = d
    } else {
        slog.Warn("Invalid SCRAPE_TIMEOUT, using default", "value", timeout, "err", err)
    }
}
```

This requires passing a logger to `ApplyEnvironment` or using the default `slog`
logger.

**Effort:** Low **Risk:** Exporter still starts with a potentially unintended
timeout

### Option C: Move parsing into Validate

Move all environment variable validation into `Validate()` so all config errors
are caught in one place.

**Effort:** Medium (refactor) **Risk:** Changes the config initialization flow

## Recommendation

**Option A.** Failing fast on bad config is the safer default for an
infrastructure component. Users should know immediately if their environment is
misconfigured.
