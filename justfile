# technitium_exporter — task runner
#
# Project automation via just. Run `just` with no arguments for the menu.

set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

# Replaces the Makefile's `-include .env`. Recipes that talk to a live
# Technitium server read TECHNITIUM_URL / TECHNITIUM_TOKEN from the
# environment; .env is gitignored and is the only place credentials belong.
# Missing .env is not an error -- recipes that need it say so themselves.
set dotenv-load := true

# Docker recipes live in their own file; without this import none of the
# docker-* recipes exist at all.
import 'docker.just'

project_name      := "technitium_exporter"
project_owner     := "donaldgifford"
go_package        := "github.com/" + project_owner + "/" + project_name
build_dir         := "build"
bin_dir           := build_dir + "/bin"
coverage_out      := "coverage.out"
coverage_floor    := "60"

# Packages the coverage floor applies to, relative to the module root.
# Deliberately not "everything": config/ and exporter/ sit at 0% pending issue
# #15, and cmd/technitium_exporter is a thin main wired together only at
# runtime. Add packages here as they gain tests -- never lower the floor to
# accommodate an untested one.
#
# This list is independent of .codecov.yml, which ignores "main.go", "docs",
# and "scripts". Keeping them separate is deliberate: codecov reports on the
# whole module, this gate is the hard fail on the packages that carry logic.
gated_packages    := "collector pkg/technitium"
allowed_licenses  := "Apache-2.0,MIT,BSD-2-Clause,BSD-3-Clause,ISC,MPL-2.0"

# Version info derived from git; falls back to dev when not in a repo or tag-less.
commit_hash := `git rev-parse --short HEAD 2>/dev/null || echo unknown`
version     := `git describe --tags --always --dirty 2>/dev/null || echo dev`
build_date  := `date -u +%Y-%m-%dT%H:%M:%SZ`

# Default: list recipes
_default:
    @just --list --unsorted

# ─── Build ──────────────────────────────────────────────────────────

# Build the CLI binary into build/bin/technitium_exporter
[group('build')]
build:
    @# Symbol names must match the vars in cmd/technitium_exporter/main.go
    @# (Version/Commit/BuildDate, capitalised). The linker silently ignores -X
    @# for a symbol it cannot resolve, so a typo here fails open: the build
    @# succeeds and the binary reports "dev" forever.
    @mkdir -p {{ bin_dir }}
    @go build -ldflags "-X main.Version={{ version }} -X main.Commit={{ commit_hash }} -X main.BuildDate={{ build_date }}" \
        -o {{ bin_dir }}/{{ project_name }} ./cmd/{{ project_name }}
    @# Report what the binary actually says, not what `just` thinks it passed --
    @# a wrong -X symbol is silently dropped by the linker, so echoing the just
    @# variable here would have hidden exactly the bug this recipe used to have.
    @# kingpin writes --version to stderr, hence the redirect.
    @echo "✓ Built {{ bin_dir }}/{{ project_name }} ($({{ bin_dir }}/{{ project_name }} --version 2>&1))"

# Remove build artifacts, release output, and the coverage profile
[group('build')]
clean:
    @rm -rf {{ build_dir }}/ dist/
    @rm -f {{ coverage_out }}
    @find . -name "*.test" -delete
    @echo "✓ Cleaned build artifacts"

# ─── Run ────────────────────────────────────────────────────────────

# Build then run the exporter: just run --web.listen-address=:9167
[group('run')]
run *ARGS: build
    @{{ bin_dir }}/{{ project_name }} {{ ARGS }}

# Build then run the exporter against a live server (needs .env or exported creds)
[group('run')]
run-local: _require-technitium build
    @{{ bin_dir }}/{{ project_name }}

# Probe the Technitium API directly: settings, then last-hour stats, through jq
[group('run')]
test-api: _require-technitium
    #!/usr/bin/env bash
    set -euo pipefail
    # -f so an HTTP error status fails the recipe instead of piping an error
    # body into jq and exiting 0. The token travels in the query string
    # because that is the only auth the Technitium API offers -- see issue #6.
    curl -sf "${TECHNITIUM_URL}/api/settings/get?token=${TECHNITIUM_TOKEN}" | jq .
    curl -sf "${TECHNITIUM_URL}/api/dashboard/stats/get?token=${TECHNITIUM_TOKEN}&type=LastHour" | jq .

