# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic
Versioning](https://semver.org/spec/v2.0.0.html).

## [1.2.0] - 2026-04-15

### Added

- **Auth/OIDC:** Added OpenID Connect authentication with support for multiple
  providers, callback handling, and automatic user provisioning flow.
- **UI/Theming:** Full light/dark theme system with `auto` mode support based on
  OS preference (`prefers-color-scheme`) and cookie-backed persistence via
  `ns116_theme_mode`.
- **UI/Header:** New theme switcher in the main header cycling through
  `Auto -> Claro -> Escuro -> Auto` with accessible focus styles and live
  theme re-application.

## [1.1.1] - 2026-04-09

### Added

- **Admin:** New "Duplicates" section to detect and group DNS records with
  identical values across all managed zones.
- **Admin:** Ability to "Ignore" specific duplicate groups, persisting these
  preferences in the database.
- **Admin:** Global "Reset Ignores" feature to clear all ignored duplicates.
- **Core:** Implemented asynchronous data loading pattern for both Duplicates
  and Records pages, enabling instant initial page loads.
- **UI:** Added Skeleton Loaders (pulsing placeholders) to provide visual
  feedback during background data fetching.
- **Audit:** Actions for ignoring and resetting duplicates are now automatically
  logged in the Audit Log.

### Fixed

- **Templates:** Added `seq` helper function to the global template `FuncMap` to
  prevent parsing errors when rendering repetitive UI elements like skeleton
  rows.

## [1.0.3] - 2026-02-23

### Changed

- **UI:** The Records list now displays short/abbreviated subdomain names by default (omitting the repetitive zone domain) in the Name column. The full FQDN remains accessible via mouse tooltip (hovering). Apex records are represented uniformly as `@`.

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
