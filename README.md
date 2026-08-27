<div align="center">

<img src="assets/logo.svg" width="132" alt="Cron Manager logo">

# Cron Manager for ASUSTOR ADM

**A small, native GUI for the real root crontab on ASUSTOR NAS.**

Manage scheduled jobs safely from ADM without adding Docker, a database, or a second scheduler.

[![CI](https://github.com/ArrowSK/asustor-cron-manager/actions/workflows/ci.yml/badge.svg)](https://github.com/ArrowSK/asustor-cron-manager/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/ArrowSK/asustor-cron-manager?display_name=tag&sort=semver)](https://github.com/ArrowSK/asustor-cron-manager/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![ASUSTOR ADM](https://img.shields.io/badge/ASUSTOR-ADM%204.x%2B-315f9f)](https://www.asustor.com/)
[![ARM64](https://img.shields.io/badge/architecture-arm64-257f7a)](#compatibility)
[![MIT](https://img.shields.io/badge/license-MIT-172231)](LICENSE)

[Install](docs/INSTALLATION.md) · [Import & export](docs/IMPORT_EXPORT.md) · [Updates](docs/UPDATES.md) · [Security](SECURITY.md) · [Troubleshooting](docs/TROUBLESHOOTING.md)

</div>

---

Cron Manager is intentionally conservative: **cron remains the scheduler and the system crontab remains the source of truth**. The app reads that crontab, preserves entries it does not own, creates a full rollback copy before every mutation, and adds metadata only to jobs that you explicitly manage.

> [!IMPORTANT]
> Cron Manager edits commands that run as **root**. Keep its web interface on a trusted LAN and treat access to it with the same care as administrative SSH access.

## Why Cron Manager

ASUSTOR exposes cron, but managing root jobs normally means SSH, manual crontab edits, and careful handling of ADM-created entries. Cron Manager adds a focused ADM interface without replacing cron with another service stack.

| Principle | What it means in practice |
|---|---|
| **Native and lightweight** | One static ARM64 Go binary with embedded HTML, CSS, and JavaScript. |
| **No second scheduler** | Cron still decides when jobs run. If Cron Manager is stopped, ordinary cron keeps working. |
| **Preserve what you do not own** | ADM jobs, variables, comments, blank lines, and unrelated raw entries are left literal unless you explicitly modify that entry. |
| **Backup before change** | Every crontab mutation saves the complete previous root crontab first. The newest 50 backups are retained. |
| **Portable managed jobs** | Export managed jobs to JSON, then merge or replace managed jobs on another installation. |
| **Observable runs** | Managed executions record time, source, exit status, and bounded output. |
| **Verified updates** | In-app updates use the official GitHub Release, SHA-256 verification, staged binary validation, and atomic replacement. |
| **Local by design** | Private/LAN clients only, local password authentication, no telemetry, no cloud account. |

## At a glance

```text
ADM / trusted LAN browser
          │
          │ HTTP :18367
          ▼
┌──────────────────────────────────────┐
│ Cron Manager                         │
│                                      │
│  Jobs · Backups · Import/Export      │
│  History · Settings · Updates        │
└──────────────────┬───────────────────┘
                   │
                   ├── crontab -l
                   ├── crontab <stdin>
                   └── /bin/sh -c <command>
                              │
                              ▼
                         system cron
```

Cron Manager does not poll jobs and does not maintain its own timing engine.

## Features

### Job management

- list the existing root crontab;
- create, edit, enable, disable, and delete jobs;
- **Run now** for managed or imported entries;
- hourly, daily, weekly, monthly, and startup presets;
- validation for standard five-field cron syntax and supported `@macros`;
- human-readable schedule descriptions where they can be stated safely;
- literal schedule display for complex expressions rather than misleading simplification.

Imported system entries are visible immediately. Merely viewing them does not rewrite them. Editing or disabling an imported entry adopts only that entry into a Cron Manager managed block.

### Backups and restore

Before any write, Cron Manager stores a timestamped copy of the **entire current root crontab**. Restore is protected the same way: the current crontab is backed up before an older copy is installed.

The app retains the newest **50** backups.

### Import and export

Cron Manager separates portability from disaster recovery:

- **Managed jobs JSON** — portable between Cron Manager installations;
- **Full root crontab** — archival text snapshot for inspection and recovery.

Portable import offers two modes:

- **Merge** — keep current managed jobs and skip exact `schedule + command` duplicates;
- **Replace managed only** — replace Cron Manager-managed blocks while leaving raw ADM/system lines untouched.

See [Import & export](docs/IMPORT_EXPORT.md) for the file format and exact semantics.

### Execution history

Managed jobs execute through a short-lived `cron-manager exec <id>` call. Cron Manager records:

- start time;
- finish time;
- scheduled vs manual source;
- exit code;
- bounded combined output.

Manual **Run now** has a 15-minute safety timeout. Captured output is capped at 64 KiB per run.

### In-app updates

Cron Manager can update its runtime directly from the official GitHub Release:

1. check the latest release;
2. download `cron-manager_linux_arm64` and its checksum asset;
3. verify SHA-256;
4. execute the staged binary and require its reported version to match the release tag;
5. retain the previous executable as `.previous`;
6. atomically activate the new binary;
7. replace the running process image in place.

A normal APKG upgrade remains available at all times. Read [Updates](docs/UPDATES.md) for the important distinction between the runtime version and App Central package metadata.

## Compatibility

Version 1.0 targets:

- **ASUSTOR ADM 4.x or newer**;
- **ARM64 / aarch64** NAS models;
- installation by an ADM administrator;
- a trusted private network.

There are no runtime dependencies on Docker, Node.js, PHP, Python, a database, or a separate web server.

## Quick install

1. Open the [latest release](https://github.com/ArrowSK/asustor-cron-manager/releases/latest).
2. Download `CronManager_<version>_arm64.apk`.
3. In ADM open **App Central → Manual Install**.
4. Select the APKG and approve the third-party application warning.
5. Launch **Cron Manager** from the ADM desktop.
6. Create a local password of at least 10 characters.

The ADM shortcut opens port `18367` on the NAS.

> [!WARNING]
> Do not expose port `18367` directly to the public internet. Cron Manager uses LAN HTTP and does not provide transport encryption by itself.

Full instructions: [Installation and upgrades](docs/INSTALLATION.md).

## How managed jobs are represented

A managed job remains self-describing inside the real crontab:

```cron
# ACM-BEGIN 0123456789abcdef
# ACM-NAME TmlnaHRseSBiYWNrdXA
# ACM-SCHEDULE MTcgKiAqICogKg
# ACM-COMMAND L3BhdGgvdG8vYmFja3VwLnNo
# ACM-ENABLED true
17 * * * * /usr/local/AppCentral/CronManager/cron-manager exec 0123456789abcdef
# ACM-END 0123456789abcdef
```

The metadata is Base64URL encoded to avoid comment and shell-quoting ambiguity. It is **not encryption**.

On normal uninstall, Cron Manager attempts to convert managed blocks back into ordinary cron entries so scheduled commands do not depend on an application that has been removed.

## Safety model

The design deliberately favors reversibility over cleverness:

- the real crontab is authoritative;
- unrelated raw lines are not normalized;
- every mutation goes through backup-before-write;
- portable import cannot replace raw ADM/system lines;
- authentication state is stored locally with restrictive permissions;
- passwords use PBKDF2-HMAC-SHA-256 with a random salt;
- sessions are random, in-memory, HttpOnly, SameSite=Strict, and expire after 12 hours;
- mutation requests require same-origin JSON requests;
- browser assets use a restrictive Content Security Policy;
- no telemetry, external frontend runtime, plugin loader, or cloud account is used.

Read the full [security policy](SECURITY.md) before changing the network exposure model.

## Build from source

Requirements: Go 1.23+ and a POSIX shell.

```bash
git clone https://github.com/ArrowSK/asustor-cron-manager.git
cd asustor-cron-manager

gofmt -w .
go test ./...
go vet ./...
bash scripts/build.sh
```

Build output:

```text
dist/
├── CronManager_<version>_arm64.apk
├── CronManager_<version>_arm64.apk.sha256
├── cron-manager_linux_arm64
└── cron-manager_linux_arm64.sha256
```

The APKG is the canonical installation package. The standalone binary pair is used by the in-app updater.

## Repository structure

```text
cmd/cron-manager/       service and cron execution entry point
cmd/mkapkg/             dependency-free APKG 2.0 packer
cmd/mkicon/             reproducible PNG logo renderer
internal/auth/          local password and session handling
internal/cron/          parser, validation, backups, import/export, mutations
internal/history/       lightweight locked execution history
internal/server/        HTTP API and embedded web interface
internal/update/        GitHub release checker and verified self-updater
packaging/apkg/CONTROL/ ADM package metadata and lifecycle hooks
assets/                 original project logo
scripts/                build and package automation
docs/                   operator and developer documentation
.github/workflows/      CI and release automation
```

## Documentation

| Document | Purpose |
|---|---|
| [Installation](docs/INSTALLATION.md) | First install, upgrades, uninstall behavior, SSH verification |
| [Import & export](docs/IMPORT_EXPORT.md) | Portable JSON format, merge/replace semantics, recovery exports |
| [Updates](docs/UPDATES.md) | In-app binary update trust model and APKG upgrade path |
| [Architecture](docs/ARCHITECTURE.md) | Runtime model, crontab representation, mutation transaction |
| [Troubleshooting](docs/TROUBLESHOOTING.md) | Common operational checks and recovery steps |
| [Development](docs/DEVELOPMENT.md) | Build, test, package, and release workflow |
| [Security](SECURITY.md) | Threat model, controls, update trust, vulnerability reporting |
| [Contributing](CONTRIBUTING.md) | Project design rules and contribution checks |
| [Changelog](CHANGELOG.md) | Release history |

## License

Cron Manager is released under the [MIT License](LICENSE).

---

<div align="center">
<sub>Keep cron simple. Add a safer interface, not another scheduler.</sub>
</div>
