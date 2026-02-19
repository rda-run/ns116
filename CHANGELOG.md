# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic
Versioning](https://semver.org/spec/v2.0.0.html).

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
