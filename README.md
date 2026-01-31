# 👻 Ghostmail CLI

A lightweight, fast command-line email client for sending and reading emails via SMTP/IMAP. Built with Go for cross-platform compatibility.

[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

## Features

- 📤 **Send emails** via SMTP with support for:
  - Multiple recipients (To, CC, BCC)
  - HTML and plain text bodies
  - File attachments (max 5 files, 10MB each)
  - Reply threading (In-Reply-To header)
  - Body from file or command line
- 📥 **Read emails** via IMAP with:
  - Mailbox listing with filters (unread, limit)
  - Full message reading with body and attachments
  - JSON output for scripting
- 🔒 **Environment-based configuration** - no credentials stored in code
- 🤖 **JSON output mode** for easy integration with scripts and agents
- 🎨 **Colored terminal output** (can be disabled)
- 🖥️ **Cross-platform** - works on Linux, macOS, and Windows

## Table of Contents

- [Installation](#installation)
- [Configuration](#configuration)
- [Commands](#commands)
  - [send](#send)
  - [inbox](#inbox)
  - [read](#read)
  - [config](#config)
- [Environment Variables](#environment-variables)
- [Examples](#examples)
- [JSON Output](#json-output)
- [Troubleshooting](#troubleshooting)
- [Development](#development)
- [License](#license)

## Installation

### From Source

```bash
go install github.com/user/ghostmail-cli/cmd/ghostmail@latest
```

### Pre-built Binaries

Download the latest release for your platform from the [Releases](https://github.com/user/ghostmail-cli/releases) page.

```bash
# Linux
curl -L https://github.com/user/ghostmail-cli/releases/latest/download/ghostmail-linux-amd64 -o ghostmail
chmod +x ghostmail
sudo mv ghostmail /usr/local/bin/

# macOS
curl -L https://github.com/user/ghostmail-cli/releases/latest/download/ghostmail-darwin-amd64 -o ghostmail
chmod +x ghostmail
sudo mv ghostmail /usr/local/bin/

# Windows (PowerShell)
Invoke-WebRequest -Uri https://github.com/user/ghostmail-cli/releases/latest/download/ghostmail-windows-amd64.exe -OutFile ghostmail.exe
```

### Building from Source

```bash
git clone https://github.com/user/ghostmail-cli.git
cd ghostmail-cli
make build
```

## Configuration

Ghostmail uses **environment variables** for configuration. No credentials are ever stored in code or config files.

### Quick Setup

```bash
# Copy example configuration
eval "$(ghostmail config example)"

# Or export manually
export GHOSTMAIL_SMTP_HOST="smtp.gmail.com"
export GHOSTMAIL_SMTP_PORT="587"
export GHOSTMAIL_SMTP_USERNAME="your-email@gmail.com"
export GHOSTMAIL_SMTP_PASSWORD="your-app-password"
export GHOSTMAIL_SMTP_FROM="your-email@gmail.com"

export GHOSTMAIL_IMAP_HOST="imap.gmail.com"
export GHOSTMAIL_IMAP_PORT="993"
export GHOSTMAIL_IMAP_USERNAME="your-email@gmail.com"
export GHOSTMAIL_IMAP_PASSWORD="your-app-password"
```

### Gmail Setup

For Gmail, you'll need to use an [App Password](https://support.google.com/accounts/answer/185833):

1. Enable 2-Factor Authentication on your Google account
2. Go to Google Account → Security → 2-Step Verification → App passwords
3. Generate an App Password for "Mail"
4. Use that password instead of your regular password

### Configuration Check

Verify your configuration:

```bash
ghostmail config check
```

## Commands

### send

Send an email via SMTP.

```bash
ghostmail send --to recipient@example.com --subject "Hello" --body "World"
```

**Required Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--to` | `-t` | Recipient email address (repeatable) |
| `--subject` | `-s` | Email subject |
| `--body` | `-m` | Email body text (or use `--body-file`) |

**Optional Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--cc` | `-c` | CC recipient (repeatable) |
| `--bcc` | `-b` | BCC recipient (repeatable) |
| `--attach` | `-a` | File attachment (repeatable, max 5 files, 10MB each) |
| `--body-file` | | Read body from file |
| `--html-file` | | Read HTML body from file |
| `--in-reply-to` | | Message-ID to reply to (for threading) |

**Examples:**

```bash
# Simple text email
ghostmail send --to recipient@example.com --subject "Hello" --body "World"

# With CC and BCC
ghostmail send \
  --to recipient@example.com \
  --cc cc@example.com \
  --bcc bcc@example.com \
  --subject "Hello" \
  --body "World"

# Multiple recipients
ghostmail send \
  -t user1@example.com \
  -t user2@example.com \
  -s "Hello" \
  -b "World"

# With attachments
ghostmail send \
  --to recipient@example.com \
  --subject "Documents" \
  --body "Please find attached" \
  --attach document.pdf \
  --attach image.png

# HTML email with plain text fallback
ghostmail send \
  --to recipient@example.com \
  --subject "Newsletter" \
  --html-file newsletter.html \
  --body "Plain text version"

# Body from file
ghostmail send \
  --to recipient@example.com \
  --subject "Report" \
  --body-file report.txt

# Reply to a message (enables threading)
ghostmail send \
  --to recipient@example.com \
  --subject "Re: Original Subject" \
  --body "My reply text" \
  --in-reply-to "<original-message-id@example.com>"
```

### inbox

List emails from a mailbox.

```bash
ghostmail inbox [flags]
```

**Flags:**
| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--limit` | `-l` | Maximum messages to show (0 = all) | 20 |
| `--unread` | `-u` | Show only unread messages | false |
| `--mailbox` | `-m` | Mailbox to list | INBOX |

**Examples:**

```bash
# List last 20 emails (default)
ghostmail inbox

# List with custom limit
ghostmail inbox --limit 10

# Show only unread messages
ghostmail inbox --unread

# List from specific mailbox
ghostmail inbox --mailbox "Sent Items"

# JSON output for scripting
ghostmail inbox --limit 50 --json

# Get unread count (with jq)
ghostmail inbox --unread --json | jq '.messages | length'
```

### read

Read a specific email by UID.

```bash
ghostmail read --uid <UID>
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--uid` | `-u` | Message UID (required) |
| `--mailbox` | `-m` | Mailbox to read from | INBOX |
| `--raw` | | Show raw/preview only (faster) |

**Examples:**

```bash
# Read email with UID 12345
ghostmail read --uid 12345

# Read from specific mailbox
ghostmail read --uid 12345 --mailbox Archive

# Quick preview (faster, no body parsing)
ghostmail read --uid 12345 --raw

# Get JSON for scripting
ghostmail read --uid 12345 --json

# Extract subject using jq
ghostmail read --uid 12345 --json | jq -r '.message.subject'
```

### config

Configuration helper commands.

```bash
# Print example configuration
ghostmail config example

# Check current configuration
ghostmail config check

# Source example config (edit first!)
eval "$(ghostmail config example)"
```

## Environment Variables

All configuration is done via environment variables with the `GHOSTMAIL_*` prefix.

### SMTP Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GHOSTMAIL_SMTP_HOST` | SMTP server hostname | (required) |
| `GHOSTMAIL_SMTP_PORT` | SMTP server port | `587` |
| `GHOSTMAIL_SMTP_USERNAME` | SMTP username | (required) |
| `GHOSTMAIL_SMTP_PASSWORD` | SMTP password | (required) |
| `GHOSTMAIL_SMTP_FROM` | Default sender email | (same as username) |
| `GHOSTMAIL_SMTP_USE_TLS` | Use TLS (instead of STARTTLS) | `false` |
| `GHOSTMAIL_SMTP_STARTTLS` | Use STARTTLS | `true` |

### IMAP Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GHOSTMAIL_IMAP_HOST` | IMAP server hostname | (required) |
| `GHOSTMAIL_IMAP_PORT` | IMAP server port | `993` |
| `GHOSTMAIL_IMAP_USERNAME` | IMAP username | (required) |
| `GHOSTMAIL_IMAP_PASSWORD` | IMAP password | (required) |
| `GHOSTMAIL_IMAP_USE_TLS` | Use TLS for IMAP | `true` |
| `GHOSTMAIL_IMAP_MAILBOX` | Default mailbox | `INBOX` |

### Example `.env` File

```bash
# ~/.ghostmail-env
export GHOSTMAIL_SMTP_HOST="smtp.gmail.com"
export GHOSTMAIL_SMTP_PORT="587"
export GHOSTMAIL_SMTP_USERNAME="your.email@gmail.com"
export GHOSTMAIL_SMTP_PASSWORD="xxxx xxxx xxxx xxxx"
export GHOSTMAIL_SMTP_FROM="your.email@gmail.com"

export GHOSTMAIL_IMAP_HOST="imap.gmail.com"
export GHOSTMAIL_IMAP_PORT="993"
export GHOSTMAIL_IMAP_USERNAME="your.email@gmail.com"
export GHOSTMAIL_IMAP_PASSWORD="xxxx xxxx xxxx xxxx"
export GHOSTMAIL_IMAP_MAILBOX="INBOX"
```

Source it: `source ~/.ghostmail-env`

## Global Flags

These flags work with all commands:

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | `-j` | Output in JSON format |
| `--no-color` | | Disable colored output |
| `--verbose` | `-v` | Enable verbose output |
| `--help` | `-h` | Show help |
| `--version` | | Show version |

## JSON Output

All commands support JSON output with the `--json` flag for easy integration:

### Send Response

```bash
ghostmail send --to test@example.com -s "Test" -b "Body" --json
```

```json
{
  "success": true,
  "message": "Email sent successfully"
}
```

### Inbox Response

```bash
ghostmail inbox --limit 5 --json
```

```json
{
  "success": true,
  "messages": [
    {
      "uid": 12345,
      "subject": "Hello",
      "from": "sender@example.com",
      "to": ["recipient@example.com"],
      "date": "2024-01-15T10:30:00Z",
      "flags": ["\\Seen"]
    }
  ],
  "total": 1
}
```

### Read Response

```bash
ghostmail read --uid 12345 --json
```

```json
{
  "success": true,
  "message": {
    "uid": 12345,
    "subject": "Hello",
    "from": "sender@example.com",
    "to": ["recipient@example.com"],
    "cc": [],
    "bcc": [],
    "date": "2024-01-15T10:30:00Z",
    "body": "Email body content...",
    "body_preview": "Email body...",
    "flags": ["\\Seen"]
  }
}
```

## Examples

### Shell Script Integration

```bash
#!/bin/bash

# Load configuration
source ~/.ghostmail-env

# Send daily report
ghostmail send \
  --to "boss@example.com" \
  --cc "team@example.com" \
  --subject "Daily Report $(date +%Y-%m-%d)" \
  --body-file report.txt \
  --attach daily-report.pdf
```

### Python Integration

```python
import subprocess
import json
import os

# Set environment
os.environ['GHOSTMAIL_IMAP_HOST'] = 'imap.gmail.com'
os.environ['GHOSTMAIL_IMAP_USERNAME'] = 'user@gmail.com'
os.environ['GHOSTMAIL_IMAP_PASSWORD'] = 'app-password'

# Get unread emails
result = subprocess.run(
    ['ghostmail', 'inbox', '--unread', '--json'],
    capture_output=True,
    text=True
)

emails = json.loads(result.stdout)
for msg in emails['messages']:
    print(f"From: {msg['from']}, Subject: {msg['subject']}")
```

### Automation with Cron

```bash
# Check for unread emails every 15 minutes
*/15 * * * * source $HOME/.ghostmail-env && ghostmail inbox --unread --json | /usr/local/bin/notify.sh

# Send daily backup report
0 9 * * * source $HOME/.ghostmail-env && ghostmail send -t admin@example.com -s "Backup Status" -b "Backup completed"
```

## Troubleshooting

### Common Errors

**"SMTP host is required"**
```
Error: SMTP host is required (set GHOSTMAIL_SMTP_HOST)
```
- Solution: Set the `GHOSTMAIL_SMTP_HOST` environment variable
- Example: `export GHOSTMAIL_SMTP_HOST="smtp.gmail.com"`

**"IMAP host is required"**
```
Error: IMAP host is required (set GHOSTMAIL_IMAP_HOST)
```
- Solution: Set the `GHOSTMAIL_IMAP_HOST` environment variable
- Example: `export GHOSTMAIL_IMAP_HOST="imap.gmail.com"`

**"too many attachments"**
```
Error: too many attachments: maximum is 5 (you have 7)
```
- Solution: Attachments are limited to 5 files maximum
- Combine files into a zip archive or send multiple emails

**"attachment is too large"**
```
Error: attachment file.pdf is too large: 15.2MB (max 10MB)
```
- Solution: Individual attachments are limited to 10MB
- Use a file sharing service for large files

### SMTP Connection Issues

- Verify SMTP host and port are correct
- For Gmail: Use App Password, not your regular password
- Check if your ISP blocks port 587 (try 465 with TLS)
- Verify `GHOSTMAIL_SMTP_STARTTLS` is set correctly for your provider

### IMAP Connection Issues

- Verify IMAP is enabled in your email provider settings
- Some providers require "Less Secure Apps" to be enabled
- Check firewall settings
- For Gmail: Use App Password instead of regular password

### Debug Mode

Use `--verbose` flag to see detailed error messages:

```bash
ghostmail send --to test@example.com -s "Test" -b "Body" --verbose
```

### Getting Help

All commands include detailed help:

```bash
ghostmail --help
ghostmail send --help
ghostmail inbox --help
ghostmail read --help
ghostmail config --help
```

## Development

### Project Structure

```
ghostmail-cli/
├── cmd/ghostmail/      # Main application entry point
├── internal/
│   ├── cli/           # CLI commands (cobra)
│   ├── config/        # Configuration management
│   ├── email/         # SMTP/IMAP clients
│   └── output/        # Output formatting
├── pkg/email/         # Public types/interfaces
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### Building

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Run tests
make test

# Clean build artifacts
make clean
```

### Makefile Targets

| Target | Description |
|--------|-------------|
| `build` | Build binary for current platform |
| `build-linux` | Build for Linux (amd64) |
| `build-darwin` | Build for macOS (amd64, arm64) |
| `build-windows` | Build for Windows (amd64) |
| `build-all` | Build for all platforms |
| `test` | Run tests |
| `clean` | Clean build artifacts |
| `install` | Install to $GOPATH/bin |

## Security Notes

- **Never commit credentials** to version control
- Use environment variables or a secrets manager
- For Gmail, always use App Passwords
- Consider using a dedicated email account for automation
- The `config check` command masks passwords in output

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Acknowledgments

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [go-imap](https://github.com/emersion/go-imap) - IMAP client library
- [Gomail](https://gopkg.in/gomail.v2) - SMTP email library
- [Color](https://github.com/fatih/color) - Terminal colors
