# Architecture

Cron Manager is built around one rule: **the system crontab stays authoritative**.

There is one long-running application process for the UI/API and no independent scheduling engine.

```text
Trusted LAN browser
       │ HTTP :18367
       ▼
┌────────────────────────────────────┐
│ cron-manager                       │
│                                    │
│  local auth + sessions             │
│  JSON API                          │
│  embedded HTML/CSS/JS              │
│  crontab parser/editor             │
│  import/export                     │
│  execution history                 │
│  GitHub release updater            │
└───────────────┬────────────────────┘
                │
                ├── crontab -l
                ├── crontab <stdin>
                ├── /bin/sh -c <command>
                └── data/{auth,history,backups}
```

Cron itself remains responsible for timing.

## Runtime components

### Service process

`cron-manager` serves the local UI and JSON API, handles authentication, reads/writes the root crontab, provides history and backup operations, and performs update checks.

The frontend is embedded in the binary. No external JavaScript framework or web server is required.

### Short-lived execution mode

Managed scheduled jobs call:

```text
cron-manager exec <id>
```

This is not a second daemon. Cron starts the binary for that job, the binary resolves the stored command from the managed block, executes it, records the result, returns the child exit code, and exits.

## Raw and managed entries

### Raw entries

Raw entries are active cron lines not wrapped by Cron Manager. They are displayed in the UI using ephemeral identifiers derived from their content/position.

Raw entries are kept literal unless the user explicitly performs an operation that requires adopting or changing that entry.

### Managed entries

Managed jobs use a self-describing block:

```text
# ACM-BEGIN <id>
# ACM-NAME <base64url>
# ACM-SCHEDULE <base64url>
# ACM-COMMAND <base64url>
# ACM-ENABLED true|false
<schedule> /usr/local/AppCentral/CronManager/cron-manager exec <id>
# ACM-END <id>
```

Metadata lives in the crontab itself rather than a private scheduler database.

The Base64URL values are encoding, not encryption.

## Mutation transaction

Every crontab mutation follows the same sequence:

1. read the current root crontab;
2. save a timestamped full backup;
3. perform the minimal requested raw-line or managed-block transformation;
4. install the complete result using the system `crontab` command;
5. retain the newest 50 backup snapshots.

Unknown comments, variables, blank lines, and unrelated raw jobs are not normalized.

If the resulting content is byte-for-byte identical to the current crontab, no write is performed.

## Execution history

Managed jobs use the execution path described above. Manual **Run now** uses the same command execution function.

Recorded information includes:

- start and completion timestamps;
- manual vs scheduled source;
- exit status;
- bounded combined output.

Manual execution has a 15-minute timeout. Captured output is capped at 64 KiB per run.

## Portable export format

Version 1 portable exports contain managed jobs only:

```json
{
  "format": "asustor-cron-manager",
  "formatVersion": 1,
  "appVersion": "1.0.0",
  "exportedAt": "2026-08-27T12:00:00Z",
  "sourceCrontabSha256": "...",
  "jobs": [
    {
      "name": "Hourly check",
      "schedule": "17 * * * *",
      "command": "/path/check.sh",
      "enabled": true
    }
  ]
}
```

Authentication, execution history, raw system entries, and internal IDs are intentionally excluded. Import creates fresh IDs.

Detailed semantics: [Import & export](IMPORT_EXPORT.md).

## Self-update architecture

The updater talks only to the latest-release endpoint for the official GitHub repository and requires the standalone ARM64 binary plus checksum assets.

The high-level transaction is:

```text
GitHub Release
     │
     ├── binary
     └── sha256
          │
          ▼
   staged download
          │
   checksum + version validation
          │
          ▼
 current → .previous
 staged  → current
          │
          ▼
      exec(2)
```

`exec(2)` preserves the PID, so ADM's existing pidfile remains valid across a successful runtime update.

See [Updates](UPDATES.md).

## Data and state

Runtime state lives under the application directory, primarily:

```text
data/
├── backups/
├── auth.json
└── history.json
```

Exact filenames are implementation details except where documented for recovery. Sensitive state is created with restrictive permissions.

The crontab itself remains the authoritative definition of scheduled jobs.

## Failure philosophy

Cron Manager is designed so that losing the GUI does not imply losing cron.

- raw jobs remain ordinary cron entries;
- managed jobs remain represented in the real crontab;
- uninstall converts managed jobs back to plain cron entries;
- every mutation has a rollback snapshot;
- the application does not need to stay awake to decide when jobs should run.
