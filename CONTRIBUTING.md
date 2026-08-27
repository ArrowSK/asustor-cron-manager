# Contributing

Cron Manager runs as root, so conservative changes, reversibility, and a small dependency surface matter more than feature count.

## Before a pull request

Run:

```sh
test -z "$(gofmt -l .)"
go test ./...
go vet ./...
bash scripts/build.sh
```

Add or update tests when changing parsing, schedule validation, crontab transformations, import/export, authentication, updater verification, or packaging behavior.

## Design rules

Contributions should preserve these invariants:

1. **The real system crontab remains the source of truth.**
2. **Do not build a second scheduler.**
3. **Preserve unrelated raw lines literally.**
4. **Every crontab mutation goes through backup-before-write.**
5. **Portable import must never silently replace raw ADM/system jobs.**
6. **Self-update must verify checksum and staged binary version before activation.**
7. **Avoid runtime dependencies unless they deliver a clear security or reliability benefit.**
8. **Prefer recoverable failure over clever automatic repair.**

## Pull request scope

Keep changes focused. A PR that changes cron parsing, updater trust, and UI styling at the same time is harder to review safely than three small PRs.

For behavior changes, describe:

- the user-visible effect;
- how existing crontabs are preserved;
- rollback/recovery behavior;
- tests added or changed.

## Documentation

Update the relevant documentation when behavior changes:

- installation/operations → `docs/INSTALLATION.md`;
- import/export → `docs/IMPORT_EXPORT.md`;
- updates → `docs/UPDATES.md`;
- architecture/invariants → `docs/ARCHITECTURE.md`;
- security boundary → `SECURITY.md`;
- user-facing release changes → `CHANGELOG.md`.

## Privacy

Never add real cron commands, hostnames, NAS IP addresses, access tokens, passwords, filesystem inventories, or other personal infrastructure to tests, issues, screenshots, examples, or documentation.
