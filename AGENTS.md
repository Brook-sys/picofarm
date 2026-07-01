# Picofarm Agent Notes

## Local utility script

There is a local-only helper script at:

```sh
scripts/dev.sh
```

It is intentionally excluded from Git via `.git/info/exclude` and must not be committed.

Useful commands:

```sh
scripts/dev.sh doctor
scripts/dev.sh status
scripts/dev.sh test
scripts/dev.sh build
scripts/dev.sh backend
scripts/dev.sh restart-backend
scripts/dev.sh stop-backend
scripts/dev.sh health
scripts/dev.sh logs
scripts/dev.sh lint
scripts/dev.sh frontend-build
scripts/dev.sh push
```

Defaults:

```sh
PICOFARM_BIN=/tmp/daedalus-current
PICOFARM_LOG=/tmp/opencode/daedalus-backend.log
PICOFARM_PORT=8084
PICOFARM_REMOTE=picofarm
PICOFARM_BRANCH=main
```

Use `scripts/dev.sh backend` after backend changes. It builds, restarts, and checks `/health`.

Use `scripts/dev.sh test` before finishing feature work when both backend and frontend may be affected.

Use `scripts/dev.sh logs` when backend health fails.

## Git policy

The user requested that code changes be committed from now on.

Do not commit local-only helper scripts. `scripts/dev.sh` is excluded locally.

Push normal commits to:

```sh
git push picofarm main
```

Never force-push unless the user explicitly asks.
