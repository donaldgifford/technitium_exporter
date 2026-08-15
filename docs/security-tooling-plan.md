# Security Tooling Integration Plan

## Context

The project needs security scanning beyond what gosec (already in golangci-lint)
provides. We agreed on four tools:

- **gosec** -- Already integrated via golangci-lint (no changes needed)
- **govulncheck** -- Go source-level vulnerability analysis (call-graph aware)
- **trivy** -- Dependency CVE scanning (and future container image scanning)
- **syft** -- SBOM generation for release artifacts

This plan adds local `make` targets for developer workflow, CI jobs for PR
gating, and release-time SBOM generation.

## Files to Modify

| File                            | Change                                    | Status |
| ------------------------------- | ----------------------------------------- | ------ |
| `mise.toml`                     | Add govulncheck, trivy, syft              | Done   |
| `Makefile`                      | Add `##@ Security` section with 4 targets | Done   |
| `.github/workflows/test.yml`    | Add `security` job (govulncheck + trivy)  | Done   |
| `.goreleaser.yml`               | Add `sboms` section                       | Done   |
| `.github/workflows/release.yml` | Add syft install step before goreleaser   | Done   |

## Step 1: mise.toml -- Done

Added three tools to `[tools]` section:

```toml
"go:golang.org/x/vuln/cmd/govulncheck" = "latest"
trivy = "latest"
syft = "latest"
```

Verified: `mise install` succeeded, all three tools available via mise.

## Step 2: Makefile -- Done

Added `##@ Security` section with targets:

| Target        | Purpose                                                                                             |
| ------------- | --------------------------------------------------------------------------------------------------- |
| `govulncheck` | Source-level Go vulnerability check (`govulncheck ./...`)                                           |
| `trivy`       | Dependency CVE scan (`trivy fs --scanners vuln --exit-code 1 --severity HIGH,CRITICAL .`)           |
| `syft`        | Generate SPDX + CycloneDX SBOMs to `build/`                                                         |
| `security`    | Aggregate: runs `govulncheck` then `trivy` (not `syft` -- SBOM is artifact generation, not a check) |

Also updated `clean` to remove SBOM files.

Verified: `make trivy` and `make syft` pass. `make govulncheck` correctly
reports Go 1.25.4 stdlib CVEs.

## Step 3: .github/workflows/test.yml -- Done

Added `security` job running in parallel with existing jobs:

- `golang/govulncheck-action@v1` for source-level vuln analysis
- `aquasecurity/trivy-action@0.33.1` for dependency CVE scanning (HIGH/CRITICAL,
  exit-code 1)

## Step 4: .goreleaser.yml -- Done

Added `sboms` section after `archives`:

```yaml
sboms:
  - artifacts: archive
    documents:
      - "{{ .ArtifactName }}.sbom.spdx.json"
```

Verified: `make release-local` produces SPDX JSON SBOMs for each archive in
`dist/`.

## Step 5: .github/workflows/release.yml -- Done

Added `anchore/sbom-action/download-syft@v0` step before GoReleaser in the
release job.

## Explicitly Deferred

- **Container image scanning** -- Dockerfile is empty. When Docker images are
  built, add `trivy image` scanning and `make trivy-image` target.
- **SARIF upload to GitHub Security tab** -- Both tools support it but requires
  `security-events: write` permission. Clean follow-up.
- **trivy misconfiguration/secret scanning** -- Enable when Dockerfiles or IaC
  configs are added.
- **Signed SBOMs / cosign attestations** -- Supply chain hardening follow-up.

## Verification

- [x] `mise install` -- govulncheck, trivy, syft install
- [x] `make govulncheck` -- reports Go stdlib vulns (Go 1.25.4)
- [x] `make trivy` -- scans go.mod, 0 HIGH/CRITICAL dependency CVEs
- [x] `make syft` -- produces `build/sbom.spdx.json` and `build/sbom.cdx.json`
- [x] `make release-local` -- produces SBOM files in `dist/` alongside archives
- [ ] Push to branch and verify `security` job passes in CI
