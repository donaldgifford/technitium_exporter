# R-003: goimports Local Prefix Points to Wrong Project

| Field    | Value               |
| -------- | ------------------- |
| Severity | MEDIUM              |
| Category | Idioms / Tooling    |
| File     | `.golangci.yml:250` |
| Linter   | Configuration issue |

## Finding

The `goimports` formatter is configured with a local prefix from a different
project:

```yaml
goimports:
  local-prefixes:
    - github.com/donaldgifford/tflint-ruleset-terraform-style
```

Should be:

```yaml
goimports:
  local-prefixes:
    - github.com/donaldgifford/technitium_exporter
```

## Impact

The `goimports` formatter is not enforcing the standard 3-group import ordering
for this project:

```go
import (
    // 1. Standard library
    "fmt"

    // 2. Third-party
    "github.com/prometheus/client_golang/prometheus"

    // 3. Local packages (this project)
    "github.com/donaldgifford/technitium_exporter/collector"
)
```

Without the correct prefix, local imports may be grouped with third-party
imports. The `gci` formatter (also enabled) partially compensates since it has
the correct prefix (`prefix(github.com/donaldgifford)`), but the two formatters
could conflict.

## Root Cause

The `.golangci.yml` was likely copied from the `tflint-ruleset-terraform-style`
project and the `goimports` local prefix wasn't updated.

## Proposed Solutions

### Option A: Fix the prefix (Recommended)

```yaml
goimports:
  local-prefixes:
    - github.com/donaldgifford/technitium_exporter
```

**Effort:** One-line change **Risk:** May reformat imports in existing files
(cosmetic diff)

### Option B: Remove goimports, rely on gci

Since both `goimports` and `gci` handle import ordering, and `gci` has the
correct prefix, you could remove `goimports` to avoid redundancy. However,
`goimports` also handles adding/removing imports, which `gci` does not.

**Effort:** One-line removal **Risk:** Loses auto-import functionality from the
formatter

## Recommendation

**Option A.** Fix the prefix. Run `golangci-lint fmt ./...` afterward to apply
any import reordering.
