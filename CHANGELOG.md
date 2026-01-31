# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2024-01-15

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
  - Full message reading with body and attachments
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

[1.0.0]: https://github.com/GodGMN/ghostmail-cli/releases/tag/v1.0.0