# Fail with instructions unless Technitium credentials are present.
[private]
_require-technitium:
    #!/usr/bin/env bash
    # `set -u` from the shell setting would abort on the unset variable before
    # the check could run, hence the :- defaults.
    set -euo pipefail
    if [ -z "${TECHNITIUM_URL:-}" ] || [ -z "${TECHNITIUM_TOKEN:-}" ]; then
      echo "✗ TECHNITIUM_URL and TECHNITIUM_TOKEN must be set." >&2
      echo "  Put them in a gitignored .env at the repo root (loaded" >&2
      echo "  automatically), or export them in your shell:" >&2
      echo "    TECHNITIUM_URL=http://dns.example.com:5380" >&2
      echo "    TECHNITIUM_TOKEN=<api-token>" >&2
      exit 1
    fi

# ─── Test ───────────────────────────────────────────────────────────

# Run all tests with the race detector
[group('test')]
test:
    @go test -race ./...

# Run tests for a single package: just test-pkg ./internal/mirror
[group('test')]
test-pkg pkg:
    @go test -v -race {{ pkg }}

# Run integration tests (//go:build integration), which need external services
[group('test')]
test-integration:
    @go test -race -tags=integration ./...

# Run tests with a coverage profile written to coverage.out
[group('test')]
test-coverage:
    @go test -race -coverprofile={{ coverage_out }} ./...

# Fail if a gated package is under the coverage floor (needs coverage.out)
[group('test')]
coverage-gate:
    #!/usr/bin/env bash
    # Reads an existing coverage.out (run `just test-coverage` first) so CI does
    # not pay for a second test run. Which packages are gated is the
    # `gated_packages` variable at the top of this file, not a path prefix --
    # the previous version matched on "internal/", a directory this repo has
    # never had, so the gate reported "nothing to gate" and exited 0 no matter
    # how bad coverage got.
    #
    # A package named in gated_packages but absent from coverage.out is a
    # failure, not a skip. That is the same fail-open trap: a rename or a typo
    # would otherwise silently retire the gate.
    set -euo pipefail
    if [ ! -f {{ coverage_out }} ]; then
      echo "✗ {{ coverage_out }} not found — run 'just test-coverage' first" >&2
      exit 1
    fi
    awk -v floor="{{ coverage_floor }}" \
        -v module="{{ go_package }}" \
        -v gated="{{ gated_packages }}" '
      BEGIN { ngated = split(gated, want, " ") }
      NR == 1 { next }                     # skip the "mode:" header
      {
        split($1, loc, ":")                # pkg/path/file.go:12.3,14.4
        n = split(loc[1], part, "/")
        pkg = part[1]
        for (i = 2; i < n; i++) pkg = pkg "/" part[i]
        total[pkg] += $2                   # statements in this block
        if ($3 > 0) hit[pkg] += $2         # ...that were executed
      }
      END {
        failed = 0
        for (i = 1; i <= ngated; i++) {
          pkg = module "/" want[i]
          if (!(pkg in total)) {
            printf "  ✗ %s: no coverage data — renamed, or stale in gated_packages?\n", pkg
            failed = 1
            continue
          }
          pct = total[pkg] > 0 ? 100 * hit[pkg] / total[pkg] : 100
          if (pct + 0.0001 < floor) {
            printf "  ✗ %s: %.1f%% (floor %d%%)\n", pkg, pct, floor
            failed = 1
          } else {
            printf "  ✓ %s: %.1f%%\n", pkg, pct
          }
        }
        exit failed
      }
    ' {{ coverage_out }}
    echo "✓ Coverage gate passed (floor {{ coverage_floor }}%)"

# Run tests and open the HTML coverage report
[group('test')]
test-report:
    @go test -coverprofile={{ coverage_out }} ./...
    @go tool cover -html={{ coverage_out }}

# ─── Lint ───────────────────────────────────────────────────────────

# Run every linter: Go, YAML, Markdown, GitHub Actions
[group('lint')]
lint: lint-go lint-yaml lint-md lint-actions
    @echo "✓ All linters passed"

# Run golangci-lint
[group('lint')]
lint-go:
    @golangci-lint run ./...

# Run golangci-lint with --fix
[group('lint')]
lint-fix:
    @golangci-lint run --fix ./...

# Verify the golangci-lint configuration
[group('lint')]
lint-config:
    @golangci-lint config verify

# Lint YAML files
[group('lint')]
lint-yaml:
    @yamllint .

