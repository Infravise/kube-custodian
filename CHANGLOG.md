# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2024-12-29

### Added

- Ability to customize log level (Supporting `debug`, `info`, `warn`, `error`)

### Changed

- Moved related functions into their own Go packages for easier maintainability
- Optimized Docker image using multi-stage build

## [0.1.0] - 2024-12-28

### Added

- Ability to automatically delete pods with a `Succeeded` status
- Ability to automatically delete pods with a `Failed` status
- Ability to automatically delete any resource containing `kube-custodian/ttl` label (Supporting `wdhm` options: `1w3d6h30m`)
- Ability to automatically delete any resource containing `kube-custodian/expires` label (Supporting RFC3339 time format: `2024-12-30T15:00:00-00:00`)
- Full support for Kubernetes version `1.30`
