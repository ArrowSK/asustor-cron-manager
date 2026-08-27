# Security policy

Cron Manager edits and executes commands as **root**. Treat access to its UI with the same care as administrative SSH access.

## Supported versions

Security fixes target the latest stable release line, currently **1.x**.

## Security controls

Cron Manager keeps a deliberately narrow runtime surface:

- requests are accepted only from loopback, private, and link-local client addresses;
- first-run authentication requires a local password of at least 10 characters;
- passwords are stored using PBKDF2-HMAC-SHA-256 with a random 16-byte salt and 180,000 iterations;
- sessions use random 32-byte tokens, are kept in memory, expire after 12 hours, and are sent as HttpOnly/SameSite=Strict cookies;
- mutation endpoints require same-origin requests and JSON request bodies;
- browser assets are served with a restrictive Content Security Policy;
- request bodies, command output, release metadata, and update downloads are bounded;
- application state and backups use restrictive filesystem permissions;
- no telemetry, cloud account, plugin loader, external JavaScript runtime, or database server is used.

## Root command execution

Cron jobs are root commands by definition in this application. Managed and manual executions use:

```text
/bin/sh -c <command>
```

Cron Manager does not attempt to sandbox commands that the administrator explicitly schedules. The primary security boundary is therefore **who can access the administrative UI**.

## Network model

The ADM desktop application is served over HTTP on port `18367`.

The private-address check reduces exposure but does **not** provide transport encryption and does not protect against a hostile device already present on the trusted LAN.

Do not:

- forward port `18367` from a router;
- expose it directly to the public internet;
- publish it through an unauthenticated public reverse proxy;
- place the NAS interface on an untrusted network.

## First-run setup

Before authentication is configured, the first allowed private-network client can initialize the password.

Complete first-run setup promptly from a trusted LAN after installation.

## Update trust model

The in-app updater queries only the official repository release endpoint and accepts release assets only from trusted GitHub hosts.

Before activation it requires:

1. the expected ARM64 binary asset;
2. the matching SHA-256 asset;
3. a successful checksum match;
4. a staged-binary `version` result matching the release tag.

The current executable is preserved as `.previous` during the update transaction.

A checksum published beside a binary protects against corruption and mismatched assets. It does **not** defend against compromise of the GitHub account/repository that publishes both artifacts. Repository/account security is part of the update trust boundary.

See [Updates](docs/UPDATES.md).

## Exported data

Portable JSON exports and full crontab snapshots contain command lines. Commands may themselves contain usernames, private paths, tokens, or other secrets.

Treat exported files as administrative backup material and redact them before public sharing.

## Vulnerability reporting

Please use a **GitHub Security Advisory** for this repository instead of a public issue.

Include:

- affected version;
- reproduction steps;
- security impact;
- suggested mitigation if known.

Do not include real credentials, NAS addresses, private cron files, or personal infrastructure details unless strictly necessary and properly redacted.
