# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Added
- IMAP message reads now include attachment filename, content type, and decoded size metadata.
- Published release artifacts include SHA-256 checksums.
- Fuzz targets for the MIME/text parsers (`FuzzExtractBody`, `FuzzStripHTML`, `FuzzDecodeHeader`, `FuzzSafeSlicePrefix`, `FuzzFormatQuotedReply`), run for a short duration in CI.
- CI now enforces a total coverage threshold (80%) and fails the build if coverage drops below it.
- Dependabot now groups dependency updates and uses `increase-if-necessary` for Go modules to reduce update churn.

### Fixed
- Restored the `cmd/ghostmail` executable entry point required to build release artifacts.
- `ghostmail config check` now exits unsuccessfully when required configuration is missing.
- The `v1.0.0` tag predates the `cmd/ghostmail` entry point and cannot produce a release artifact. It remains immutable for history and must not be treated as an installable release; `v1.0.1` is the corrective release.
- IMAP connections now apply a 15s connect/greeting timeout and a 60s per-command timeout, so a stalled server can no longer hang the CLI indefinitely.
- Full message reads no longer request `BODY[]` alongside a size-capped partial fetch, avoiding a double transfer of the message body.
- `reply --all` now strips display names before comparing/skipping the original sender and de-duplicating CC recipients.
- SMTP and IMAP configuration validation now rejects ports outside the 1-65535 range.

### Changed
- The project and release CI now require patched Go 1.25.12; direct and indirect Go dependencies were updated, including migration to maintained `github.com/wneessen/go-mail`.
- Release CI now verifies Go modules, pins GitHub Actions dependencies, and uses read-only permissions outside the publishing job.
- Expanded automated coverage for CLI validation, mail sending and parsing, configuration defaults and validation, and JSON output errors.
- Updated installation, reply, message-read, and Gmail authentication documentation to match current behavior.

### Security
- Added a vulnerability reporting policy and weekly dependency update checks for Go modules and GitHub Actions.
- SMTP and IMAP require TLS or STARTTLS by default, and message parsing enforces size limits for untrusted MIME content.

## [1.0.2] - 2026-08-07

### Fixed
- Removed a duplicate checksum file upload from the release workflow so the published `ghostmail-checksums.txt` only lists release binaries.

## [1.0.1] - 2026-08-07

### Added
- Re-published installable cross-platform release artifacts for Linux (amd64), macOS (amd64, arm64), and Windows (amd64), built from a commit that includes the `cmd/ghostmail` entry point.
- SHA-256 checksums attached to each release.

### Fixed
- Corrective release: `v1.0.0` could not build because `cmd/ghostmail/main.go` was missing. `v1.0.1` ships from a buildable commit.

## [1.0.0] - 2024-01-15 — NOT INSTALLABLE

### Added
- Initial release of Ghostmail CLI
- Send emails via SMTP with support for:
  - Multiple recipients (To, CC, BCC)
  - HTML and plain text bodies
  - File attachments (max 5 files, 10MB each)
  - Reply threading (In-Reply-To header)
  - Body from file or command line
- Read emails via IMAP with:
  - Mailbox listing with filters (unread, limit)
  - Full message reading with body and message metadata
  - JSON output for scripting
- Environment-based configuration via `GHOSTMAIL_*` environment variables
- JSON output mode for easy integration with scripts and agents
- Colored terminal output (can be disabled with `--no-color`)
- Cross-platform support for Linux, macOS, and Windows
- Comprehensive CLI help and documentation
- GitHub Actions CI/CD pipeline

### Commands
- `ghostmail send` - Send emails via SMTP
- `ghostmail inbox` - List emails from mailbox
- `ghostmail read` - Read specific email by UID
- `ghostmail reply` - Reply to an email
- `ghostmail config` - Configuration helper commands

### Security
- All credentials via environment variables (never stored in code)
- Support for Gmail App Passwords
- TLS/STARTTLS support for secure connections

> **Note:** The `v1.0.0` tag predates `cmd/ghostmail/main.go` and produces no
> installable binary. Do not `go install ...@v1.0.0`. Use `v1.0.1` or later.

[1.0.2]: https://github.com/GodGMN/ghostmail-cli/releases/tag/v1.0.2
[1.0.1]: https://github.com/GodGMN/ghostmail-cli/releases/tag/v1.0.1
[1.0.0]: https://github.com/GodGMN/ghostmail-cli/releases/tag/v1.0.0
