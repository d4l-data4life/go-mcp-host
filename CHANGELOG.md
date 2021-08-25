# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

- Speed-up builds and tests by adding cache to Docker
- Streamline logging
- Replace deprecated UUID package `satori` by `gofrs`
- Upgrade go-svc to v1.17.0

### Deprecated

### Removed

### Fixed

- `ldflags` are now populated properly when running `go build` inside Docker

### Security

- Switch JWT lib to fix security issues
