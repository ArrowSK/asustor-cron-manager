# Troubleshooting

Cron Manager is intentionally small, so most operational checks come down to the ADM service, the root crontab, and the local data directory.

## The ADM icon opens nothing

Check the service:

```sh
/usr/local/AppCentral/CronManager/CONTROL/start-stop.sh status
```

If it is stopped:

```sh
/usr/local/AppCentral/CronManager/CONTROL/start-stop.sh start
```

Then verify the local health endpoint:

```sh
curl -I http://127.0.0.1:18367/healthz
```

A healthy service returns HTTP `204`.

## The service is running but the browser cannot connect

Cron Manager accepts private, loopback, and link-local clients only.

Check that:

- you are connecting from the local/private network;
- port `18367` is not blocked by a local firewall rule;
- the ADM shortcut is pointing to the NAS address you expect;
- you are not trying to reach the app through a public reverse proxy.

Do not solve this by exposing the port directly to the internet.

## A job disappeared or changed unexpectedly

Inspect the real root crontab first:

```sh
crontab -l
```

Then inspect Cron Manager backups in the UI or on disk:

```text
/usr/local/AppCentral/CronManager/data/backups/
```

Every Cron Manager mutation saves the complete previous crontab before writing the new one.

## Restore a previous crontab

Prefer the Backups page in the UI. Restore itself is protected by another backup of the current state.

If the UI is unavailable, the `.cron` files under `data/backups/` are plain crontab snapshots and can be inspected manually before recovery.

## An imported ADM job has a strange name

Imported entries have no native display-name metadata. Cron Manager derives a best-effort label from the command while keeping the original cron line literal.

The displayed name is cosmetic. Editing the job lets you assign a proper managed-job name.

## A complex schedule is shown literally

This is intentional. Cron Manager humanizes only patterns it can describe without ambiguity.

For example, a simple hourly schedule may display as `Every hour at :17`, while a list/range expression can remain visible as the original cron expression.

The literal cron expression is authoritative.

## Run now fails

Remember that commands execute as root through:

```text
/bin/sh -c <command>
```

Check:

- command path exists;
- scripts are executable when required;
- the command does not depend on an interactive shell profile;
- required environment variables are declared in the command or crontab;
- filesystem/network resources are available to the NAS.

Managed run output is bounded to 64 KiB. Manual Run now has a 15-minute timeout.

## In-app update says no release is available

The updater requires a published GitHub Release, not just a commit or tag.

The release must contain:

```text
cron-manager_linux_arm64
cron-manager_linux_arm64.sha256
```

If those assets are absent, install the APKG manually from the release instead.

## App Central shows an older version than the UI

This can happen after an in-app binary update. The runtime binary changed, but App Central package metadata still reflects the last APKG installation.

Install the matching APKG to synchronize the displayed package version. See [Updates](UPDATES.md).

## Logs

The service log is:

```text
/usr/local/AppCentral/CronManager/cron-manager.log
```

For a concise diagnostic set:

```sh
/usr/local/AppCentral/CronManager/CONTROL/start-stop.sh status
/usr/local/AppCentral/CronManager/cron-manager version
curl -I http://127.0.0.1:18367/healthz
crontab -l
tail -n 100 /usr/local/AppCentral/CronManager/cron-manager.log
```

Redact real commands, tokens, hostnames, NAS addresses, and private paths before posting diagnostics publicly.
