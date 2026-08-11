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
   `go-version-file: 'go.mod'` in every `actions/setup-go` step, not `go-version: 'X.Y'` — a
   hardcoded version silently drifts from `go.mod`'s `go` directive whenever the module's Go
   version is bumped without updating every workflow file too.

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
   fork checked out in CI too**, or `go mod download`/`go vet`/golangci-lint fail at
   module-resolution time. `actions/checkout@v4` with a `path:` that escapes the initial working
   directory does NOT work (its sandbox rejects `path: ../engram-consensus-core` outright, no
   override) — use a plain `git clone` in a `run:` step instead:
   ```yaml
   - name: Checkout CometBFT fork (go.mod's local replace directive)
     run: |
       git clone --quiet https://github.com/cuongct924/engram-consensus-core.git ../engram-consensus-core
       git -C ../engram-consensus-core checkout --quiet <pinned-commit-sha>
   ```
   Pin the commit to the same one the local sibling checkout is on — not a moving `main`. Update
   this step in lockstep in both `go-test.yml` and `go-lint.yml` (golangci-lint resolves the module
   graph too). Run `scripts/check-fork-pin.sh` to verify both files agree and flag drift against
   the local `../engram-consensus-core` checkout.

7. **Run `black scripts/` locally before committing any Python change under `scripts/`.**
   `black --check` fails the whole CI job on a single unformatted file anywhere in the tree, not
   just files you touched — run the formatter itself rather than hand-fixing to satisfy it by eye.

## Before submitting a workflow change

- Confirm every `paths:` filter still matches real files in the repo (a stale glob that matches
  nothing means the workflow silently never runs — same failure class as a stale Go version, just
  quieter).
- If the change touched the CometBFT fork pin, run `scripts/check-fork-pin.sh` from the repo root.
- If you changed `go-version-file`, `golangci-lint-action` version, or added a new job, there's no
  local way to fully dry-run GitHub Actions — read the action's own version-compatibility notes
  (e.g. golangci-lint-action's README) rather than guessing at a version number.
