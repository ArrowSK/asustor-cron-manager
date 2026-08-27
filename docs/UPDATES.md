# Updates

Cron Manager supports two upgrade paths:

1. **conventional APKG upgrade through App Central**;
2. **verified in-app runtime update from the official GitHub Release**.

The APKG remains the canonical installation package.

## In-app update flow

Cron Manager checks:

```text
https://api.github.com/repos/ArrowSK/asustor-cron-manager/releases/latest
```

A release is eligible for automatic runtime update only when it contains both:

```text
cron-manager_linux_arm64
cron-manager_linux_arm64.sha256
```

The update process is intentionally defensive:

1. query the latest release over HTTPS;
2. accept release assets only from trusted GitHub hosts;
3. download the checksum with a size bound;
4. download the ARM64 binary with a size bound;
5. verify SHA-256;
6. mark the staged binary executable;
7. execute `staged-binary version` and require it to match the release tag;
8. move the current binary to `cron-manager.previous`;
9. atomically rename the staged binary into place;
10. after the HTTP response is returned, replace the current process image with the updated binary.

The final process replacement uses `exec(2)`, so the PID remains unchanged and ADM's existing pidfile continues to point to the service.

If the final exec step fails, Cron Manager attempts to restore the `.previous` binary.

## Trust boundary

The checksum prevents accidental corruption or mismatched assets. It does **not** protect against compromise of the GitHub repository/account that publishes both the binary and checksum.

Repository/account security is therefore part of the updater trust boundary.

## App Central version display

The in-app updater replaces the runtime binary only. It intentionally does not call undocumented ADM package-manager internals.

That means the UI/runtime can report a newer version while **App Central still displays the version of the last installed APKG**.

Installing the matching APKG later synchronizes App Central's package metadata.

This mismatch is expected after a binary self-update and is not by itself an error.

## When an APKG upgrade is required

Use a conventional APKG upgrade when a release changes anything outside the embedded runtime binary, for example:

- ADM package metadata;
- lifecycle hooks;
- service startup behavior;
- filesystem layout;
- package-declared ports or dependencies.

Such releases should say so explicitly in the release notes.

## Manual verification

To verify the running binary over SSH:

```sh
/usr/local/AppCentral/CronManager/cron-manager version
```

To verify an APKG checksum after download:

```sh
sha256sum -c CronManager_<version>_arm64.apk.sha256
```

## Recovery files

During self-update the previous executable is retained as:

```text
/usr/local/AppCentral/CronManager/cron-manager.previous
```

This is a rollback aid for the update transaction, not a long-term version archive.
