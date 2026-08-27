# Development guide

Cron Manager intentionally keeps a small dependency surface because it runs with root privileges on a NAS.

## Toolchain

- Go 1.23+
- POSIX shell
- `unzip` for convenient APKG inspection

There are no third-party Go modules and no frontend package manager.

## Repository checks

Before opening a pull request or publishing a release:

```sh
test -z "$(gofmt -l .)"
go test ./...
go vet ./...
bash scripts/build.sh
```

If `gofmt -l` prints any files, run:

```sh
gofmt -w .
```

## Local development

The production application needs root access because it manages the root crontab. Develop against a disposable Linux VM/container or provide a test `crontab` shim before exercising mutation paths.

A basic local build:

```sh
go build -o /tmp/cron-manager ./cmd/cron-manager
```

Example loopback launch:

```sh
sudo ACM_DATA=/tmp/acm-data \
     ACM_RUNNER=/tmp/cron-manager \
     ACM_LISTEN=127.0.0.1:18367 \
     /tmp/cron-manager
```

Then open:

```text
http://127.0.0.1:18367
```

> [!CAUTION]
> Do not point development builds at a production root crontab unless that is explicitly the test you intend to perform and you already have an independent backup.

## Build artifacts

Run:

```sh
bash scripts/build.sh
```

The build:

1. regenerates PNG icons from repository-owned logo geometry;
2. compiles a static Linux/ARM64 binary;
3. copies the standalone binary used by the in-app updater;
4. stages ADM `CONTROL/` files;
5. builds an APKG 2.0 archive;
6. writes SHA-256 files for the APKG and standalone binary.

Output:

```text
dist/
├── CronManager_<version>_arm64.apk
├── CronManager_<version>_arm64.apk.sha256
├── cron-manager_linux_arm64
└── cron-manager_linux_arm64.sha256
```

Generated PNGs are intentionally ignored by Git because they are reproducible from `cmd/mkicon` and the SVG source.

## APKG format

The package contains:

```text
apkg-version
control.tar.gz
data.tar.gz
```

`CONTROL/config.json` defines the ADM desktop application and port `18367`. Lifecycle scripts start/stop the single service, initialize local data permissions, and protect managed cron jobs during uninstall.

The repository-owned `cmd/mkapkg` tool builds APKG 2.0 packages with the Go standard library, avoiding a build-time dependency on external ASUSTOR packaging tools.

## Versioning

`VERSION` is the canonical application version string.

The Go build injects it into the runtime binary. The APKG packer substitutes it into ADM package metadata.

Do not maintain independent version strings in source code.

## CI

The CI workflow performs:

- formatting check;
- unit tests;
- `go vet`;
- ARM64/APKG build;
- APKG listing;
- build-artifact upload.

## Release process

1. update `VERSION`;
2. update `CHANGELOG.md`;
3. update `packaging/apkg/CONTROL/changelog.txt` when App Central-facing notes change;
4. run all local quality checks;
5. push the complete source tree to `main`;
6. CI validates the commit;
7. the Release workflow publishes the matching APKG and standalone updater assets if that version does not already have a release.

The release assets expected by the updater are:

```text
cron-manager_linux_arm64
cron-manager_linux_arm64.sha256
```

The normal installation assets are:

```text
CronManager_<version>_arm64.apk
CronManager_<version>_arm64.apk.sha256
```

## Import/export compatibility

Portable export documents have an explicit `formatVersion`.

If an incompatible change is ever necessary:

- increment the format version;
- preserve backward-reading support where practical;
- do not silently reinterpret existing fields;
- add tests covering old and new documents.

## Self-update compatibility

The updater is binary-oriented. Runtime and embedded-UI changes can update cleanly through the standalone binary.

Changes that require new ADM lifecycle hooks, package metadata, declared ports, or filesystem layout must be documented as requiring a conventional APKG upgrade.

## Testing priorities

Changes deserve focused tests when they affect:

- raw crontab preservation;
- managed-block parsing/rendering;
- backup-before-write behavior;
- cron expression validation;
- imported-entry adoption;
- portable import/export;
- authentication/session behavior;
- update URL/checksum/version verification;
- APKG lifecycle safety.
