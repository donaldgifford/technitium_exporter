# R-010: No Tests for Config and Exporter Packages

| Field    | Value                  |
| -------- | ---------------------- |
| Severity | LOW                    |
| Category | Testing                |
| File     | `config/`, `exporter/` |
| Linter   | Test coverage          |

## Finding

Two packages have no test files:

```
?   github.com/donaldgifford/technitium_exporter/config   [no test files]
?   github.com/donaldgifford/technitium_exporter/exporter  [no test files]
```

### config package

`config/config.go` has logic worth testing:

- `ApplyEnvironment()` -- environment variable override behavior
- `Validate()` -- required field validation
- Interaction between flags and env vars (env takes precedence)
- Edge cases: empty strings, whitespace-only values
- `SCRAPE_TIMEOUT` parsing (related to R-004)

### exporter package

`exporter/exporter.go` has a single function:

- `LandingPageHandler()` -- returns HTML with the metrics path injected

This is simple but testable with `httptest.ResponseRecorder`.

## Impact

The `config` package has the higher risk. Misconfigured environment variable
handling could cause the exporter to connect to the wrong server or use a stale
token. The `exporter` package is low-risk since the landing page is cosmetic.

## Root Cause

Tests were prioritized for the collector and client packages (the core
functionality). Config and exporter were deferred.

## Proposed Solutions

### Option A: Add config tests (Recommended)

```go
func TestConfig_Validate(t *testing.T) {
    tests := []struct {
        name    string
        url     string
        token   string
        wantErr error
    }{
        {"valid", "http://localhost:5380", "token", nil},
        {"missing url", "", "token", ErrMissingURL},
        {"missing token", "http://localhost:5380", "", ErrMissingToken},
        {"both missing", "", "", ErrMissingURL},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cfg := &Config{
                TechnitiumURL:   tt.url,
                TechnitiumToken: tt.token,
            }
            err := cfg.Validate()
            if !errors.Is(err, tt.wantErr) {
                t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
            }
        })
    }
}

func TestConfig_ApplyEnvironment(t *testing.T) {
    t.Setenv("TECHNITIUM_URL", "http://override:5380")
    t.Setenv("TECHNITIUM_TOKEN", "override-token")

    cfg := &Config{
        TechnitiumURL:   "http://original:5380",
        TechnitiumToken: "original-token",
    }
    cfg.ApplyEnvironment()

    if cfg.TechnitiumURL != "http://override:5380" {
        t.Errorf("URL not overridden: %s", cfg.TechnitiumURL)
    }
    if cfg.TechnitiumToken != "override-token" {
        t.Errorf("Token not overridden: %s", cfg.TechnitiumToken)
    }
}
```

**Effort:** Low **Risk:** None

### Option B: Add exporter tests

```go
func TestLandingPageHandler(t *testing.T) {
    handler := LandingPageHandler("/metrics")
    req := httptest.NewRequest(http.MethodGet, "/", nil)
    rec := httptest.NewRecorder()

    handler(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("status = %d, want 200", rec.Code)
    }
    if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
        t.Errorf("content-type = %s, want text/html", ct)
    }
    if !strings.Contains(rec.Body.String(), "/metrics") {
        t.Error("response body missing metrics path")
    }
}
```

**Effort:** Low **Risk:** None

### Option C: Add both

Combine Options A and B.

**Effort:** Low **Risk:** None

## Recommendation

**Option C** -- add tests for both packages. The config tests have real value
for catching regressions in the env var override logic. The exporter test is
quick to write and brings the package to non-zero coverage.
