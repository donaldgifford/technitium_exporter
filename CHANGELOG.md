# Changelog

All notable changes to this project are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
this project adheres to [Semantic Versioning](https://semver.org/).
## [unreleased]

### Bug Fixes

- *(makefile)* Remove hardcoded Technitium credentials
- *(ci)* Pin trufflehog to v3.96.0, harden workflow

### Miscellaneous Tasks

- Update repo

## [0.3.0] - 2026-02-12

### Features

- *(types)* Add chart data and top-entry types to StatsResponseData
- *(collector)* Add 6 metric descriptors and --collector.top-entries flag
- *(collector)* Add collection logic for chart data, top entries, and uptime

### Bug Fixes

- *(types)* Use Chart.js format for queryTypeChartData and protocolTypeChartData

### Documentation

- Update CLAUDE.md metrics table and mark Phase 6 verification complete
- Mark Phase 6 smoke test complete

### Testing

- *(client)* Add chart data and empty chart data parsing tests
- *(collector)* Add tests for chart data, top entries, and uptime metrics

## [0.2.0] - 2026-02-09

### Features

- Add security scanning targets to Makefile
- Add SBOM generation to goreleaser releases

### Bug Fixes

- Install syft in package-lint CI job
- Upgrade Go to 1.25.7 to resolve stdlib CVEs

### Documentation

- Update CLAUDE.md and plan with security tooling

### Miscellaneous Tasks

- Add govulncheck, trivy, and syft to mise.toml
- Add security scanning job to CI pipeline
- Install syft in release workflow for SBOM generation

## [0.1.0] - 2026-02-02

