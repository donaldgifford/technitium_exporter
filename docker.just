# technitium_exporter — docker recipes
#
# Docker image build/push pipeline via docker buildx bake. Reads
# docker-bake.hcl for target definitions. Import this from the main
# justfile with `import 'docker.just'`.

# NOTE: no `set shell` here. Settings may only be defined once across a
# justfile and its imports, and the importing justfile already sets it --
# redefining it here makes `just` refuse to parse anything at all.

# ─── Docker ─────────────────────────────────────────────────────────

# docker-bake.hcl reads VERSION / COMMIT_SHA / BUILD_DATE from the environment
# and feeds them to the Dockerfile's ARGs. Without them the image builds fine
# but the binary inside reports "dev".
export VERSION := version
export COMMIT_SHA := commit_hash
export BUILD_DATE := build_date

# Local single-arch build via docker buildx bake
[group('docker')]
docker-build:
    @docker buildx bake

# Multi-arch build and push via docker buildx bake
[group('docker')]
docker-buildx:
    @docker buildx bake release

# NOTE: there is deliberately no `docker-push` recipe. It duplicated
# `docker-buildx` (the `release` target already pushes via
# output=type=registry), and pushes should come from ghcr.yml on a tag, where
# the image also gets cosign-signed -- not from a laptop.

# CI build (linux/amd64 only, for fast PR validation)
[group('docker')]
docker-ci:
    @docker buildx bake ci
