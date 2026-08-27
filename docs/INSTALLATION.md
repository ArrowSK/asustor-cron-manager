# Installation and upgrades

This guide covers first installation, normal APKG upgrades, in-app runtime updates, verification, and uninstall behavior.

> [!IMPORTANT]
> Cron Manager manages the **root crontab**. Install and use it only on a NAS you administer.

## Requirements

Cron Manager 1.0 targets:

- ASUSTOR ADM 4.x or newer;
- ARM64 / aarch64 NAS models;
- an administrator performing the App Central installation;
- a trusted private network.

No Docker container, database, Go runtime, Node.js, PHP, Python, or separate web server is required.

## First installation

1. Open the [latest GitHub Release](https://github.com/ArrowSK/asustor-cron-manager/releases/latest).
2. Download `CronManager_<version>_arm64.apk`.
3. Optionally verify the adjacent `.sha256` file.
4. In ADM open **App Central → Manual Install**.
5. Select the APKG and approve the normal third-party application warning.
6. Launch **Cron Manager** from the ADM desktop.
7. Create a local password of at least 10 characters.

The ADM shortcut opens the app on port `18367`.

> [!WARNING]
> Keep port `18367` on a trusted LAN. Cron Manager serves HTTP and does not provide transport encryption by itself.

## What happens to an existing root crontab

Cron Manager reads the existing root crontab immediately.

Active cron entries it did not create are shown as **Imported**. Merely viewing, searching, or manually running an imported entry does not rewrite the original line.

An imported entry is adopted into a managed block only when an operation needs Cron Manager metadata, such as **Edit** or **Disable**.

Comments, blank lines, variables, and unrelated raw entries are preserved during mutations.

## Recommended first checks

After installation:

1. open the Jobs page and confirm the expected existing jobs are visible;
2. open Backups and confirm the page loads;
3. create a harmless test job or use Run now on a safe command;
4. confirm execution history records the run;
5. export managed jobs once so you are familiar with the recovery path.

## Portable migration

Use **Export & import → Export managed jobs** to download a portable JSON file.

On another Cron Manager installation choose:

- **Merge** — retain existing managed jobs and add only jobs that do not already exist by exact `schedule + command`;
- **Replace managed only** — remove current Cron Manager-managed blocks and replace them from the export.

Neither mode removes raw ADM/system entries.

The **full root crontab** export is for archival and recovery. It is intentionally not treated as a portable cross-device import format.

See [Import & export](IMPORT_EXPORT.md).

## In-app runtime update

Open **Settings → Application updates**.

Cron Manager checks the latest release in `ArrowSK/asustor-cron-manager`. If a newer release is available and contains the required standalone binary assets, the app can update itself without invoking undocumented ADM package-manager commands.

The update verifies both SHA-256 and the staged binary's reported version before activation.

See [Updates](UPDATES.md) for the full trust model and the App Central version-display caveat.

## Conventional APKG upgrade

A normal App Central upgrade is always supported:

1. download the newer `CronManager_<version>_arm64.apk` from the release;
2. optionally verify the checksum;
3. install it through **App Central → Manual Install** over the existing package.

The package ID remains `CronManager`, so the application state directory is reused.

Before a major upgrade, keeping a portable managed-jobs export in addition to the automatic crontab backups is sensible administrative hygiene.

## Uninstall behavior

Before a normal uninstall, Cron Manager attempts to convert every managed block back into a plain cron entry.

- enabled managed jobs become active cron lines;
- disabled managed jobs become commented cron lines.

If this conversion fails, the uninstall hook aborts rather than knowingly leaving scheduled entries that depend on a binary that is about to disappear.

## Verification over SSH

As root:

```sh
crontab -l
/usr/local/AppCentral/CronManager/CONTROL/start-stop.sh status
curl -I http://127.0.0.1:18367/healthz
/usr/local/AppCentral/CronManager/cron-manager version
```

Expected results:

- `crontab -l` prints the current root crontab;
- `start-stop.sh status` prints `running`;
- `/healthz` returns HTTP `204`;
- `cron-manager version` prints the runtime version.

## Files on the NAS

The application is installed under:

```text
/usr/local/AppCentral/CronManager/
```

Important paths include:

```text
cron-manager          application binary
cron-manager.log      service log
cron-manager.pid      ADM service pidfile
data/                  local authentication/history data
data/backups/          automatic root-crontab backups
CONTROL/               ADM package metadata and lifecycle scripts
```

Application data and backups are created with restrictive permissions.

## Next steps

- [Import & export](IMPORT_EXPORT.md)
- [Updates](UPDATES.md)
- [Troubleshooting](TROUBLESHOOTING.md)
- [Security](../SECURITY.md)