# Lint Markdown (markdownlint + prettier formatting check)
[group('lint')]
lint-md:
    @markdownlint-cli2 "**/*.md"
    @prettier --check "**/*.md"

# Lint GitHub Actions workflows
[group('lint')]
lint-actions:
    @actionlint

# ─── Format ─────────────────────────────────────────────────────────

# Format everything: Go, YAML, Markdown
[group('lint')]
fmt: fmt-go fmt-yaml fmt-md
    @echo "✓ Formatted"

# Format Go code with the formatters configured in .golangci.yml
[group('lint')]
fmt-go:
    @# gofmt, goimports, gci, gofumpt, golines — driven by .golangci.yml so
    @# `just fmt-go` and `just lint-go` can never disagree about formatting.
    @golangci-lint fmt ./...

# Format YAML files
[group('lint')]
fmt-yaml:
    @yamlfmt .

# Format Markdown (CHANGELOG.md is excluded via .prettierignore)
[group('lint')]
fmt-md:
    @prettier --write "**/*.md"

# ─── Security & compliance ──────────────────────────────────────────

# Scan for known vulnerabilities in the dependency tree
[group('security')]
govulncheck:
    @govulncheck ./...

# Scan the dependency tree for HIGH/CRITICAL CVEs
[group('security')]
trivy:
    @trivy fs --scanners vuln --exit-code 1 --severity HIGH,CRITICAL .

# Run every security scanner: govulncheck (call-graph aware) + trivy (CVEs)
[group('security')]
security: govulncheck trivy
    @echo "✓ All security checks passed"

# Generate SPDX + CycloneDX SBOMs into build/
[group('security')]
syft:
    @# Deliberately not part of `just security`: an SBOM is an artifact to
    @# publish, not a check that can pass or fail.
    @mkdir -p {{ build_dir }}
    @syft dir:. --output spdx-json={{ build_dir }}/sbom.spdx.json \
        --output cyclonedx-json={{ build_dir }}/sbom.cdx.json
    @echo "✓ SBOMs generated in {{ build_dir }}/"

# Check dependency licenses against the allow list
[group('security')]
license-check:
    @go-licenses check ./... --allowed_licenses={{ allowed_licenses }}

# Generate CSV report of all dependency licenses
[group('security')]
license-report:
    @go-licenses report ./... --template=.github/licenses-csv.tpl

# ─── Changelog ──────────────────────────────────────────────────────

# Regenerate CHANGELOG.md from conventional commits (never hand-edit it)
[group('changelog')]
changelog:
    @git-cliff -o CHANGELOG.md
    @echo "✓ CHANGELOG.md regenerated"

# Mirror of CI's drift check: fail if CHANGELOG.md is stale
[group('changelog')]
changelog-check:
    #!/usr/bin/env bash
    set -euo pipefail
    trap 'rm -f CHANGELOG.check.md' EXIT
    git-cliff -o CHANGELOG.check.md 2>/dev/null
    if ! diff -q CHANGELOG.md CHANGELOG.check.md >/dev/null; then
      echo "✗ CHANGELOG.md is stale — run 'just changelog' and commit" >&2
      diff CHANGELOG.md CHANGELOG.check.md || true
      exit 1
    fi
    echo "✓ CHANGELOG.md is up to date"

# ─── Repo admin ─────────────────────────────────────────────────────

# Sync GitHub labels from labeler.yml + pr-labels.yml: just labels --dry-run
[group('repo')]
labels *ARGS:
    @./scripts/labels.sh {{ ARGS }}

# ─── Release ────────────────────────────────────────────────────────

# Validate the goreleaser config
[group('release')]
release-check:
    @goreleaser check

# Snapshot release locally (no publish, no sign)
[group('release')]
release-local:
    @goreleaser release --snapshot --clean --skip=publish --skip=sign

# Tag and push a new release: just release v0.1.0
[group('release')]
release tag:
    @git tag -a {{ tag }} -m "Release {{ tag }}"
    @git push origin {{ tag }}

# ─── Composite gates ────────────────────────────────────────────────

# Pre-commit gate: lint + test
[group('gate')]
check: lint test
    @echo "✓ Pre-commit checks passed"

# Full CI gate: everything CI runs, locally
[group('gate')]
ci: lint test-coverage coverage-gate build govulncheck license-check changelog-check
    @echo "✓ CI pipeline complete"
