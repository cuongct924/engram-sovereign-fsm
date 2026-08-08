---
name: github-actions-ci
description: Conventions for editing .github/workflows/*.yml in this repo. Use whenever adding a CI job, bumping a dependency that affects CI (Go version, golangci-lint config schema, Python version), or troubleshooting a CI failure.
---

# GitHub Actions conventions for this repo

Three workflows exist: `go-test.yml`, `go-lint.yml`, `python-scripts.yml`. Each is scoped to the
file types it cares about via `on.push.paths`/`on.pull_request.paths` — keep that scoping when
adding jobs; don't make every workflow run on every push.

## Hard rules

1. **Go version comes from `go.mod`, never a hardcoded string.** Use
   `go-version-file: 'go.mod'` in every `actions/setup-go` step, not `go-version: 'X.Y'`. A
   hardcoded version silently drifts from `go.mod`'s `go` directive and breaks CI the next time
   someone bumps the module's Go version without remembering to also edit every workflow file —
   this exact bug (`go-version: '1.21'` against a `go 1.26.1` module) was found and fixed once
   already; don't reintroduce it.

2. **`.golangci.yml`'s `version:` key is the config schema version, not a version pin for the
   linter binary.** This repo's config uses `version: "2"` (the v2 schema). `golangci-lint-action`
   must be v6 or later to run a golangci-lint binary that understands that schema — v3-v5 of the
   action default to v1.x binaries that will fail to parse the config. If bumping `.golangci.yml`'s
   schema version, check whether `golangci-lint-action`'s version needs bumping too, and vice versa.

3. **Test job runs `go vet ./...` before `go test ./...`, as a separate step.** `go vet` failures
   (e.g. passing a proto message by value where a pointer is required — see the `go-spec-fidelity`
   skill) are cheap to catch early and easy to miss if buried inside a passing test run's output.

4. **Action versions track upstream majors, not "whatever was there before."** When touching a
   workflow file for any reason, check whether `actions/checkout`, `actions/setup-go`,
   `actions/setup-python` are on a current major (v4 checkout, v5 setup-go/setup-python as of this
   writing) — old majors use deprecated Node runtimes GitHub is phasing out. Don't do a
   repo-wide version-bump sweep unprompted, but do fix versions in any file you're already editing.

5. **`python-scripts.yml` is scoped to `scripts/**/*.py` and `requirements.txt` only** — it runs
   `black --check`, `flake8`, and installs `requirements.txt` plus `flake8`/`black`/`pytest` ad hoc
   (there's no `requirements-dev.txt` yet). If `scripts/` gains real pytest test files (currently
   none exist — see `CLAUDE.md`'s "Current status", M7 is not yet done), add a `pytest` step rather
   than assuming one is already wired.

6. **`go.mod`'s local `replace github.com/cometbft/cometbft => ../engram-consensus-core` needs the
   fork checked out in CI too, or `go mod download`/`go vet`/golangci-lint all fail immediately at
   module-resolution time.** That path is relative to the repo root and only exists on a machine
   that also has the sibling `engram-consensus-core` checkout — `actions/checkout@v4` only fetches
   the one repo it's pointed at, so a bare `go-test.yml`/`go-lint.yml` breaks the instant this
   `replace` directive was added (confirmed as the actual root cause of a real CI failure, not
   guessed). Fix: both workflows need a second `actions/checkout@v4` step, before any `go`/lint
   step, checking out `cuongct924/engram-consensus-core` (confirmed public — no token needed) with
   `path: ../engram-consensus-core` (resolves relative to `github.workspace`, landing as a sibling
   of the main repo's checkout root, exactly matching the `replace` directive's relative path).
   Pin `ref:` to the same commit/branch the local sibling checkout is actually on, not just
   "whatever `main` happens to be" — a moving target there makes CI depend on a fork commit nobody
   deliberately chose for that run. If the `replace` directive's target path or the fork's remote
   ever changes, this checkout step must be updated in lockstep, in both workflow files.

7. **Run `black scripts/` locally before committing any Python change under `scripts/`, don't wait
   for CI to report formatting failures.** `black --check` fails the whole job on a single
   unformatted file, and it does not selectively check only the files you touched — the whole
   `scripts/` tree needs to already be black-clean before you push, including files you didn't
   personally edit this round (confirmed live: `black --check scripts/` failed on 24 files
   simultaneously, most never touched in the change that surfaced the failure). Don't hand-fix
   formatting to satisfy black by eye — run the formatter itself and let it be idempotent.

## Before submitting a workflow change

- Confirm every `paths:` filter still matches real files in the repo (a stale glob that matches
  nothing means the workflow silently never runs — same failure class as a stale Go version, just
  quieter).
- If you changed `go-version-file`, `golangci-lint-action` version, or added a new job, there's no
  local way to fully dry-run GitHub Actions — read the action's own version-compatibility notes
  (e.g. golangci-lint-action's README) rather than guessing at a version number.
