---
id: INV-0001
title: "Forge-registry template migration review"
status: Concluded
author: Donald Gifford
created: 2026-08-02
---

<!-- markdownlint-disable-file MD025 MD041 MD013 -->

# INV 0001: Forge-registry template migration review

**Status:** Concluded **Author:** Donald Gifford **Date:** 2026-08-02

<!--toc:start-->

- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Environment](#environment)
- [Findings](#findings)
  - [H-1. Live Technitium API token committed in Makefile and in git history](#h-1-live-technitium-api-token-committed-in-makefile-and-in-git-history)
  - [H-2. just build and the Docker image never set version info — wrong ldflags symbols](#h-2-just-build-and-the-docker-image-never-set-version-info--wrong-ldflags-symbols)
  - [H-3. Dockerfile $$ escaping expands to the shell PID](#h-3-dockerfile--escaping-expands-to-the-shell-pid)
  - [H-4. docker-bake.hcl never passes the build args at all](#h-4-docker-bakehcl-never-passes-the-build-args-at-all)
  - [H-5. docker.just is never imported — every docker recipe is unreachable](#h-5-dockerjust-is-never-imported--every-docker-recipe-is-unreachable)
  - [H-6. docker buildx bake with no target fails — no default group](#h-6-docker-buildx-bake-with-no-target-fails--no-default-group)
  - [H-7. CHANGELOG.md is stale — the drift check fails right now](#h-7-changelogmd-is-stale--the-drift-check-fails-right-now)
  - [H-8. just lint-md fails — the markdownlint config is never loaded](#h-8-just-lint-md-fails--the-markdownlint-config-is-never-loaded)
  - [M-1. The coverage gate is a no-op — it gates internal/, which does not exist](#m-1-the-coverage-gate-is-a-no-op--it-gates-internal-which-does-not-exist)
  - [M-2. cliff.toml renders PR links literally — $${2} is over-escaped](#m-2-clifftoml-renders-pr-links-literally--2-is-over-escaped)
  - [M-3. yamllint ignores .github/ entirely](#m-3-yamllint-ignores-github-entirely)
  - [M-4. golangci-lint version drift: CI v2.11.4, mise 2.12.2](#m-4-golangci-lint-version-drift-ci-v2114-mise-2122)
  - [M-5. Two changelog systems coexist](#m-5-two-changelog-systems-coexist)
  - [M-6. ci.yml runs goreleaser twice, on mismatched action versions, one shallow](#m-6-ciyml-runs-goreleaser-twice-on-mismatched-action-versions-one-shallow)
  - [M-7. ci.yml labeler job: mislabeled step, and fork PRs will fail](#m-7-ciyml-labeler-job-mislabeled-step-and-fork-prs-will-fail)
  - [M-8. security.yml invokes govulncheck-action with no checkout](#m-8-securityyml-invokes-govulncheck-action-with-no-checkout)
  - [M-9. No concurrency groups on any workflow](#m-9-no-concurrency-groups-on-any-workflow)
  - [M-10. license-check.yml installs go-licenses unpinned](#m-10-license-checkyml-installs-go-licenses-unpinned)
  - [L-1. .gitignore has an unsubstituted template placeholder](#l-1-gitignore-has-an-unsubstituted-template-placeholder)
  - [L-2. .golangci.yml carries exclusions for dependencies this project lacks](#l-2-golangciyml-carries-exclusions-for-dependencies-this-project-lacks)
  - [L-3. mise.toml installs unused tooling](#l-3-misetoml-installs-unused-tooling)
  - [L-4. catalog-info.yaml: title typo and a TechDocs ref that cannot resolve](#l-4-catalog-infoyaml-title-typo-and-a-techdocs-ref-that-cannot-resolve)
  - [L-5. CODEOWNERS still carries its template instruction](#l-5-codeowners-still-carries-its-template-instruction)
  - [L-6. release.yml: duplicate syft install, and a comment about jobs that do not exist](#l-6-releaseyml-duplicate-syft-install-and-a-comment-about-jobs-that-do-not-exist)
  - [L-7. ghcr.yml: stale cross-repo comment and wrong schema directive](#l-7-ghcryml-stale-cross-repo-comment-and-wrong-schema-directive)
  - [L-8. .github/labeler.yml: a duplicate and several dead globs](#l-8-githublabeleryml-a-duplicate-and-several-dead-globs)
  - [L-9. scripts/labels.sh dropped labels that labeler.yml still emits](#l-9-scriptslabelssh-dropped-labels-that-labeleryml-still-emits)
  - [L-10. justfile run recipe: donor-repo doc comment, and missing env](#l-10-justfile-run-recipe-donor-repo-doc-comment-and-missing-env)
  - [L-11. .claude/ is untracked and not ignored](#l-11-claude-is-untracked-and-not-ignored)
  - [L-12. CLAUDE.md is now materially out of date](#l-12-claudemd-is-now-materially-out-of-date)
  - [What is working](#what-is-working)
- [Make to just migration](#make-to-just-migration)
  - [Missing — must port before deleting Makefile](#missing--must-port-before-deleting-makefile)
  - [Intentionally dropped — no action needed](#intentionally-dropped--no-action-needed)
  - [Already ported](#already-ported)
  - [New capability with no Makefile ancestor](#new-capability-with-no-makefile-ancestor)
  - [Deletion order](#deletion-order)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
- [References](#references)
<!--toc:end-->

## Question

Two questions, on the `chore/update-deps` branch that realigns this repo with
the forge-registry Go+Docker template:

1. Does the migrated tooling actually work, and did anything break or arrive
   half-wired?
2. Which `Makefile` targets still have no `justfile` equivalent, i.e. what
   blocks deleting `Makefile`?

## Hypothesis

A template port of this size lands mostly clean, but leaks two classes of
defect: **placeholders the template never substituted** (paths, variable names,
project-specific values from the donor repo) and **wiring gaps** where a new
file exists but nothing references it. Expected these to cluster in the Docker
and changelog layers, since both are entirely new to this repo.

Both classes were confirmed, plus one pre-existing secret exposure surfaced
while reading `Makefile`.

## Context

**Triggered by:** review request on branch `chore/update-deps` (all changes
still uncommitted in the working tree).

The branch adds 11 GitHub Actions workflows, a `justfile` + `docker.just`,
`Dockerfile` + `docker-bake.hcl` + `.dockerignore`, git-cliff changelog tooling,
Renovate, Backstage catalog metadata, and docz scaffolding — while leaving the
original `Makefile` in place.

## Approach

1. Diff every modified tracked file against `HEAD`; read every untracked
   addition in full.
2. Execute each `justfile` recipe that is safe to run locally and record real
   exit codes rather than reading them for intent.
3. Reproduce the Docker build-arg and bake-target behaviour with minimal
   throwaway builds.
4. Verify every pinned GitHub Action tag resolves, via the GitHub API.
5. Diff the `Makefile` target list against the `justfile` recipe list.

## Environment

| Component              | Version / Value                                |
| ---------------------- | ---------------------------------------------- |
| Branch                 | `chore/update-deps` (uncommitted working tree) |
| Go (go.mod)            | 1.26.5                                         |
| just                   | 1.51.0                                         |
| golangci-lint (mise)   | 2.12.2                                         |
| golangci-lint (CI pin) | v2.11.4                                        |
| git-cliff              | mise `latest`                                  |
| Docker                 | buildx bake, local daemon                      |

## Findings

Severity: **H** = breaks or misleads today, **M** = works but wrong or fragile,
**L** = cleanup. "Verified" means the behaviour was reproduced, not inferred.

### H-1. Live Technitium API token committed in `Makefile` and in git history

`Makefile:36-37` hardcodes a real server address and API token:

```makefile
TECHNITIUM_URL := "http://<internal-dns-host>:5380"
TECHNITIUM_TOKEN := "<64-hex-char-api-token>"
```

Verified present in history (`git log -S`) at commit `00d895e "testing the ci"`,
so deleting the file does not remove it. The new `trufflehog.yml` will not catch
this — TruffleHog has no detector for Technitium tokens, and
`--results=verified,unknown` still relies on a detector firing.

Rotate the token on the DNS server, then purge from history. This is independent
of the just migration and should not wait for it.

### H-2. `just build` and the Docker image never set version info — wrong ldflags symbols

`cmd/technitium_exporter/main.go:25-29` declares:

```go
Version   = "dev"
Commit    = "none"
BuildDate = "unknown"
```

`.goreleaser.yml:10` targets these correctly (`main.Version`, `main.Commit`,
`main.BuildDate`). But `justfile:31` and `Dockerfile:16` both target
`main.version`, `main.commit`, `main.date` — lowercase. The Go linker silently
ignores `-X` for symbols it cannot resolve, so the flags are dropped without
error.

Verified — the recipe prints a version it did not actually inject:

```text
$ just build
✓ Built build/bin/technitium_exporter (v0.3.0-dirty)
$ ./build/bin/technitium_exporter --version
dev (commit: none, built: unknown)
```

Release binaries are fine (goreleaser is correct); local and container builds
are not. The success message actively hides it.

### H-3. `Dockerfile` `$$` escaping expands to the shell PID

`Dockerfile:16` uses `$${VERSION}`. `RUN` is not on Docker's variable-expansion
instruction list, so the string reaches `/bin/sh` intact and the shell reads
`$$` as its own PID.

Verified with a minimal image:

```text
RUN echo "double-dollar: [$${VERSION}]" && echo "single-dollar: [${VERSION}]"
  double-dollar: [1{VERSION}]
  single-dollar: [v1.2.3]
```

So even after fixing H-2 the ldflag would be `-X main.Version=1{VERSION}`. Drop
one `$`.

### H-4. `docker-bake.hcl` never passes the build args at all

The `Dockerfile` declares `ARG VERSION/COMMIT/DATE`, but no bake target defines
an `args` block — only `labels`. Verified via `docker buildx bake dev --print`:
the target has `labels`, `tags`, `output`, and no `args` key. The OCI labels get
the metadata; the binary inside the image never does. H-2 and H-3 are both
downstream of this — all three need fixing together for image versioning to
work.

### H-5. `docker.just` is never imported — every docker recipe is unreachable

`docker.just:5` instructs "Import this from the main justfile with
`import 'docker.just'`", and `justfile` contains no `import` line. Verified:

```text
$ just --list | grep -i docker
(no output)
```

`just docker-build`, `docker-buildx`, `docker-push`, `docker-ci` do not exist.
Add `import 'docker.just'` near the top of `justfile`.

### H-6. `docker buildx bake` with no target fails — no `default` group

`docker.just` `docker-build` runs bare `docker buildx bake`, and `docker-push`
runs `docker buildx bake --push`. Verified:

```text
$ docker buildx bake --print
ERROR: failed to find target default
```

Add `group "default" { targets = ["dev"] }` to `docker-bake.hcl`, or name the
target explicitly in both recipes. Note `docker-push` is also conceptually off:
`--push` on the `dev` target contradicts its `output = ["type=docker"]`, and
`release` already sets `output = ["type=registry"]`, so `docker-push` is
redundant with `docker-buildx`.

### H-7. `CHANGELOG.md` is stale — the drift check fails right now

Verified `just changelog-check` exits 1. The committed `CHANGELOG.md` is 5 lines
(header only); git-cliff generates 51+ including the `0.1.0` → `0.3.0` history.
Two independent causes:

1. The file was never regenerated after `cliff.toml` landed.
2. Prettier already reflowed the header prose to 80 columns, so it no longer
   matches `cliff.toml`'s own wrapping — a byte-for-byte diff fails even on the
   header alone. `.prettierignore` now excludes `CHANGELOG.md`, but it was added
   after the damage.

`.github/workflows/changelog.yml` runs this check on every PR, so **every PR on
this branch fails CI until `just changelog` is run and committed.**

### H-8. `just lint-md` fails — the markdownlint config is never loaded

`.markdowncilint.yml` is not a filename markdownlint-cli2 recognises (it looks
for `.markdownlint.yaml`, `.markdownlint-cli2.yaml`, `.markdownlint.jsonc`,
etc.). Verified there is no recognised config file in the repo, so cli2 falls
back to defaults — including MD013 at 80 columns, which most of `docs/`
violates:

```text
$ just lint-md
docs/security-tooling-plan.md:76:81 MD013/line-length [Expected: 80; Actual: 142]
MAINTAINERS.md:1 MD041/first-line-heading ...
error: recipe `lint-md` failed on line 149 with exit code 1
```

This cascades: `just lint`, `just check`, and `just ci` all fail.

**Rename to `.markdownlint.yaml`, not `.markdownlint-cli2.yaml`** — both
filenames are recognised but they take different shapes, and this file's
contents (`MD004: false`, `MD013: false`, ... at the top level) are the plain
rules format that `.markdownlint.yaml` expects. The cli2 wrapper format needs
those rules nested under a `config:` key. Verified both ways: copied to
`.markdownlint.yaml` the MD013 noise disappears; copied to
`.markdownlint-cli2.yaml` it does not, because the top-level rules are ignored.

`.markdownlint.yaml` is also already globbed by `.github/labeler.yml`'s `repo`
section, so the rename fixes one of the dead globs in L-8 for free.

Renaming is necessary but not sufficient — 50 violations remain once the config
loads. `markdownlint-cli2 --fix` clears 27 of them (13× MD031, 14× MD032),
leaving 23 that need hand edits: 21× MD040 (code fences with no language), 1×
MD041, 1× MD024. Separately, `prettier --check "**/*.md"` — the second half of
`just lint-md` — fails on 13 files.

### M-1. The coverage gate is a no-op — it gates `internal/`, which does not exist

`justfile:84` filters packages on `{{ go_package }}/internal/`. This repo has no
`internal/` — packages are `collector/`, `config/`, `exporter/`,
`pkg/technitium`. Verified:

```text
$ just coverage-gate
  · no internal/ packages to gate yet
✓ Coverage gate passed (floor 60%)
```

CI runs this as "Coverage gate (per-package floor)" and it can never fail. The
inline comment also claims "`.codecov.yml` already ignores `cmd/`" — it does
not; `.codecov.yml` ignores `main.go`, `docs`, `scripts`. Retarget the prefix at
the real package roots, or drop the recipe rather than ship a green check that
measures nothing. Note `config/` and `exporter/` currently have **no test
files** at all (open issue #15), so a real gate would fail immediately — worth
knowing before flipping it on.

### M-2. `cliff.toml` renders PR links literally — `$${2}` is over-escaped

`cliff.toml:50` uses `replace = "([#$${2}](<REPO>/issues/$${2}))"`. In the Rust
regex replacer `$$` is an escaped literal `$`. Verified on a scratch repo with
commit `feat: add thing (#42)`:

```text
- Add thing ([#${2}](https://github.com/donaldgifford/technitium_exporter/issues/${2}))
```

Every squash-merged PR gets a broken link. `link_parsers` on line 71 correctly
uses single-`$` `$1`, which confirms the doubling is accidental. Use `${2}`.

### M-3. yamllint ignores `.github/` entirely

`.yamllint.yml` adds `.github/` to the ignore list, so `just lint-yaml` skips
all 11 workflow files this branch adds — the highest-churn YAML in the repo.
`actionlint` covers workflow semantics but not formatting/style. The ignore list
also names `.charts/`, and `key-duplicates` exempts
`config/testdata/section_key_dup.bad.yml`; verified neither path exists here
(donor-repo leftovers).

### M-4. golangci-lint version drift: CI v2.11.4, mise 2.12.2

`.github/workflows/ci.yml:38` pins `v2.11.4`; `mise.toml` pins `2.12.2`. Local
and CI can disagree on lint results. Have CI read the version from mise (it
already runs `mise-action` in `test-go`) or pin both to the same value.

### M-5. Two changelog systems coexist

`.chglog.yml` + `changelog.yml` (chglog, feeds the goreleaser `nfpms` deb
changelog) and `cliff.toml` + `CHANGELOG.md` (git-cliff) are both live, and
`mise.toml` installs both binaries. The `justfile` only knows about git-cliff;
CLAUDE.md documents only chglog. This is defensible if deliberate — they target
different artifacts — but nothing says so. Document the split or drop chglog.

### M-6. `ci.yml` runs goreleaser twice, on mismatched action versions, one shallow

- `package-lint` (line 103) checks out with `fetch-depth: 0` and uses
  `goreleaser-action@v6`.
- `build` (line 137) checks out **without** `fetch-depth: 0` and uses
  `goreleaser-action@v7.1.0`.

goreleaser needs full history and tags to derive a version; on a shallow clone
snapshot builds produce wrong version strings. Both jobs also run a full
`release --snapshot --clean`, duplicating the most expensive step in CI. Merge
them, or have `package-lint` consume `build`'s `dist/`.

### M-7. `ci.yml` labeler job: mislabeled step, and fork PRs will fail

Lines 21-23 name the step "Checkout code" while it is `actions/labeler@v6`. More
substantively, the workflow triggers on `pull_request`, which issues a read-only
token for fork PRs regardless of the job's `pull-requests: write` — the job will
fail on any external contribution. Use `pull_request_target`, or mark it
`continue-on-error: true`.

### M-8. `security.yml` invokes govulncheck-action with no checkout

`ci.yml`'s `security` job checks out first (line 77); `security.yml:19` calls
the same action with no checkout step. One of the two is wrong. Confirm whether
`donaldgifford/govulncheck-action` self-checkouts and make them consistent.

### M-9. No `concurrency` groups on any workflow

Rapid pushes stack redundant runs. `changelog-regen.yml` is the risky one: it
pushes to `main`, and two overlapping runs can race on the same commit. Add:

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true
```

(with `cancel-in-progress: false` for `changelog-regen.yml`).

### M-10. `license-check.yml` installs go-licenses unpinned

Line 27 runs `go install github.com/google/go-licenses@latest` while `mise.toml`
already pins it. Unpinned `@latest` in CI is both non-reproducible and a
supply-chain surface. Use `mise-action` as the other jobs do.

### L-1. `.gitignore` has an unsubstituted template placeholder

Line for `/${project_name}` was never expanded — literal, matches nothing. The
merge also duplicated `*.test`, `*.out`, `.env`, `.idea/`, `.vscode/`, and
`dist/`, which now each appear twice. The `dist/` comment still says "created by
`make release-local`".

### L-2. `.golangci.yml` carries exclusions for dependencies this project lacks

`errcheck.exclude-functions` names `github.com/fatih/color` and
`github.com/spf13/cobra`; `go.mod` has neither (this project uses kingpin). The
overrides also reference `path: cmd/(compare|diff)\.go$` and `mock_.*\.go$` —
none of those files exist. The blanket `G304:` gosec exclusion is justified as
"CLI tool reads user-specified file paths by design", which is not true of this
exporter; it weakens gosec for no benefit. `just lint-go` passes cleanly today
(`0 issues`), so removing the dead entries is zero-risk.

### L-3. `mise.toml` installs unused tooling

`cobra-cli` (project uses kingpin) and `mockery/v3` (CLAUDE.md is explicit that
tests use `httptest` and no interface mocking). Every CI job running
`mise-action` pays to install these.

### L-4. `catalog-info.yaml`: title typo and a TechDocs ref that cannot resolve

`title: technitium_expoerter` — typo. And `backstage.io/techdocs-ref: "dir:."`
requires an `mkdocs.yml` at the repo root; verified none exists (`.docz.yaml`
also points `wiki.mkdocs_path` at the same missing file). TechDocs will fail to
build. Either run `docz wiki` to generate `mkdocs.yml` or drop the annotation.

### L-5. `CODEOWNERS` still carries its template instruction

"IMPORTANT: Replace @org/CHANGEME with your actual team before merging" — the
line below it is already correct. Delete the instruction.

### L-6. `release.yml`: duplicate syft install, and a comment about jobs that do not exist

Lines 63-64 and 72-73 both run `anchore/sbom-action/download-syft@v0`. The
`publish-ghcr` comment (lines 84-89) describes "two publish workflows" and a
`chart` job that is idempotent on chart version — neither exists in this repo.

### L-7. `ghcr.yml`: stale cross-repo comment and wrong schema directive

The `outputs` comment (lines 49-54) references `ecr.yml`, which is not in this
repo. Line 2 uses a bare `# $schema=...github-action.json` instead of the
`# yaml-language-server: $schema=...github-workflow.json` directive used
consistently everywhere else — so editors get no schema for this file.

### L-8. `.github/labeler.yml`: a duplicate and several dead globs

`Makefile` is listed twice under `ci`. The `repo` section globs
`.goreleaser.yaml`, `.markdownlint.yaml`, `.prettierrc.yaml`, `changelog.yaml` —
the real files all use `.yml`, so those four never match. The newly added
`**/.markdownlint-cli2.yaml` matches nothing until H-8 is fixed.

### L-9. `scripts/labels.sh` dropped labels that `labeler.yml` still emits

`code-quality` and `testing` were removed from `LABEL_COLORS` and
`LABEL_DESCRIPTIONS`, but both are still top-level keys in
`.github/labeler.yml`, so `extract_labels_from_labeler` still finds them. They
will be created with the gray `EDEDED` fallback and an empty description.

### L-10. `justfile` `run` recipe: donor-repo doc comment, and missing env

The comment reads "just run plan -config-dir ./approved-providers" — from a
different project. And unlike `make run` / `make run-local`, the recipe passes
no `TECHNITIUM_URL` / `TECHNITIUM_TOKEN`, so `just run` fails on a bare
checkout.

### L-11. `.claude/` is untracked and not ignored

`.gitignore` ignores only `.claude/donald-loop.local.md`, so
`.claude/settings.json` would be committed on the next `git add -A`.
`.dockerignore` and `.prettierignore` both exclude the directory, implying it is
not meant to ship. Pick one.

### L-12. CLAUDE.md is now materially out of date

Go 1.25.7 (actual 1.26.5); every command documented as `make ...`; a "test.yml"
workflow that this branch deletes; `golang/govulncheck-action@v1` (actual:
`donaldgifford/govulncheck-action@v1`); `trivy-action@0.33.1` (actual
`v0.36.0`). Needs a pass once the just migration settles.

### What is working

Worth recording, since it bounds the blast radius:

- `just build`, `just test`, `just test-coverage` — pass.
- `just lint-go` — `0 issues`. `just lint-config` — clean.
- `just lint-actions` (actionlint over all 11 workflows) — clean, exit 0.
- `just release-check` — goreleaser config validates.
- `just license-check` — passes (warnings only, non-Go files in deps).
- All 23 pinned GitHub Action tags resolve via the API — no phantom versions,
  including the less-obvious `actions/checkout@v7` and
  `actions/upload-artifact@v7`.
- `dependabot.yml`'s `open-pull-requests-limit: 0` security-only mode, and the
  `pull_request_target` severity labeler (which correctly checks out no code),
  are both right.

## Make to just migration

`Makefile` is still tracked and unmodified on this branch. Five targets have no
`justfile` equivalent, and four of them are documented in CLAUDE.md — so
deleting `Makefile` today would break documented workflows.

### Missing — must port before deleting `Makefile`

| Makefile target | Purpose                                                             | Notes for the just port                                                                                                                                                                                      |
| --------------- | ------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `trivy`         | `trivy fs --scanners vuln --exit-code 1 --severity HIGH,CRITICAL .` | Documented in CLAUDE.md. CI already runs `trivy-action`; the local recipe is the only gap. **`trivy` is also missing from `mise.toml`** — it was dropped in this branch's rewrite, so port the tool pin too. |
| `syft`          | SBOM generation (SPDX + CycloneDX) to `build/`                      | Documented in CLAUDE.md. `syft` is still in `mise.toml`.                                                                                                                                                     |
| `security`      | Composite: `govulncheck` + `trivy`                                  | Documented in CLAUDE.md. `just govulncheck` exists; add the composite once `trivy` lands.                                                                                                                    |
| `run-local`     | Run the exporter against a local Technitium                         | Documented in CLAUDE.md. **Do not port the hardcoded token (H-1)** — take URL/token from the environment or a gitignored `.env`.                                                                             |
| `test-api`      | `curl` the two Technitium endpoints through `jq`                    | Same token caveat. `jq` is already pinned in `mise.toml`.                                                                                                                                                    |

### Intentionally dropped — no action needed

| Makefile target | Why                                                    |
| --------------- | ------------------------------------------------------ |
| `build-core`    | Internal alias for `build`.                            |
| `test-all`      | Alias for `test`.                                      |
| `help`          | Replaced by `just --list` (the `_default` recipe).     |
| `log-%`         | Make-specific echo helper; just recipes echo directly. |

### Already ported

`build`, `clean`, `test`, `test-pkg`, `test-coverage`, `test-report`, `lint`,
`lint-fix`, `fmt`, `ci`, `check`, `release`, `release-check`, `release-local`,
`govulncheck`.

Note `just fmt-go` is a behaviour change, not a port: `make fmt` shelled out to
`gofmt` + `goimports` directly, while `just fmt-go` delegates to
`golangci-lint fmt` so formatting and linting cannot disagree. This is the
better design — the `GOIMPORTS_LOCAL_ARG` in `Makefile` needs to be represented
in `.golangci.yml`'s `formatters` section (it is, via `gci`/`goimports` local
prefixes) for the behaviour to be preserved. Worth a spot-check after deletion.

### New capability with no Makefile ancestor

`test-integration`, `coverage-gate` (see M-1), `lint-yaml`, `lint-md`,
`lint-actions`, `lint-config`, `fmt-yaml`, `fmt-md`, `license-check`,
`license-report`, `changelog`, `changelog-check`, `labels`, and the whole
`docker.just` set (see H-5).

### Deletion order

1. Rotate the token (H-1) — independent of everything else, do it first.
2. Port `trivy` / `syft` / `security`, re-add the `trivy` pin to `mise.toml`.
3. Port `run-local` / `test-api` reading credentials from the environment.
4. `git rm Makefile`, then purge the token from history.
5. Update `.github/labeler.yml` (remove both `Makefile` entries, keep
   `justfile`) and CLAUDE.md's build-command section (L-12).

## Conclusion

**Answer:** The migration is structurally sound but not landable as-is.

Eight high-severity issues break real behaviour, and they cluster exactly where
predicted. The Docker layer is the worst of it: four separate defects (H-3, H-4,
H-5, H-6) mean the image pipeline has never successfully run end-to-end with
correct metadata, and two of them make the recipes unreachable or hard-fail. The
changelog and markdown-lint layers (H-7, H-8) will fail CI on the first PR.

The single most consequential finding is unrelated to the template port: a live
API token has been in `Makefile` and in git history since commit `00d895e`
(H-1).

Two findings are worse than the plain bugs because they report success while
doing nothing: `just build` prints a version it did not inject (H-2), and the CI
coverage gate passes without measuring any package (M-1).

The Go code itself is untouched and healthy — lint clean, tests pass, goreleaser
config valid, and every pinned action tag resolves.

On the second question: `Makefile` cannot be deleted yet. Five targets are
unported, four of them documented in CLAUDE.md, and one (`trivy`) also lost its
`mise.toml` tool pin in this branch.

## Recommendation

**Before merging this branch:**

1. Rotate the Technitium API token, then purge it from history (H-1).
2. Fix ldflags symbol names in `justfile` and `Dockerfile` to `main.Version` /
   `main.Commit` / `main.BuildDate` (H-2).
3. Fix the Docker chain together — single `$` (H-3), add `args` to the bake
   targets (H-4), add `import 'docker.just'` (H-5), add a `default` group (H-6).
   Then run `just docker-build` once and confirm
   `docker run --rm <image> --version` reports a real version.
4. Run `just changelog` and commit (H-7).
5. Rename `.markdowncilint.yml` → `.markdownlint.yaml`, then `--fix` and
   hand-clear the 23 residual violations to get `just lint-md` green (H-8).
6. Re-point or remove the coverage gate (M-1) — do not ship a check that cannot
   fail.
7. Fix `$${2}` → `${2}` in `cliff.toml`, then regenerate (M-2).

**Follow-up, can land separately:** M-3 through M-10 and all L-items. The
`Makefile` deletion should be its own PR following the ordering above, paired
with the CLAUDE.md refresh (L-12).

**Suggested verification gate** — this whole sequence should end green:

```bash
just ci && just docker-build && docker run --rm ghcr.io/donaldgifford/technitium_exporter:dev --version
```

Today it fails at `just lint-md` (step 1 of `just ci`).

## References

- Branch: `chore/update-deps` (working tree, uncommitted)
- Token exposure commit: `00d895e "testing the ci"`
- `CLAUDE.md` — build commands, metrics table, security tooling (stale, see
  L-12)
- Open issue #15 — missing `config/` and `exporter/` tests (blocks a real M-1
  gate)
- [Dockerfile variable substitution](https://docs.docker.com/reference/dockerfile/#environment-replacement)
  — `RUN` is not on the substitution list, which is the root of H-3
- [git-cliff configuration](https://git-cliff.org/docs/configuration) — `${N}`
  replacement syntax, M-2
- [markdownlint-cli2 config discovery](https://github.com/DavidAnson/markdownlint-cli2#configuration)
  — recognised filenames, H-8
