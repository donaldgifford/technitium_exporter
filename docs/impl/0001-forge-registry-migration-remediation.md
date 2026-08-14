---
id: IMPL-0001
title: "Forge-registry migration remediation"
status: In Progress
author: Donald Gifford
created: 2026-08-02
---

<!-- markdownlint-disable-file MD025 MD041 MD013 MD024 -->

# IMPL 0001: Forge-registry migration remediation

**Status:** In Progress **Author:** Donald Gifford **Date:** 2026-08-02

<!-- prettier-ignore-start -->

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
      - [Housekeeping](#housekeeping)
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

<!-- prettier-ignore-end -->

## Objective

Remediate the 33 findings from INV-0001 (M-11 and L-14 were raised later, by the
audit of this work, and are out of scope here) so the `chore/update-deps` branch
is mergeable, then retire `Makefile` in favour of `justfile`.

**Implements:** INV-0001

The end state is a repo where this sequence is green:

```bash
just ci && just docker-build && \
  docker run --rm ghcr.io/donaldgifford/technitium_exporter:dev --version
```

That sequence is now green. All eight phases' repo-side tasks have landed.

The doc is **In Progress** rather than Completed because four Phase 1
owner-actions remain open, and they are the consequential ones: the leaked
Technitium API token has not been rotated, the GitHub Support request to evict
it from the `refs/pull/*` objects has not been submitted, and the `main`
repository ruleset is still disabled from the force-push. None of those can be
done from inside the repo. See Phase 1.

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

Repo-side work landed 2026-08-12 (PR #28, PR #29). The remaining items are
owner-actions on the DNS server and on GitHub Support; they are **deferred by
explicit decision**, not forgotten, and do not block Phases 2-8.

- [x] Remove the `TECHNITIUM_URL` / `TECHNITIUM_TOKEN` assignments from
      `Makefile` (the file itself is deleted in Phase 5) — PR #28, replaced with
      `-include .env` and a `require-technitium` guard
- [x] Purge the token from history with `git filter-repo` (OQ-1a) — `main`
      rewritten to `dfee94e`, 36 commits, zero blobs containing the credential
- [x] Close the 6 superseded Dependabot PRs (#22, #23, #24, #25, #26, #27) with
      a note pointing at this branch
- [x] Force-push the rewritten history — `main` and all three tags
- [x] Draft the GitHub Support request to GC unreachable objects and drop the
      stale `refs/pull/*` refs
      (`.github/SUPPORT-REQUEST-purge-unreachable-objects.md`)
- [ ] **[owner]** Revoke the exposed token in the Technitium admin UI and issue
      a replacement, then store it in a gitignored `.env`
- [ ] **[owner]** Confirm the old token is rejected, and audit the DNS server
      for its use between `00d895e` and revocation
- [ ] **[owner]** Submit the drafted Support ticket — the force-push alone does
      NOT evict the objects; all 18 `refs/pull/N/head` refs still hold them, and
      no client-side removal is possible
- [ ] **[owner]** Re-enable the `main` repository ruleset (id 12379777),
      disabled to permit the force-push and still `disabled` as of 2026-08-12
- [ ] **[owner]** After Support confirms, verify `00d895e` no longer resolves,
      then delete the drafted ticket and the pre-purge backup bundle in `~` (the
      bundle contains the original credential)

#### Success Criteria

- A fresh clone contains zero blobs with the credential — **met**, verified
  2026-08-12
- No open PR references a pre-rewrite SHA — **met**, all 6 closed and branches
  deleted
- `gh api "repos/.../contents/Makefile?ref=00d895e"` no longer returns the token
  — **not met**, and not achievable client-side. This is the criterion that
  distinguishes a rewritten branch from an actually purged repo; it needs the
  Support ticket
- The old token is confirmed rejected by the live server — **not met**, owner
  action

Phase 1 is therefore _complete for everything doable in this repo_ and open on
four owner-actions. Phases 2-8 proceed independently.

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

- [x] `justfile:31` — change `-X main.version/commit/date` to
      `-X main.Version/Commit/BuildDate`
- [x] `Dockerfile:16` — same symbol rename
- [x] `Dockerfile:16` — change `$${VERSION}` / `$${COMMIT}` / `$${DATE}` to a
      single `$` (`RUN` is not on Docker's substitution list, so `$$` reaches
      the shell and expands to its PID)
- [x] `docker-bake.hcl` — add an `args` block to `_common` mapping `VERSION`,
      `COMMIT`, `DATE` to the existing `VERSION` / `COMMIT_SHA` / `BUILD_DATE`
      bake variables
- [x] Verify the three ldflag symbol names against `main.go` rather than against
      each other
- [x] Re-check that `just build`'s success `echo` reports the version actually
      compiled in, not the `just` variable

#### Success Criteria

- `just build && ./build/bin/technitium_exporter --version` prints a real
  version, commit, and date — not `dev (commit: none, built: unknown)`. **Met**
  — now reports `v0.3.0-9-g2a89950-dirty (commit: 2a89950, built: ...)`. Note
  kingpin writes `--version` to **stderr**, not stdout, so the recipe's
  verification echo needs `2>&1`; see L-13 in INV-0001
- `docker buildx bake dev --print` shows an `args` key on the target
- The version string contains no `{VERSION}` fragment and no bare PID

---

### Phase 3: Docker pipeline wiring

The image pipeline has never run end to end. Phase 2 fixed what the build
injects; this phase makes the recipes reachable and the bake invocation valid.

#### Tasks

- [x] Add `import 'docker.just'` to `justfile` (H-5) — `docker.just` documents
      this requirement in its own header but nothing does it
- [x] Add `group "default" { targets = ["dev"] }` to `docker-bake.hcl` (H-6) —
      bare `docker buildx bake` currently exits with
      `ERROR: failed to find target default`
- [x] Delete the `docker-push` recipe from `docker.just` (OQ-2a) — it runs
      `bake --push` against a `dev` target whose `output` is `type=docker`,
      which contradicts itself, and duplicates `docker-buildx`. Releases push
      from `ghcr.yml` on a tag, not from a laptop
- [x] Update `docker-bake.hcl:68`'s stale `make docker-push` comment reference
- [x] Confirm `just --list` shows the docker recipes — three, not four;
      `docker-push` was deleted per OQ-2a
- [x] **(unplanned)** Remove `set shell` from `docker.just`. `just` permits a
      setting to be defined only once across a justfile and its imports, so
      adding the import alone made `just` refuse to parse _anything_:
      `error: setting 'shell' first set on line 5 is redefined on line 7`
- [x] **(unplanned)** Export `VERSION` / `COMMIT_SHA` / `BUILD_DATE` from
      `docker.just`. Phase 2 taught `docker-bake.hcl` to forward build args, but
      nothing was setting the bake variables, so a local `just docker-build`
      still produced an image reporting `dev`

#### Success Criteria

- `just --list | grep docker` lists the docker recipes (currently returns
  nothing)
- `just docker-build` completes and loads an image into the local daemon
- `docker run --rm ghcr.io/donaldgifford/technitium_exporter:dev --version`
  reports the correct version — this is the end-to-end proof for Phases 2 and 3
  together. **Met**: reports
  `v0.3.0-11-g6b187af-dirty (commit: 6b187af, built: 2026-08-13T03:16:19Z)`,
  with OCI labels populated to match and the image running as `nonroot`
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

- [x] Fix `cliff.toml:50` — `$${2}` → `${2}` in `commit_preprocessors` (`$$` is
      an escaped literal `$` in the Rust regex replacer, so PR links currently
      render as `[#${2}](…/issues/${2})`) — landed in PR #28
- [x] Run `just changelog` and commit the regenerated `CHANGELOG.md`
- [x] Verify `just changelog-check` exits 0 — the sync commit itself is excluded
      by `cliff.toml`'s `^chore.*[Cc]hangelog` skip parser, so regenerating does
      not immediately re-stale the file. Note the standing friction this
      implies: every PR needs a trailing `chore(changelog):` commit (see
      Phase 6)
- [x] Confirm `CHANGELOG.md` is listed in `.prettierignore` so `just fmt-md`
      cannot reflow it back into drift — landed in PR #28

##### Markdown lint (H-8)

- [x] `git mv .markdowncilint.yml .markdownlint.yaml` — **not**
      `.markdownlint-cli2.yaml`; the file holds top-level plain rules
      (`MD013: false`, …), which is the format `.markdownlint.yaml` expects.
      Verified both ways: the cli2 wrapper name ignores top-level rules and
      MD013 keeps firing
- [x] Run `prettier --write "**/*.md"` first, then `markdownlint-cli2 --fix`
      (prettier's blank-line handling resolves most MD031/MD032 before
      markdownlint has to)
- [x] Hand-fix the 21 MD040 violations by adding a language to each bare code
      fence
- [x] Fix `MAINTAINERS.md:1` MD041 (add a top-level heading)
- [x] Fix `docs/llms.md:256` MD024 (duplicate "Key Rules" heading)
- [x] Stop prettier fighting the docz-generated surfaces (OQ-3a). Implemented
      **not** via `.prettierignore` as planned — that is whole-file only, and
      the 6 `docs/*/README.md` index files are mostly hand-written prose that
      should stay formatted. Wrapped each `<!-- BEGIN/END DOCZ -->` block in
      `<!-- prettier-ignore-start -->` / `<!-- prettier-ignore-end -->` instead,
      which sits outside the region `docz update` rewrites and so survives
      regeneration. Verified: `docz update` twice in a row is a no-op, and
      `just lint-md` passes in either order. Finished in Phase 8, once
      `docz update` had regenerated enough times to expose the second half: the
      `<!--toc:start-->` block docz injects into every doc body fights prettier
      the same way the index tables did. Same markers, but they need a **blank
      line** separating them from the TOC — placed flush, prettier reads the
      closing marker as a continuation of the last (indented) TOC list item and
      re-indents it, changing the very file it was supposed to leave alone.
      Verified stable over three prettier → docz → prettier cycles and with docz
      run last

- [x] Verify `just lint-md` exits 0
- [x] **(unplanned)** Add `.markdownlint-cli2.yaml` carrying `ignores:`.
      `.markdownlintignore` is a markdownlint-**cli** (v1) feature and is
      silently ignored by cli2, so the first attempt at excluding `CHANGELOG.md`
      and `.claude/` changed nothing. cli2 options and lint rules are separate
      files: rules stay in `.markdownlint.yaml`, `ignores` has to live in the
      cli2 config
- [x] **(unplanned)** Delete the stray second, empty
      `<!-- BEGIN/END DOCZ AUTO-GENERATED -->` pair at the tail of
      `docs/investigation/README.md` and `docs/plan/README.md`. Pre-existing;
      `docz update` populates the first pair only, so the second was dead weight
      that a future `docz` could have filled instead

##### Coverage gate (M-1)

- [x] Re-point `justfile:84`'s `internal/` prefix at `collector/` and
      `pkg/technitium/` only (OQ-4a). Current per-package coverage: `collector`
      96.9%, `pkg/technitium` 91.5%, `config` 0%, `exporter` 0%, `cmd` 0% — both
      gated packages clear the 60% floor comfortably today. Implemented as an
      explicit `gated_packages` list rather than a second path prefix, so adding
      a package is a one-word edit and the set is auditable at a glance
- [x] Leave a comment in the recipe naming `config/` and `exporter/` as
      deliberately ungated pending issue #15, so the omission reads as a
      decision rather than an oversight
- [x] Fix the recipe's inline comment, which claims `.codecov.yml` ignores
      `cmd/`; it actually ignores `main.go`, `docs`, `scripts`
- [x] Ensure the gate can actually fail — verify by temporarily lowering the
      floor above a real package's coverage
- [x] **(unplanned)** Treat a gated package with no rows in `coverage.out` as a
      failure rather than a silent skip. This is the same fail-open bug the
      `internal/` prefix had, one rename away from returning: without it, a typo
      in `gated_packages` retires the gate and still exits 0

##### Housekeeping

- [x] **(unplanned)** Prefix the explanatory comments inside the `build` and
      `fmt-go` recipe bodies with `@`. `just` echoes every un-prefixed recipe
      line before running it, comments included, so the Phase 2 comments were
      being printed into every local and CI build log

#### Success Criteria

- `just lint-md` exits 0 — **met**, 31 files, 0 errors, prettier clean
- `just changelog-check` exits 0 — **met**
- `just coverage-gate` reports at least one real package, and a deliberately
  raised floor makes it fail — **met**: reports `collector` 96.9% and
  `pkg/technitium` 91.5%; fails at floor 95 (one package), at floor 99 (both),
  on a stale package name, and when `coverage.out` is absent
- `just ci` passes end to end — the first time on this branch — **met**, exit 0
- A test commit of the form `feat: thing (#42)` renders as a working link in
  regenerated changelog output — **met**, renders as
  `[#42](https://github.com/donaldgifford/technitium_exporter/issues/42)`

---

### Phase 5: Retire the Makefile

`Makefile` cannot be deleted until its 5 unported targets exist in `justfile`; 4
of the 5 are documented in CLAUDE.md, so deleting early breaks documented
workflows. Note `trivy` also lost its `mise.toml` pin during the branch's
rewrite, so the tool needs re-adding, not just the recipe.

Neither `run-local` nor `test-api` may carry the hardcoded credentials forward
(Phase 1).

#### Tasks

- [x] Re-add the `trivy` pin to `mise.toml` with a `# renovate:` annotation
      matching the surrounding style — pinned `0.73.0`, matching the explicit
      pins used for the other linters rather than `latest`
- [x] Add `just trivy` —
      `trivy fs --scanners vuln --exit-code 1 --severity HIGH,CRITICAL .`
- [x] Add `just syft` — SBOM generation (SPDX + CycloneDX) into `build/`
- [x] Add `just security` — composite of `govulncheck` + `trivy`
- [x] Add `just run-local` reading `TECHNITIUM_URL` / `TECHNITIUM_TOKEN` from
      the environment, with a clear error when unset
- [x] Add `just test-api` — the two `curl` calls through `jq`, same env
      sourcing. Added `curl -f` while porting: without it an HTTP error status
      pipes the error body into `jq` and the recipe exits 0
- [x] Fix `justfile`'s `run` recipe (L-10): replace the donor-repo doc comment
      ("just run plan -config-dir ./approved-providers") and source credentials
      from the environment
- [x] Spot-check that `just fmt-go` preserves `make fmt`'s local-import grouping
      — `Makefile` passed `-local github.com/donaldgifford` explicitly;
      `just fmt-go` relies on `.golangci.yml`'s `gci`/`goimports` prefixes to do
      the same. Verified by scrambling `collector/collector.go`'s import block
      (local first, third-party interleaved with stdlib) and confirming
      `just fmt-go` restores it byte-identically
- [x] `git rm Makefile`
- [x] Remove both `Makefile` entries from `.github/labeler.yml`'s `ci` section
      (it is listed twice — L-8), keep `justfile`
- [x] Grep the repo for surviving `make` references and update them
- [x] **(unplanned)** Add `set dotenv-load := true`. The `Makefile` had
      `-include .env`; without an equivalent, the ported credential recipes
      would only ever work from an already-exported shell
- [x] **(unplanned)** Add `*.just` to `.github/labeler.yml`'s `ci` globs — Phase
      3 split the recipes across `justfile` and `docker.just`, and the
      `justfile` glob does not match the latter
- [x] **(unplanned)** Replace the four `Bash(make …)` entries in
      `.claude/settings.json` with `Bash(just *)`

#### Success Criteria

- `Makefile` is gone and `git grep -n "make "` returns no stale instructions —
  **met**. Completed planning and verification records (`docs/MVP.md`,
  `docs/exporter-enhancements-*`, `docs/security-tooling-plan.md`,
  `docs/review/`, and the drafted Support request) deliberately keep their
  `make` references: they record what was actually run at the time, and
  rewriting them would falsify the record
- `just security`, `just trivy`, `just syft` all run to completion — **met**,
  trivy reports 0 vulnerabilities in `go.mod`, syft writes both SBOMs
- `just run-local` fails with a clear message when credentials are unset, and
  works when they are — **met** for the guard; the live-server half is not
  exercised here because the token is pending rotation (Phase 1 owner action)
- Every command documented in CLAUDE.md's build section resolves to a real
  `just` recipe — **met**, all 13 verified via `just --show`

---

### Phase 6: Workflow hardening

Follow-up PR. None of these block the merge, but several waste CI time or fail
on fork PRs.

#### Tasks

- [x] M-4 — install golangci-lint via `mise-action` in CI and drop the
      `version:` input from `golangci-lint-action` (OQ-5a), making `mise.toml`
      the single source of truth. CI pins `v2.11.4` today, `mise.toml` pins
      `2.12.2`. **Implemented differently**: `golangci-lint-action` installs its
      own binary regardless of what is already on `PATH`, so mise-action plus
      that action would not have made `mise.toml` authoritative. Replaced the
      action with `mise-action` + `run: just lint`
- [x] M-6 — give `ci.yml`'s `build` job `fetch-depth: 0` (goreleaser needs tags;
      `package-lint` already has it), and align the two jobs on one
      `goreleaser-action` major (currently `v6` and `v7.1.0`). Also aligned
      `release.yml` from the floating `@v7` onto the same `@v7.1.0`, so the
      snapshot CI validates with is built by the same goreleaser that cuts the
      real release
- [x] M-6 — deduplicate the two full `goreleaser release --snapshot` runs across
      `build` and `package-lint` — merged into one `build` job; lintian and the
      SBOM scan now consume the same `dist/`
- [x] M-7 — fix the `labeler` job: the step is named "Checkout code" but is
      `actions/labeler@v6`, and `pull_request` yields a read-only token on fork
      PRs, so it will fail on external contributions. Renamed the step and
      guarded the job on the head repo being this repo. Chose skip-on-fork over
      `pull_request_target`, which would run with a writable token against an
      untrusted head ref — too much risk for cosmetic labels
- [x] M-8 — make `security.yml` consistent with `ci.yml`'s security job on
      whether `donaldgifford/govulncheck-action` needs a preceding checkout.
      Resolved by reading the action: it is a composite whose first step is its
      own `actions/checkout` (input `repo-checkout`, default true), and it runs
      `actions/setup-go` itself. So `security.yml` was already correct and
      `ci.yml` had a redundant checkout + setup-go. `ci.yml` keeps its checkout
      (trivy needs the tree) but now passes `repo-checkout: false`; the
      redundant `setup-go` is gone. Both files document the asymmetry
- [x] M-9 — add `concurrency` groups to all workflows; use
      `cancel-in-progress: false` for `changelog-regen.yml`, which pushes to
      `main` and can race with itself. Applied to 10 workflows (trufflehog
      already had one); `release.yml` and `ghcr.yml` also got
      `cancel-in-progress: false` on the same reasoning — cancelling a partial
      release or a partial manifest-list push is worse than a redundant run
- [x] M-10 — replace `license-check.yml`'s
      `go install github.com/google/go-licenses@latest` with the `mise.toml` pin
      via `mise-action`. Switched the steps to `just license-check` /
      `just license-report` too, so the licence allow-list is defined once in
      the justfile instead of being spelled out again inline
- [x] M-3 — remove `.github/` from `.yamllint.yml`'s ignore list (OQ-6a) and fix
      whatever surfaces across the 11 workflow files; also drop the dead
      `.charts/` and `config/testdata/section_key_dup.bad.yml` entries. Four
      errors surfaced, all `empty-values` on bare Actions triggers
      (`pull_request:`, `workflow_dispatch:`). Turned off
      `empty-values.forbid-in-block-mappings` rather than adding a
      `# yamllint disable-line` to every trigger in every workflow forever;
      yamllint has no per-path rule overrides. Added `build/` and `dist/` to the
      ignore list, which were being linted as if they were source
- [x] M-5 — keep both chglog and git-cliff (OQ-7a) and make the split explicit:
      add `just` recipes for the chglog side (deb package changelog) alongside
      the existing git-cliff ones (repo `CHANGELOG.md`), and document which
      artifact each one feeds — added `changelog-deb` and `changelog-deb-add`
      under a header comment stating the split
- [x] **(unplanned)** Make CI actually run the non-Go linters. No workflow ran
      yamllint, markdownlint, prettier, or actionlint — `just lint` covered them
      locally but CI only ever ran golangci-lint. That gap is precisely how H-8
      survived: a markdownlint config named `.markdowncilint.yml`, a name the
      tool never looks for, sat unread with nothing to catch it. Folded into the
      M-4 change, since the `lint` job now runs `just lint` wholesale

#### Success Criteria

- CI wall-clock drops measurably (one goreleaser snapshot run, not two) — **met
  by construction**; `build` and `package-lint` are one job. Not yet measured
  against a real run
- A PR from a fork does not fail the labeler job — **met**, the job is skipped
  when `head.repo.full_name` differs from the repo
- Local and CI golangci-lint report identical results on the same commit —
  **met**, both now resolve the binary from `mise.toml` and run `just lint-go`
- No workflow references a tool version that disagrees with `mise.toml` —
  **met**
- `just lint` passes with `.github/` no longer excluded from yamllint — **met**,
  0 errors; the 7 remaining `line-length` findings are warnings by config and
  all pre-date this phase

---

### Phase 7: Template residue cleanup

Follow-up PR. Cosmetic and correctness cleanup of values the template carried
over from the donor repo. Individually trivial; collectively they are what makes
the repo read as genuinely ported rather than copied.

#### Tasks

- [x] L-1 — remove the unsubstituted `/${project_name}` line from `.gitignore`
      and de-duplicate `*.test`, `*.out`, `.env`, `.idea/`, `.vscode/`, `dist/`
      (each now appears twice); fix the `make release-local` comment. Rewrote
      the file into one deduplicated set of sections; `/${project_name}` became
      `/technitium_exporter`, which is what a bare `go build ./cmd/...` drops at
      the root
- [x] L-2 — remove `.golangci.yml` exclusions for absent dependencies
      (`github.com/fatih/color`, `github.com/spf13/cobra` — this project uses
      kingpin), the `cmd/(compare|diff)\.go$` and `mock_.*\.go$` paths, and the
      blanket `G304:` gosec exclusion justified as "CLI tool reads
      user-specified file paths" (untrue of this exporter). All four verified
      dead before removal: neither dependency is in `go.mod`, `cmd/` holds only
      `main.go`, there are no `mock_*.go` files, and the exporter opens no files
      at all — so the G304 exclusion could only ever have hidden a future real
      finding
- [x] L-3 — remove `cobra-cli` and `mockery/v3` from `mise.toml`; neither is
      used and both are installed on every `mise-action` CI job
- [x] L-4 — fix `catalog-info.yaml`'s `technitium_expoerter` typo
- [x] L-4 — run `docz wiki` to generate the `mkdocs.yml` that
      `techdocs-ref: "dir:."` requires (OQ-8a); `.docz.yaml`'s `wiki` block is
      already configured for it (techdocs-core, nav titles, exclusions), so this
      looks like a step that was simply never run. Verify the generated nav
      covers all six docz types — **confirmed**, 26 pages, all six types present
- [x] L-5 — delete CODEOWNERS' "Replace @org/CHANGEME" instruction; the line
      below it is already correct
- [x] L-6 — remove `release.yml`'s duplicate syft install (lines 63-64 and
      72-73) and the `publish-ghcr` comment describing a `chart` job and "two
      publish workflows" that do not exist here
- [x] L-7 — fix `ghcr.yml`'s `ecr.yml` reference and its bare `# $schema=`
      directive (should be `# yaml-language-server: $schema=` pointing at
      `github-workflow.json`, as every other workflow does)
- [x] L-8 — fix `.github/labeler.yml`: the duplicate `Makefile` entry, and the
      `repo` globs naming `.goreleaser.yaml`, `.prettierrc.yaml`,
      `changelog.yaml` when the real files use `.yml`
- [x] L-9 — restore `code-quality` and `testing` to `scripts/labels.sh`'s
      colour/description maps, or drop them from `.github/labeler.yml`; they
      currently get created with the gray `EDEDED` fallback and no description.
      Added to both maps, with descriptions matched to the labels' current live
      values so a `--force` run is idempotent rather than overwriting them
- [x] L-11 — commit `.claude/settings.json` and ignore the rest of `.claude/`
      (OQ-9a): add `.claude/*` plus a `!.claude/settings.json` negation to
      `.gitignore`, replacing the narrow `donald-loop.local.md` rule. The glob
      form matters: git cannot re-include a file inside an ignored _directory_,
      so `.claude/` would have made the negation a no-op
- [x] **(unplanned)** Fix `.github/labeler.yml`'s `cmd/**.go` and `pkg/**.go`
      globs. Both packages keep their sources one directory deeper
      (`cmd/technitium_exporter/main.go`, `pkg/technitium/*.go`), which
      `dir/**.go` does not match — so changes to the exporter's entry point and
      its API client were never getting the `go` label. Now `dir/**/*.go`, which
      matches zero or more intervening segments. Also dropped the
      `renovate.json` and `.codecov.yaml` globs, alternate spellings of files
      this repo does not have. Every remaining glob was verified to resolve
- [x] **(unplanned)** Exclude the generated `mkdocs.yml` from yamllint and
      yamlfmt. `docz wiki update` writes it with 4-space indent and no document
      start, and reverts any formatting on the next run — verified by formatting
      it and re-running docz. Same class of problem as `CHANGELOG.md`, and the
      same resolution
- [x] **(unplanned)** Add `just docs-index` and `just docs-wiki` so regenerating
      the docz index tables and the mkdocs nav is a named step rather than a
      command someone has to remember

#### Success Criteria

- No file contains an unsubstituted template placeholder or a reference to a
  path, dependency, or workflow that does not exist in this repo — **met**;
  every `.github/labeler.yml` glob was checked to resolve against the tree
- `just lint` still passes — **met**
- `scripts/labels.sh --dry-run` proposes no gray-fallback labels — **met**, all
  17 labels resolve to an explicit colour and description

---

### Phase 8: Documentation refresh

Follow-up PR, last so it documents the settled state rather than a moving
target.

#### Tasks

- [x] L-12 — update CLAUDE.md's build-commands section: every `make X` becomes
      `just X` (done in Phase 5, where the recipes were ported)
- [x] L-12 — correct the Go version (documented 1.25.7, actual 1.26.5 per
      `go.mod`) — now 1.26.6, see the security bump below
- [x] L-12 — replace the `test.yml` workflow reference; this branch deletes it
      in favour of `ci.yml` (done in Phase 6, alongside the job merge)
- [x] L-12 — correct `golang/govulncheck-action@v1` to
      `donaldgifford/govulncheck-action@v1` and `trivy-action@0.33.1` to
      `v0.36.0`
- [x] Document the Docker workflow in CLAUDE.md — `Dockerfile`,
      `docker-bake.hcl`, `docker.just`, and the GHCR publish path are entirely
      undocumented. Added a Docker section covering the three bake targets, the
      three-layer version-metadata path, and the `$` vs `$$` trap
- [x] Document the changelog workflow and whichever tool survives OQ-7 — both
      survive; the new Changelog section tables which artifact each feeds and
      records the standing `chore(changelog): sync` requirement
- [x] Add a "Task Runner" section explaining `just --list` and the recipe groups
- [x] Update INV-0001's status to reflect that remediation has landed — stays
      `Concluded` (correct for an investigation) with a note pointing at this
      doc and naming the four still-open owner-actions
- [x] Run `docz update` to refresh the index tables
- [x] **(unplanned)** Bump Go 1.26.5 → 1.26.6 across `mise.toml`, `go.mod`,
      `Dockerfile`, and CLAUDE.md. `just govulncheck` began failing with exit 3
      on five stdlib advisories — GO-2026-6218 (`net/url`), GO-2026-6090
      (`crypto/tls`), GO-2026-6089 and GO-2026-5026 (`net/http`), GO-2026-5972
      (`encoding/asn1`) — all fixed in 1.26.6, and one with a live call path
      through `technitium.Client.doRequest`. Not a consequence of this plan; the
      advisories simply landed mid-flight. Left in rather than deferred because
      it fails `just ci`
- [x] **(unplanned)** Fold the `Packaging > Changelog` subsection into the new
      top-level Changelog section — two headings with the same text, which MD024
      correctly rejected

#### Success Criteria

- Every command in CLAUDE.md executes successfully as written — **met**, all 17
  distinct `just` invocations verified to resolve via `just --show`
- No reference to `make`, `test.yml`, or Go 1.25.7 survives — **met**
- `docz update --dry-run` reports no drift — **met**
- `just ci` green after the Go bump, with `just govulncheck` reporting no
  vulnerabilities — **met**

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

- [x] `just ci` — the composite gate; must pass before merge — **exit 0**
- [x] `just build && ./build/bin/technitium_exporter --version` — Phase 2 proof
- [x] `just docker-build && docker run --rm <image> --version` — Phase 3 proof.
      Image reports `v0.3.0-26-g928bf04 (commit: 928bf04, built: …)`, with the
      OCI `image.version` / `image.revision` labels matching and the container
      running as `nonroot:nonroot`
- [x] `just security`, `just trivy`, `just syft` — Phase 5 proof. All exit 0;
      trivy reports 0 HIGH/CRITICAL in `go.mod`, syft writes both SBOMs
- [x] `just run-local` with and without credentials set — Phase 5 proof.
      Verified **unset only**: the guard fires with instructions and exits 1.
      The credentials-present path is untested because the token is pending
      rotation (Phase 1 owner-action) — this is the one item below that a live
      server would close
- [ ] `git log -S <token> --all` returns empty — Phase 1 proof. **Not met, as
      expected.** Three commits still match (`00d895e`, `91bbe98`, `36c092a`),
      and `git for-each-ref --contains` shows every one of them is reachable
      _only_ from `refs/remotes/pr/*` — the local mirrors of GitHub's hidden
      `refs/pull/*`. `main` and every branch are clean; this is the same
      criterion as Phase 1's third bullet and it closes only when GitHub Support
      GCs the unreachable objects. Note `--all` is what makes this visible: a
      plain `git log -S` on the branches returns empty and would have been
      falsely reassuring
- [x] Open a throwaway PR to confirm `changelog.yml`, the labeler, and the
      required-labels check all behave — several findings only manifest in a
      real PR context. Done as PR #30 rather than a throwaway, since this branch
      is the change. All 12 checks pass. The three that could not be verified
      locally: `Label PR` passes under the new fork guard (same-repo PR takes
      the allowed branch); `Check Required Labels` passes; and in the merged
      `Build & Package` job, `Locate archive SBOM` succeeds — it `exit 1`s when
      the glob matches nothing — with `Upload SBOM scan SARIF` reporting
      `success` rather than `skipped`, which is what distinguishes a real SARIF
      upload from the guard swallowing an empty one (H-10)
- [x] Confirm the coverage gate fails when the floor is raised above a real
      package's coverage — fails at floor 95 (one package), at 99 (both), on a
      stale package name, and when `coverage.out` is absent

Regression watch: `just test` must stay green throughout. No phase should change
`collector/` or `pkg/technitium/`. **Held**: `just test` green at every commit,
and `git diff --name-only main...HEAD -- collector/ pkg/technitium/` returns
zero files.

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
