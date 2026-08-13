---
id: IMPL-0001
title: "Forge-registry migration remediation"
status: Draft
author: Donald Gifford
created: 2026-08-02
---

<!-- markdownlint-disable-file MD025 MD041 MD013 MD024 -->

# IMPL 0001: Forge-registry migration remediation

**Status:** Draft **Author:** Donald Gifford **Date:** 2026-08-02

<!--toc:start-->

- [Objective](#objective)
- [Scope](#scope)
  - [In Scope](#in-scope)
  - [Out of Scope](#out-of-scope)
- [Implementation Phases](#implementation-phases)
  - [Phase 1: Secret rotation and history purge](#phase-1-secret-rotation-and-history-purge)
    - [Tasks](#tasks)
    - [Success Criteria](#success-criteria)
  - [Phase 2: Build metadata correctness](#phase-2-build-metadata-correctness)
    - [Tasks](#tasks-1)
    - [Success Criteria](#success-criteria-1)
  - [Phase 3: Docker pipeline wiring](#phase-3-docker-pipeline-wiring)
    - [Tasks](#tasks-2)
    - [Success Criteria](#success-criteria-2)
  - [Phase 4: Unblock CI — changelog, markdown lint, coverage gate](#phase-4-unblock-ci--changelog-markdown-lint-coverage-gate)
    - [Tasks](#tasks-3)
      - [Changelog (H-7, M-2)](#changelog-h-7-m-2)
      - [Markdown lint (H-8)](#markdown-lint-h-8)
      - [Coverage gate (M-1)](#coverage-gate-m-1)
    - [Success Criteria](#success-criteria-3)
  - [Phase 5: Retire the Makefile](#phase-5-retire-the-makefile)
    - [Tasks](#tasks-4)
    - [Success Criteria](#success-criteria-4)
  - [Phase 6: Workflow hardening](#phase-6-workflow-hardening)
    - [Tasks](#tasks-5)
    - [Success Criteria](#success-criteria-5)
  - [Phase 7: Template residue cleanup](#phase-7-template-residue-cleanup)
    - [Tasks](#tasks-6)
    - [Success Criteria](#success-criteria-6)
  - [Phase 8: Documentation refresh](#phase-8-documentation-refresh)
    - [Tasks](#tasks-7)
    - [Success Criteria](#success-criteria-7)
- [File Changes](#file-changes)
- [Testing Plan](#testing-plan)
- [Dependencies](#dependencies)
- [Decisions](#decisions)
- [References](#references)
<!--toc:end-->

## Objective

Remediate the 33 findings from INV-0001 so the `chore/update-deps` branch is
mergeable, then retire `Makefile` in favour of `justfile`.

**Implements:** INV-0001

The end state is a repo where this sequence is green:

```bash
just ci && just docker-build && \
  docker run --rm ghcr.io/donaldgifford/technitium_exporter:dev --version
```

Today it fails at `just lint-md`, the first step of `just ci`.

## Scope

### In Scope

- Rotating the leaked Technitium API token and purging it from git history
  (INV-0001 H-1)
- Fixing all 9 high-severity findings (H-1 … H-9; H-9 is already fixed on PR
  #28, where CI surfaced it)
- Fixing the 10 medium findings (M-1 … M-10)
- Fixing the 12 low findings (L-1 … L-12)
- Porting the 5 unported `Makefile` targets to `justfile` and deleting
  `Makefile`
- Refreshing CLAUDE.md to match the post-migration reality

### Out of Scope

- Writing the missing `config/` and `exporter/` tests (open issue #15). Per
  OQ-4a the coverage gate covers `collector/` and `pkg/technitium/` only; the
  other two packages join it when #15 lands.
- Any change to `collector/`, `pkg/technitium/`, or the exporter's runtime
  behaviour. The Go source is healthy and is not touched by this plan.
- The Loki/Alloy integration and exporter-enhancement plans in `docs/`.
- Adopting a Helm chart or the `chart` publish job that `release.yml`'s comments
  reference (see L-6) — those comments are donor-repo residue and get deleted,
  not implemented.

## Implementation Phases

Each phase builds on the previous one. A phase is complete when all its tasks
are checked off and its success criteria are met.

Phases 1–5 are the merge blockers. Phases 6–8 can land as follow-up PRs.

---

### Phase 1: Secret rotation and history purge

Independent of everything else and the most time-sensitive item in the plan. The
repo is **public** with 0 forks, so the token in `Makefile:36-37` has been
world-readable since commit `00d895e` (2 commits into a 30-commit history).
Rotation is mandatory regardless of what happens to the history.

Low blast radius for a rewrite: sole owner, no forks, no external contributors.
The 6 open Dependabot PRs are all superseded by this branch (trivy 0.36.0,
labeler v6, goreleaser v7, codecov v6, import-gpg v7 are already applied here),
so they can be closed rather than rebased.

#### Tasks

- [ ] Revoke the exposed token (see `Makefile` at commit `00d895e`) in the
      Technitium admin UI and issue a replacement
- [ ] Confirm the old token is rejected:
      `curl -s "$URL/api/settings/get?token=<old>"` returns an auth failure
- [ ] Audit the DNS server for use of the old token between `00d895e` and
      revocation, to the extent the logs allow
- [ ] Remove the `TECHNITIUM_URL` / `TECHNITIUM_TOKEN` assignments from
      `Makefile` (the file itself is deleted in Phase 5)
- [ ] Purge the token from history with `git filter-repo` (OQ-1a), e.g.
      `git filter-repo --replace-text .token-to-purge` with a
      `<token>==>REDACTED` line in that gitignored file
- [ ] Close the 6 superseded Dependabot PRs (#22, #23, #24, #25, #26, #27) with
      a note pointing at this branch
- [ ] Force-push the rewritten history
- [ ] File the GitHub Support request to GC unreachable objects and drop the
      stale `refs/pull/*` refs (draft at
      `.github/SUPPORT-REQUEST-purge-unreachable-objects.md`) — the force-push
      alone does NOT evict them
- [ ] Re-enable the `main` repository ruleset (id 12379777), disabled to permit
      the force-push
- [ ] After Support confirms, verify `00d895e` no longer resolves, then delete
      the drafted ticket and the pre-purge backup bundle in `~`
- [ ] Store the new token in a gitignored `.env` (never in a tracked file)

#### Success Criteria

- A fresh clone contains zero blobs with the credential (done — verified
  2026-08-12)
- `gh api "repos/.../contents/Makefile?ref=00d895e"` no longer returns the token
  — this is the criterion that distinguishes a rewritten branch from an actually
  purged repo, and it is **not** satisfied by the force-push alone
- The old token is confirmed rejected by the live server
- No open PR references a pre-rewrite SHA

---

### Phase 2: Build metadata correctness

`just build` and the container image both silently fail to inject version
information, and the recipe prints a success message containing a version it did
not actually set. Three defects stack here (H-2, H-3, H-4) and all three must be
fixed before the injection works end to end.

Ground truth is `cmd/technitium_exporter/main.go:25-29`, which declares
`Version`, `Commit`, `BuildDate`. `.goreleaser.yml:10` already targets these
correctly and is not changed.

#### Tasks

- [ ] `justfile:31` — change `-X main.version/commit/date` to
      `-X main.Version/Commit/BuildDate`
- [ ] `Dockerfile:16` — same symbol rename
- [ ] `Dockerfile:16` — change `$${VERSION}` / `$${COMMIT}` / `$${DATE}` to a
      single `$` (`RUN` is not on Docker's substitution list, so `$$` reaches
      the shell and expands to its PID)
- [ ] `docker-bake.hcl` — add an `args` block to `_common` mapping `VERSION`,
      `COMMIT`, `DATE` to the existing `VERSION` / `COMMIT_SHA` / `BUILD_DATE`
      bake variables
- [ ] Verify the three ldflag symbol names against `main.go` rather than against
      each other
- [ ] Re-check that `just build`'s success `echo` reports the version actually
      compiled in, not the `just` variable

#### Success Criteria

- `just build && ./build/bin/technitium_exporter --version` prints a real
  version, commit, and date — not `dev (commit: none, built: unknown)`
- `docker buildx bake dev --print` shows an `args` key on the target
- The version string contains no `{VERSION}` fragment and no bare PID

---

### Phase 3: Docker pipeline wiring

The image pipeline has never run end to end. Phase 2 fixed what the build
injects; this phase makes the recipes reachable and the bake invocation valid.

#### Tasks

- [ ] Add `import 'docker.just'` to `justfile` (H-5) — `docker.just` documents
      this requirement in its own header but nothing does it
- [ ] Add `group "default" { targets = ["dev"] }` to `docker-bake.hcl` (H-6) —
      bare `docker buildx bake` currently exits with
      `ERROR: failed to find target default`
- [ ] Delete the `docker-push` recipe from `docker.just` (OQ-2a) — it runs
      `bake --push` against a `dev` target whose `output` is `type=docker`,
      which contradicts itself, and duplicates `docker-buildx`. Releases push
      from `ghcr.yml` on a tag, not from a laptop
- [ ] Update `docker-bake.hcl:68`'s stale `make docker-push` comment reference
- [ ] Confirm `just --list` shows all four docker recipes

#### Success Criteria

- `just --list | grep docker` lists the docker recipes (currently returns
  nothing)
- `just docker-build` completes and loads an image into the local daemon
- `docker run --rm ghcr.io/donaldgifford/technitium_exporter:dev --version`
  reports the correct version — this is the end-to-end proof for Phases 2 and 3
  together
- `docker buildx bake ci --print` still resolves both platforms

---

### Phase 4: Unblock CI — changelog, markdown lint, coverage gate

Three findings each independently fail CI on the first PR. Grouped because
together they are exactly what stands between `just ci` and green.

The markdown work is the bulk of it. Renaming the config surfaces 50 real
violations that the missing config had been hiding; `--fix` clears 27, leaving
23 hand edits (21× MD040 code fences with no language, 1× MD041, 1× MD024).

#### Tasks

##### Changelog (H-7, M-2)

- [ ] Fix `cliff.toml:50` — `$${2}` → `${2}` in `commit_preprocessors` (`$$` is
      an escaped literal `$` in the Rust regex replacer, so PR links currently
      render as `[#${2}](…/issues/${2})`)
- [ ] Run `just changelog` and commit the regenerated `CHANGELOG.md`
- [ ] Verify `just changelog-check` exits 0
- [ ] Confirm `CHANGELOG.md` is listed in `.prettierignore` so `just fmt-md`
      cannot reflow it back into drift

##### Markdown lint (H-8)

- [ ] `git mv .markdowncilint.yml .markdownlint.yaml` — **not**
      `.markdownlint-cli2.yaml`; the file holds top-level plain rules
      (`MD013: false`, …), which is the format `.markdownlint.yaml` expects.
      Verified both ways: the cli2 wrapper name ignores top-level rules and
      MD013 keeps firing
- [ ] Run `prettier --write "**/*.md"` first, then `markdownlint-cli2 --fix`
      (prettier's blank-line handling resolves most MD031/MD032 before
      markdownlint has to)
- [ ] Hand-fix the 21 MD040 violations by adding a language to each bare code
      fence
- [ ] Fix `MAINTAINERS.md:1` MD041 (add a top-level heading)
- [ ] Fix `docs/llms.md:256` MD024 (duplicate "Key Rules" heading)
- [ ] Add the docz-generated surfaces to `.prettierignore` (OQ-3a): the 6
      `docs/*/README.md` index files, and whatever is needed to stop prettier
      fighting the `<!--toc:start-->` block docz injects into every doc body.
      Confirm the fix by running `docz update` then `just lint-md` and checking
      that order no longer matters
- [ ] Verify `just lint-md` exits 0

##### Coverage gate (M-1)

- [ ] Re-point `justfile:84`'s `internal/` prefix at `collector/` and
      `pkg/technitium/` only (OQ-4a). Current per-package coverage: `collector`
      96.9%, `pkg/technitium` 91.5%, `config` 0%, `exporter` 0%, `cmd` 0% — both
      gated packages clear the 60% floor comfortably today
- [ ] Leave a comment in the recipe naming `config/` and `exporter/` as
      deliberately ungated pending issue #15, so the omission reads as a
      decision rather than an oversight
- [ ] Fix the recipe's inline comment, which claims `.codecov.yml` ignores
      `cmd/`; it actually ignores `main.go`, `docs`, `scripts`
- [ ] Ensure the gate can actually fail — verify by temporarily lowering the
      floor above a real package's coverage

#### Success Criteria

- `just lint-md` exits 0
- `just changelog-check` exits 0
- `just coverage-gate` reports at least one real package, and a deliberately
  raised floor makes it fail
- `just ci` passes end to end — the first time on this branch
- A test commit of the form `feat: thing (#42)` renders as a working link in
  regenerated changelog output

---

### Phase 5: Retire the Makefile

`Makefile` cannot be deleted until its 5 unported targets exist in `justfile`; 4
of the 5 are documented in CLAUDE.md, so deleting early breaks documented
workflows. Note `trivy` also lost its `mise.toml` pin during the branch's
rewrite, so the tool needs re-adding, not just the recipe.

Neither `run-local` nor `test-api` may carry the hardcoded credentials forward
(Phase 1).

#### Tasks

- [ ] Re-add the `trivy` pin to `mise.toml` with a `# renovate:` annotation
      matching the surrounding style
- [ ] Add `just trivy` —
      `trivy fs --scanners vuln --exit-code 1 --severity HIGH,CRITICAL .`
- [ ] Add `just syft` — SBOM generation (SPDX + CycloneDX) into `build/`
- [ ] Add `just security` — composite of `govulncheck` + `trivy`
- [ ] Add `just run-local` reading `TECHNITIUM_URL` / `TECHNITIUM_TOKEN` from
      the environment, with a clear error when unset
- [ ] Add `just test-api` — the two `curl` calls through `jq`, same env sourcing
- [ ] Fix `justfile`'s `run` recipe (L-10): replace the donor-repo doc comment
      ("just run plan -config-dir ./approved-providers") and source credentials
      from the environment
- [ ] Spot-check that `just fmt-go` preserves `make fmt`'s local-import grouping
      — `Makefile` passed `-local github.com/donaldgifford` explicitly;
      `just fmt-go` relies on `.golangci.yml`'s `gci`/`goimports` prefixes to do
      the same
- [ ] `git rm Makefile`
- [ ] Remove both `Makefile` entries from `.github/labeler.yml`'s `ci` section
      (it is listed twice — L-8), keep `justfile`
- [ ] Grep the repo for surviving `make` references and update them

#### Success Criteria

- `Makefile` is gone and `git grep -n "make "` returns no stale instructions
- `just security`, `just trivy`, `just syft` all run to completion
- `just run-local` fails with a clear message when credentials are unset, and
  works when they are
- Every command documented in CLAUDE.md's build section resolves to a real
  `just` recipe

---

### Phase 6: Workflow hardening

Follow-up PR. None of these block the merge, but several waste CI time or fail
on fork PRs.

#### Tasks

- [ ] M-4 — install golangci-lint via `mise-action` in CI and drop the
      `version:` input from `golangci-lint-action` (OQ-5a), making `mise.toml`
      the single source of truth. CI pins `v2.11.4` today, `mise.toml` pins
      `2.12.2`
- [ ] M-6 — give `ci.yml`'s `build` job `fetch-depth: 0` (goreleaser needs tags;
      `package-lint` already has it), and align the two jobs on one
      `goreleaser-action` major (currently `v6` and `v7.1.0`)
- [ ] M-6 — deduplicate the two full `goreleaser release --snapshot` runs across
      `build` and `package-lint`
- [ ] M-7 — fix the `labeler` job: the step is named "Checkout code" but is
      `actions/labeler@v6`, and `pull_request` yields a read-only token on fork
      PRs, so it will fail on external contributions
- [ ] M-8 — make `security.yml` consistent with `ci.yml`'s security job on
      whether `donaldgifford/govulncheck-action` needs a preceding checkout
- [ ] M-9 — add `concurrency` groups to all workflows; use
      `cancel-in-progress: false` for `changelog-regen.yml`, which pushes to
      `main` and can race with itself
- [ ] M-10 — replace `license-check.yml`'s
      `go install github.com/google/go-licenses@latest` with the `mise.toml` pin
      via `mise-action`
- [ ] M-3 — remove `.github/` from `.yamllint.yml`'s ignore list (OQ-6a) and fix
      whatever surfaces across the 11 workflow files; also drop the dead
      `.charts/` and `config/testdata/section_key_dup.bad.yml` entries
- [ ] M-5 — keep both chglog and git-cliff (OQ-7a) and make the split explicit:
      add `just` recipes for the chglog side (deb package changelog) alongside
      the existing git-cliff ones (repo `CHANGELOG.md`), and document which
      artifact each one feeds

#### Success Criteria

- CI wall-clock drops measurably (one goreleaser snapshot run, not two)
- A PR from a fork does not fail the labeler job
- Local and CI golangci-lint report identical results on the same commit
- No workflow references a tool version that disagrees with `mise.toml`

---

### Phase 7: Template residue cleanup

Follow-up PR. Cosmetic and correctness cleanup of values the template carried
over from the donor repo. Individually trivial; collectively they are what makes
the repo read as genuinely ported rather than copied.

#### Tasks

- [ ] L-1 — remove the unsubstituted `/${project_name}` line from `.gitignore`
      and de-duplicate `*.test`, `*.out`, `.env`, `.idea/`, `.vscode/`, `dist/`
      (each now appears twice); fix the `make release-local` comment
- [ ] L-2 — remove `.golangci.yml` exclusions for absent dependencies
      (`github.com/fatih/color`, `github.com/spf13/cobra` — this project uses
      kingpin), the `cmd/(compare|diff)\.go$` and `mock_.*\.go$` paths, and the
      blanket `G304:` gosec exclusion justified as "CLI tool reads
      user-specified file paths" (untrue of this exporter)
- [ ] L-3 — remove `cobra-cli` and `mockery/v3` from `mise.toml`; neither is
      used and both are installed on every `mise-action` CI job
- [ ] L-4 — fix `catalog-info.yaml`'s `technitium_expoerter` typo
- [ ] L-4 — run `docz wiki` to generate the `mkdocs.yml` that
      `techdocs-ref: "dir:."` requires (OQ-8a); `.docz.yaml`'s `wiki` block is
      already configured for it (techdocs-core, nav titles, exclusions), so this
      looks like a step that was simply never run. Verify the generated nav
      covers all six docz types
- [ ] L-5 — delete CODEOWNERS' "Replace @org/CHANGEME" instruction; the line
      below it is already correct
- [ ] L-6 — remove `release.yml`'s duplicate syft install (lines 63-64 and
      72-73) and the `publish-ghcr` comment describing a `chart` job and "two
      publish workflows" that do not exist here
- [ ] L-7 — fix `ghcr.yml`'s `ecr.yml` reference and its bare `# $schema=`
      directive (should be `# yaml-language-server: $schema=` pointing at
      `github-workflow.json`, as every other workflow does)
- [ ] L-8 — fix `.github/labeler.yml`: the duplicate `Makefile` entry, and the
      `repo` globs naming `.goreleaser.yaml`, `.prettierrc.yaml`,
      `changelog.yaml` when the real files use `.yml`
- [ ] L-9 — restore `code-quality` and `testing` to `scripts/labels.sh`'s
      colour/description maps, or drop them from `.github/labeler.yml`; they
      currently get created with the gray `EDEDED` fallback and no description
- [ ] L-11 — commit `.claude/settings.json` and ignore the rest of `.claude/`
      (OQ-9a): add `.claude/*` plus a `!.claude/settings.json` negation to
      `.gitignore`, replacing the narrow `donald-loop.local.md` rule

#### Success Criteria

- No file contains an unsubstituted template placeholder or a reference to a
  path, dependency, or workflow that does not exist in this repo
- `just lint` still passes
- `scripts/labels.sh --dry-run` proposes no gray-fallback labels

---

### Phase 8: Documentation refresh

Follow-up PR, last so it documents the settled state rather than a moving
target.

#### Tasks

- [ ] L-12 — update CLAUDE.md's build-commands section: every `make X` becomes
      `just X`
- [ ] L-12 — correct the Go version (documented 1.25.7, actual 1.26.5 per
      `go.mod`)
- [ ] L-12 — replace the `test.yml` workflow reference; this branch deletes it
      in favour of `ci.yml`
- [ ] L-12 — correct `golang/govulncheck-action@v1` to
      `donaldgifford/govulncheck-action@v1` and `trivy-action@0.33.1` to
      `v0.36.0`
- [ ] Document the Docker workflow in CLAUDE.md — `Dockerfile`,
      `docker-bake.hcl`, `docker.just`, and the GHCR publish path are entirely
      undocumented
- [ ] Document the changelog workflow and whichever tool survives OQ-7
- [ ] Add a "Task Runner" section explaining `just --list` and the recipe groups
- [ ] Update INV-0001's status to reflect that remediation has landed
- [ ] Run `docz update` to refresh the index tables

#### Success Criteria

- Every command in CLAUDE.md executes successfully as written
- No reference to `make`, `test.yml`, or Go 1.25.7 survives
- `docz update --dry-run` reports no drift

---

## File Changes

| File                                  | Action | Description                                                          |
| ------------------------------------- | ------ | -------------------------------------------------------------------- |
| `Makefile`                            | Delete | After its 5 remaining targets are ported (Phase 5)                   |
| `justfile`                            | Modify | ldflags symbols, `import 'docker.just'`, 5 ported recipes, `run` fix |
| `Dockerfile`                          | Modify | ldflags symbols and `$$` → `$`                                       |
| `docker-bake.hcl`                     | Modify | Add `args` block and `group "default"`                               |
| `cliff.toml`                          | Modify | `$${2}` → `${2}`                                                     |
| `CHANGELOG.md`                        | Modify | Regenerate via `just changelog`                                      |
| `.markdowncilint.yml`                 | Rename | → `.markdownlint.yaml` (plain-rules format)                          |
| `mise.toml`                           | Modify | Re-add `trivy`; drop `cobra-cli` and `mockery/v3`                    |
| `.gitignore`                          | Modify | Drop `/${project_name}`, de-duplicate, `.claude/` negation (OQ-9a)   |
| `.prettierignore`                     | Modify | Ignore docz-generated surfaces (OQ-3a)                               |
| `docker.just`                         | Modify | Delete the `docker-push` recipe (OQ-2a)                              |
| `.golangci.yml`                       | Modify | Remove exclusions for absent deps and paths                          |
| `.yamllint.yml`                       | Modify | Drop the `.github/` ignore (OQ-6a) and the dead paths                |
| `.github/labeler.yml`                 | Modify | Drop `Makefile` entries; fix `.yaml`/`.yml` globs                    |
| `.github/workflows/ci.yml`            | Modify | `fetch-depth`, goreleaser dedup, labeler job, concurrency            |
| `.github/workflows/release.yml`       | Modify | Drop duplicate syft install and donor-repo comments                  |
| `.github/workflows/ghcr.yml`          | Modify | Fix `ecr.yml` reference and schema directive                         |
| `.github/workflows/security.yml`      | Modify | Checkout consistency; concurrency                                    |
| `.github/workflows/license-check.yml` | Modify | Use the mise pin instead of `go install @latest`                     |
| `.github/CODEOWNERS`                  | Modify | Drop the template instruction line                                   |
| `catalog-info.yaml`                   | Modify | Fix the `technitium_expoerter` typo                                  |
| `scripts/labels.sh`                   | Modify | Restore `code-quality` / `testing` entries                           |
| `CLAUDE.md`                           | Modify | Full refresh (Phase 8)                                               |
| `*.md` (13 files)                     | Modify | markdownlint + prettier fixes                                        |
| `.env`                                | Create | Gitignored; local Technitium credentials                             |
| `mkdocs.yml`                          | Create | Generated by `docz wiki` for TechDocs (OQ-8a)                        |

## Testing Plan

This is a tooling change, so verification is by executing the tooling rather
than by new Go tests. The Go source is untouched.

- [ ] `just ci` — the composite gate; must pass before merge
- [ ] `just build && ./build/bin/technitium_exporter --version` — Phase 2 proof
- [ ] `just docker-build && docker run --rm <image> --version` — Phase 3 proof
- [ ] `just security`, `just trivy`, `just syft` — Phase 5 proof
- [ ] `just run-local` with and without credentials set — Phase 5 proof
- [ ] `git log -S <token> --all` returns empty — Phase 1 proof
- [ ] Open a throwaway PR to confirm `changelog.yml`, the labeler, and the
      required-labels check all behave — several findings only manifest in a
      real PR context
- [ ] Confirm the coverage gate fails when the floor is raised above a real
      package's coverage

Regression watch: `just test` must stay green throughout. No phase should change
`collector/` or `pkg/technitium/`.

## Dependencies

- **Phase 1 blocks nothing technically** but should land first — the token is
  publicly exposed while this sits.
- **Phase 2 blocks Phase 3.** The bake `args` wiring is meaningless until the
  ldflags symbols are right, and the Phase 3 success criterion is an end-to-end
  version check that depends on both.
- **Phase 4 blocks Phase 5.** `just ci` must be green before removing a fallback
  build path.
- **Phases 6–8 depend on Phases 1–5** but not on each other.
- **Phase 8 should land last** so it documents the settled state.
- **OQ-4 has an external dependency:** a coverage gate covering all packages
  requires the `config/` and `exporter/` tests tracked in issue #15.
- **Phase 1 requires admin access** to the Technitium server to rotate the
  token, and `git filter-repo` or BFG installed locally.

## Decisions

All ten open questions were resolved on 2026-08-02, each to option **a**. The
alternatives considered are preserved in this document's history; what follows
is the decision and the reason it was chosen. Phase tasks above have been
rewritten to be unconditional.

| #     | Decision                                                            | Affects  |
| ----- | ------------------------------------------------------------------- | -------- |
| OQ-1  | Purge history with `git filter-repo` and force-push                 | Phase 1  |
| OQ-2  | Delete the `docker-push` recipe                                     | Phase 3  |
| OQ-3  | `.prettierignore` the docz-generated surfaces                       | Phase 4  |
| OQ-4  | Gate `collector/` + `pkg/technitium/` only, at the 60% floor        | Phase 4  |
| OQ-5  | CI installs golangci-lint via `mise-action`; `mise.toml` is the pin | Phase 6  |
| OQ-6  | Remove `.github/` from `.yamllint.yml`'s ignore list                | Phase 6  |
| OQ-7  | Keep chglog and git-cliff; document and script the split            | Phase 6  |
| OQ-8  | Generate `mkdocs.yml` with `docz wiki`                              | Phase 7  |
| OQ-9  | Commit `.claude/settings.json`, ignore the rest of `.claude/`       | Phase 7  |
| OQ-10 | Phase 1 as its own PR, then Phases 2-5 together                     | Sequence |

**OQ-1 — history purge.** Chose `git filter-repo` + force-push. Executed
2026-08-12; `main` is now `dfee94e`, 36 commits, zero blobs containing the
credential, all three tags rewritten and reachable.

**The reasoning behind this choice was partly wrong, and the outcome fell short
of it.** The claim was that filter-repo "is the only option that actually
removes the secret from a public repo." On GitHub that is false. All 18
`refs/pull/N/head` refs still point into the pre-rewrite history, and GitHub
serves any object in the store by SHA — so the credential remained retrievable
at `00d895e` after the rewrite:

```bash
gh api "repos/donaldgifford/technitium_exporter/contents/Makefile?ref=00d895e"
# decoded line 37 still contained the token
```

Verified that no client-side remedy exists. All three are refused:

| Attempt                                      | Result                       |
| -------------------------------------------- | ---------------------------- |
| `git push origin --delete refs/pull/22/head` | `deny updating a hidden ref` |
| `gh api -X DELETE .../pulls/22`              | 404 — no such endpoint       |
| force-update a PR ref to a clean commit      | `deny updating a hidden ref` |

So on GitHub, options (a) and (b) differ far less than stated: a rewrite alone
evicts nothing. Eviction requires GitHub Support to GC unreachable objects and
drop the stale PR refs — tracked as a Phase 1 task, with the ticket drafted at
`.github/SUPPORT-REQUEST-purge-unreachable-objects.md`.

**Rotation is the control that actually matters**, and that was true before the
rewrite as well. The rewrite is hygiene; rotation is the fix.

**OQ-2 — `docker-push`.** Deleted. It duplicates `docker-buildx`, and its bake
invocation contradicts itself (`--push` against a `type=docker` output). Pushes
belong to `ghcr.yml` on a tag, where the image also gets signed — one fewer way
to put an unsigned image in the registry by hand.

**OQ-3 — docz vs prettier.** Generated content is not hand-formatted;
`CHANGELOG.md` already sets that precedent in the same file. This removes the
order-dependence between `docz update` and `just lint-md` rather than papering
over it with a remember-to-run-both convention.

**OQ-4 — coverage gate.** Gating `collector/` (96.9%) and `pkg/technitium/`
(91.5%) converts a check that cannot fail into one that can, today, without
pulling issue #15 into this branch's scope. `config/` and `exporter/` are at 0%
and join the gate when their tests land — recorded in the recipe as a deliberate
omission so it does not read as an oversight later.

**OQ-5 — golangci-lint pin.** One source of truth beats two pins that must be
remembered together. `mise.toml` already carries Renovate annotations, and
`ci.yml`'s `test-go` job already runs `mise-action`, so this is mostly deletion.

**OQ-6 — yamllint and `.github/`.** The workflows are the highest-churn YAML in
the repo and currently the only YAML nothing style-checks. They are also brand
new, so the cleanup cost will never be lower than it is right now.

**OQ-7 — chglog and git-cliff.** They feed genuinely different artifacts: the
Debian package changelog and the repo `CHANGELOG.md`. The problem was never the
overlap, it was that the split was implicit — undocumented and half-scripted.
Adding chglog recipes alongside the git-cliff ones makes it legible.

**OQ-8 — TechDocs.** `.docz.yaml`'s `wiki` block is already fully configured
(techdocs-core plugin, nav titles, exclusions). The `mkdocs.yml` it targets is
missing because `docz wiki` was never run, not because the annotation was
aspirational — so generating it costs one command and makes
`catalog-info.yaml`'s existing `techdocs-ref` resolve.

**OQ-9 — `.claude/`.** Shared project permissions belong in the repo;
per-session state does not. This is what the existing narrow
`donald-loop.local.md` rule was reaching for, generalised.

**OQ-10 — PR sequencing.** Phase 1 ships alone so the secret rotation is not
queued behind review of anything else. Phases 2-5 land together because they are
one coherent "make the migration actually work" change, and because Phases 2 and
3 cannot be independently verified — the proof for both is a single end-to-end
`docker run --version`, so splitting them would merge a PR with a known-broken
path.

## References

- [INV-0001](../investigation/0001-forge-registry-template-migration-review.md)
  — the investigation this plan implements; all H/M/L IDs refer to its findings
- Branch: `chore/update-deps` (working tree, uncommitted)
- Token exposure commit: `00d895e "testing the ci"`
- Open issue #15 — missing `config/` and `exporter/` tests (gates OQ-4 option b)
- Open PRs #22-#27 — superseded Dependabot bumps, closed in Phase 1
- [git filter-repo](https://github.com/newren/git-filter-repo) — OQ-1 option a
- [Dockerfile variable substitution](https://docs.docker.com/reference/dockerfile/#environment-replacement)
  — `RUN` is not on the substitution list (Phase 2)
- [markdownlint-cli2 config discovery](https://github.com/DavidAnson/markdownlint-cli2#configuration)
  — recognised filenames and their formats (Phase 4)
- [git-cliff configuration](https://git-cliff.org/docs/configuration) — `${N}`
  replacement syntax (Phase 4)
