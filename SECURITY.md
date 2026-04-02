# Security Policy

## Supported Versions

| Version  | Supported          |
|----------|--------------------|
| 0.1.x    | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in kflashback, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please email: **security@kflashback.io**

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

## Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial assessment**: Within 5 business days
- **Fix timeline**: Depends on severity, typically within 30 days

## Security Practices

- Container images run as non-root with read-only filesystem
- RBAC follows least-privilege principle (read-only access to tracked resources)
- No secrets are stored in the history database by default
- All API endpoints are read-only (no mutation of cluster resources)
- Database is local to the controller pod (no external network access required)
- Distroless base image minimizes attack surface

## Disclosure Policy

We follow coordinated disclosure. After a fix is released, we will publish a security advisory on GitHub.
