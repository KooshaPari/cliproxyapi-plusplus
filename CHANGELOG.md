# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.1.0] - 2026-03-28

### Added
- Comprehensive specification documents (PRD, FUNCTIONAL_REQUIREMENTS, ADR)
- User journeys and implementation planning documentation
- Standard CI workflow with vet and test automation
- Enhanced documentation site

### Security
- Prevent path-injection in resolveAuthPath with baseDir validation
- Clean baseDir on SetAuthDir to prevent traversal attacks
- Validate auth file names to prevent unsafe input

### Fixed
- Build errors and test failures across multiple packages
- os.WriteFile missing third argument in cmd/mcpdebug
- cursorproto.NewMsg to newMsg (unexported) in cmd/protocheck
- nil pointer dereference on resp.Request in amp/proxy
- Token repository and postgres store path-injection vulnerabilities

## [1.0.0] - 2026-01

### Added
- Initial cliproxyapi-plusplus release
- Multi-account routing for Cursor with round-robin and session isolation
- Management API for Cursor OAuth authentication
- Conversation checkpoint and session_id for multi-turn context
- Conductor cooldown integration with StatusError
- Auto-migration for sessions to healthy account on quota exhaustion
- Auto-identify accounts from JWT sub for multi-account support

### Features (from upstream)
- BmoPlus sponsorship support
- Multi-model payload rules matching
- Gitlab Duo panel parity improvements
- Consistent logging with log package

[Unreleased]: https://github.com/KooshaPari/cliproxyapi-plusplus/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/KooshaPari/cliproxyapi-plusplus/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/KooshaPari/cliproxyapi-plusplus/releases/tag/v1.0.0
