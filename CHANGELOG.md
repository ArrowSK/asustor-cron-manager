# Changelog

All notable user-facing changes are documented here.

## 1.0.1 — 2026-08-27

Patch release fixing the Settings updater UI.

- Fixed `Check for updates` failing after the automatic first update check with `Cannot set properties of null (setting 'textContent')`.
- Kept the current-version DOM anchor stable when the update status panel is re-rendered.
- Hardened the post-update restart path so a missing version element cannot break recovery feedback.

## 1.0.0 — 2026-08-27

First stable release.

### Cron management

- Native ARM64 ASUSTOR ADM application for the real root crontab.
- List, create, edit, enable, disable, delete, and Run now.
- Conservative handling of imported ADM/system jobs.
- Improved imported-job naming for leading environment assignments and shell wrappers.
- Safe schedule descriptions: complex/list expressions remain literal rather than being described incorrectly.
- Validation for five-field cron expressions and supported `@macros`.

### Safety and portability

- Full root-crontab backup before every mutation and restore.
- Retention of the newest 50 rollback snapshots.
- One-click restore from the UI.
- Portable managed-job JSON export.
- Managed-job import with Merge and Replace-managed-only modes.
- Full root crontab text export for archival/recovery use.
- Managed blocks convert back to plain cron entries on normal uninstall.

### Runtime and updates

- Single static Go ARM64 binary with embedded dependency-free web UI.
- Execution history with bounded output for managed jobs.
- GitHub Release update checking from Settings.
- Verified binary self-update using SHA-256 plus staged-binary version validation.
- Atomic binary activation with `.previous` rollback copy and in-place process replacement.
- Release workflow publishes both APKG and standalone self-update assets.

### Security

- Private/LAN client-address boundary.
- PBKDF2-HMAC-SHA-256 local password storage.
- Random HttpOnly/SameSite sessions.
- Same-origin and JSON-only mutation protections.
- Restrictive Content Security Policy and bounded request/download sizes.
- No telemetry, third-party frontend runtime, database, Docker, or cloud-account dependency.

### Documentation and presentation

- Original Cron Manager logo with reproducible icon generation.
- Responsive ADM-oriented interface.
- Stable-release documentation covering installation, import/export, updates, architecture, development, security, and troubleshooting.
