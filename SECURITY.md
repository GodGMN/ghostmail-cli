# Security Policy

## Supported Versions

Security fixes are applied to the latest release on the `master` branch. Older
release lines receive fixes only at the maintainers' discretion; upgrade to the
newest tag whenever possible.

| Version | Supported          |
|---------|--------------------|
| latest `v1.x` tag on `master` | :white_check_mark: |
| older tags                    | :x:                |

## Reporting a Vulnerability

Please do not report security vulnerabilities through public GitHub issues.
Use [GitHub's private vulnerability reporting](https://github.com/GodGMN/ghostmail-cli/security/advisories/new) to provide a description, steps to reproduce, impact, and any suggested remediation.

If private reporting is unavailable, contact the repository maintainers privately. Please allow a reasonable time for a response and coordinated fix before public disclosure. We aim to acknowledge reports within a few business days.

## Disclosure

Coordinated disclosure is preferred. Once a fix is available we publish a new
release tag and, for significant issues, a GitHub Security Advisory with credit
to the reporter (unless they prefer to remain anonymous).

## Known Security Considerations

- **`GHOSTMAIL_*_ALLOW_INSECURE=true`** disables the TLS/STARTTLS requirement
  and transmits credentials in plaintext. It exists only for explicitly trusted
  plaintext servers (e.g. local relays) and is unsafe on untrusted networks.
- The release pipeline runs `govulncheck` and fails on any *reachable*
  vulnerability. An advisory in a required-but-not-called module does not block
  the release; see the latest CI run for the current scan result.
