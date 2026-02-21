# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic
Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.2] - 2026-02-21

### Added

- **UI:** Visual feedback (loading spinners) on Record Create, Edit, and Delete
  actions to indicate processing state.

### Changed

- **Audit:** Overhauled audit log interface and backend to explicitly denote the
  targeted Zone Name automatically natively using cache joining.
- **Audit:** Audit entries now exhibit deep, granular `before -> after`
  comparisons precisely showcasing the changed payloads in DNS edits (instead of
  raw AWS outputs), and securely retain properties on deletions.

### Fixed

- **Core:** Implement octal unescaping for values coming from Route53, ensuring
  special symbols and wildcard prefixes (`*`) display their correct ASCII
  counterparts and not raw byte strings (e.g., `\052`).
- **Audit:** Fixed incorrect client IP logging when running behind a reverse
  proxy (e.g. Ingress Nginx) by inspecting `X-Forwarded-For` and `X-Real-IP`
  headers.

## [1.0.1] - 2026-02-19

### Added

- **LDAP:** Support for POSIX groups (RFC 2307) using `memberUid` attribute via
  new `group_filter` configuration option.
- **Scripts:** New `scripts/generate_ldap_config.sh` tool to automatically
  detect LDAP server characteristics (Active Directory vs OpenLDAP vs POSIX),
  validate group memberships, simulate role precedence, and generate a
  ready-to-use configuration.
- **Documentation:** Comprehensive `doc/LDAP_DEBUGGING.md` guide for
  troubleshooting authentication issues.

### Fixed

- **Build:** Corrected system version injection during Docker build process.
- **LDAP:** Improved group search logic in `internal/auth` to support custom
  filters and dynamic user/dn substitution.

## [1.0.0] - 2026-02-15

### Added

- Initial release of NS116 DNS Manager.
