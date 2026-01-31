# Contributing to Ghostmail CLI

Thank you for your interest in contributing to Ghostmail CLI! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Development Setup](#development-setup)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Commit Message Guidelines](#commit-message-guidelines)
- [Code Review Process](#code-review-process)
- [Reporting Bugs](#reporting-bugs)
- [Requesting Features](#requesting-features)

## Development Setup

### Prerequisites

- Go 1.21 or later
- Git
- Make (optional, for using Makefile)

### Setting Up the Development Environment

1. **Fork and clone the repository:**
   ```bash
   git clone https://github.com/GodGMN/ghostmail-cli.git
   cd ghostmail-cli
   ```

2. **Install dependencies:**
   ```bash
   go mod download
   ```

3. **Verify the setup by running tests:**
   ```bash
   go test ./...
   ```

4. **Build the project:**
   ```bash
   go build ./cmd/ghostmail
   ```

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
├── .github/           # GitHub workflows and templates
├── go.mod
├── go.sum
├── Makefile
├── README.md
├── SKILL.md           # Agent skill guide
└── CONTRIBUTING.md    # This file
```

## Coding Standards

### Go Code Style

We follow standard Go conventions:

1. **Formatting:** Use `gofmt` to format all code
   ```bash
   gofmt -s -w .
   ```

2. **Linting:** Run `go vet` before submitting
   ```bash
   go vet ./...
   ```

3. **Naming Conventions:**
   - Use camelCase for unexported identifiers
   - Use PascalCase for exported identifiers
   - Use ALL_CAPS for constants
   - Avoid underscores in names (except in test files)

4. **Comments:**
   - All exported types, functions, and constants must have a doc comment
   - Comments should start with the name of the thing being described
   - Example: `// Send sends an email via SMTP`

### Error Handling

- Always check errors and handle them appropriately
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- Return errors rather than logging them in library code
- Use meaningful error messages that help users understand what went wrong

### Configuration

- All configuration must use environment variables with the `GHOSTMAIL_` prefix
- Never hardcode credentials or sensitive information
- Provide sensible defaults where appropriate

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with race detection
go test -race ./...

# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Writing Tests

- All new code should include unit tests
- Test files should be named `*_test.go`
- Use table-driven tests where appropriate
- Test both success and error cases
- Mock external dependencies (IMAP, SMTP) for unit tests

Example:
```go
func TestLoadConfig(t *testing.T) {
    tests := []struct {
        name     string
        envVars  map[string]string
        wantErr  bool
    }{
        // test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

## Pull Request Process

1. **Create a branch:**
   ```bash
   git checkout -b feature/your-feature-name
   ```
   Use descriptive branch names:
   - `feature/` for new features
   - `fix/` for bug fixes
   - `docs/` for documentation updates
   - `refactor/` for code refactoring

2. **Make your changes:**
   - Follow the coding standards
   - Add or update tests as needed
   - Update documentation if necessary

3. **Test your changes:**
   ```bash
   go test ./...
   go vet ./...
   go build ./cmd/ghostmail
   ```

4. **Commit your changes:**
   ```bash
   git add .
   git commit -m "feat: add new feature"
   ```

5. **Push to your fork:**
   ```bash
   git push origin feature/your-feature-name
   ```

6. **Create a Pull Request:**
   - Go to the GitHub repository
   - Click "New Pull Request"
   - Select your branch and compare with `main`
   - Fill in the PR template

## Commit Message Guidelines

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

- `feat:` A new feature
- `fix:` A bug fix
- `docs:` Documentation only changes
- `style:` Changes that don't affect the meaning of the code (formatting, etc.)
- `refactor:` A code change that neither fixes a bug nor adds a feature
- `perf:` A code change that improves performance
- `test:` Adding or correcting tests
- `chore:` Changes to the build process or auxiliary tools

Examples:
```
feat: add support for HTML email templates
fix: correct attachment size validation
docs: update README with Gmail setup instructions
test: add unit tests for config loading
```

## Code Review Process

1. All PRs require at least one review from a maintainer
2. Automated checks (CI) must pass
3. Address review comments promptly
4. Once approved, a maintainer will merge your PR

### What Reviewers Look For

- Correctness: Does the code work as intended?
- Tests: Are there adequate tests?
- Documentation: Is the code well-documented?
- Style: Does it follow Go conventions?
- Performance: Are there any obvious performance issues?
- Security: Are there any security concerns?

## Reporting Bugs

### Before Reporting

1. Check if the bug has already been reported
2. Try the latest version to see if it's already fixed
3. Review the documentation to ensure it's not expected behavior

### How to Report

Use the [Bug Report Template](.github/ISSUE_TEMPLATE/bug_report.md) and include:

- Clear description of the bug
- Steps to reproduce
- Expected behavior
- Actual behavior
- Environment details (OS, Go version, etc.)
- Relevant logs or error messages

## Requesting Features

### Before Requesting

1. Check if the feature has already been requested
2. Consider if it aligns with the project's goals
3. Be prepared to explain the use case

### How to Request

Use the [Feature Request Template](.github/ISSUE_TEMPLATE/feature_request.md) and include:

- Clear description of the feature
- Use case or problem it solves
- Proposed solution (if you have one)
- Alternatives considered

## Questions?

If you have questions about contributing, feel free to:

- Open an issue with the `question` label
- Reach out to the maintainers

Thank you for contributing to Ghostmail CLI!
