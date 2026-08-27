# Import and export

Cron Manager deliberately separates **portable job migration** from **full-crontab recovery**.

## Managed jobs export

**Export managed jobs** downloads a JSON document containing Cron Manager-managed jobs only.

It does **not** include:

- raw ADM/system entries;
- comments or unrelated crontab variables;
- passwords or sessions;
- execution history;
- internal managed-job IDs.

A version 1 export looks like this:

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

The `sourceCrontabSha256` value is informational. Import assigns fresh internal IDs.

## Import validation

Before changing the crontab, Cron Manager validates:

- export format name;
- export format version;
- job count limit;
- non-empty job names;
- maximum job-name length;
- cron schedule syntax;
- non-empty, single-line commands.

A rejected import does not modify the crontab.

## Merge mode

**Merge** keeps the current managed jobs and appends imported jobs that are not already represented by the same exact:

```text
schedule + command
```

Exact duplicates are skipped. Job names alone do not define duplication.

Raw ADM/system lines remain untouched.

## Replace managed only

**Replace managed jobs only** removes the current Cron Manager-managed blocks and appends the jobs from the export.

It does **not** remove or rewrite raw ADM/system lines.

This makes the mode useful for restoring a known managed-job set without treating the whole NAS crontab as portable configuration.

## Backup before import

Both import modes use the same backup-before-write transaction as normal edits. The complete current root crontab is saved before the imported result is installed.

## Full root crontab export

**Download full root crontab** creates a plain text snapshot of the current root crontab.

Use it for:

- administrative archives;
- manual inspection;
- disaster recovery;
- comparing changes outside the app.

Do not assume a full root crontab from one NAS is safe to install on another NAS. ADM applications and system jobs may differ between devices.

## Sensitive information

Cron commands can contain paths, usernames, tokens, or other secrets. Treat both JSON exports and full crontab snapshots as administrative backup material.

## Compatibility policy

Portable exports include an explicit `formatVersion`.

Future incompatible format changes must use a new version and preserve backward-reading logic where practical. Existing fields should not be silently repurposed with different meanings.
