# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Added ability to edit and regenerate user messages

### Changed

### Deprecated

### Removed

### Fixed

- Show error message if something goes wrong with the connection

### Security

## [v1.1.0] - 2025-11-03

### Fixed

- fix Helm chart issues
- renamed all missing occurrences of GO_SVC_TEMPLATE -> GO_MCP_HOST

## [v1.0.0] - 2025-11-02

### Added

- Full Model Context Protocol (MCP) implementation with stdio and HTTP transports
- MCP primitives support: Tools, Resources, and Prompts
- Dynamic capability negotiation and notification handling
- AI agent orchestration with multi-iteration planning and execution
- Ollama integration with streaming responses and function calling
- Conversation management with PostgreSQL persistence
- Multi-user support with JWT authentication
- RESTful API with WebSocket streaming for real-time responses
- Kubernetes deployment via Helm charts
- Public Go library API for embedding in applications
- Example programs demonstrating various usage patterns

[Unreleased]: https://github.com/d4l-data4life/go-mcp-host/compare/v1.1.0...HEAD
[v1.1.0]: https://github.com/d4l-data4life/go-mcp-host/compare/v1.0.0...v1.1.0
[v1.0.0]: https://github.com/d4l-data4life/go-mcp-host/releases/tag/v1.0.0
