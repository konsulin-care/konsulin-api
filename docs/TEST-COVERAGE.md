# Test coverage and scanner reporting

Run the complete Go suite with race detection and instrumentation across packages:

```bash
go test -count=1 -race -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...
python3 scripts/ci/verify_go_coverage.py < coverage.out
go tool cover -html=coverage.out -o coverage.html
```

The reporter reads the profile from standard input only, so no command-line
value selects a file. It counts every block, including package-level function
literals, and merges duplicate blocks from tests that exercise other packages.
`go tool cover -func` can omit package-level function literals from its total.
Use `--minimum 100` to enforce exactly 100% without rounding up.
Statement coverage measures execution; assertions and end-to-end journeys
establish the behaviors exercised by those executions.

## Changed-line gate

Pull requests must cover every production Go statement they add or change.
The PR quality job appends the branch diff to the profile and fails below 100%:

```bash
base=$(git merge-base HEAD origin/develop)
{ cat coverage.out; git diff --unified=0 "$base" HEAD -- '*.go'; } |
  python3 scripts/ci/verify_go_coverage.py --module "$(go list -m)" --changed-minimum 100
```

A coverage block counts as changed when any added line falls inside it, and
`_test.go` files are ignored. The report lists each uncovered changed block.
Run the reporter's own tests with
`python3 -m unittest discover -s scripts/ci -p 'test_*.py'`.

## Local SonarQube

The scanner configuration defaults to `http://localhost:19000`. Run a local
SonarQube Community instance and create a local project and analysis token.
After generating `coverage.out`, run `sonar-scanner` with `SONAR_TOKEN` in its
environment and `-Dsonar.projectKey=konsulin-local`. Keep tokens out of source
files. A scanner running in Docker needs the server's Docker network address
instead of `localhost`; override `sonar.host.url` for that run.
Local analysis imports the report without cloud credentials and does not
update the upstream PR's SonarQube Cloud check.

## Optional SonarQube Cloud

Automatic analysis does not import coverage reports. Disable it in the project's
Administration → Analysis Method, then use CI-based analysis. Configure:

- Repository secret `SONAR_TOKEN` with analysis access to the project.
- Repository variable `SONAR_PROJECT_KEY` identifying the project.
- Repository variable `SONAR_ORGANIZATION` identifying its organization.

The PR quality job uploads `coverage.out` and runs the cloud scanner when all three
settings are present. `sonar-project.properties` imports the Go report;
production source files are not excluded from coverage.
GitHub withholds upstream credentials from fork PRs. Configure the fork's own
Sonar project and run **Repository coverage** manually on the feature branch.
This updates the fork project, not the upstream PR check. The upstream owner
must configure CI-based analysis there. Do not use `pull_request_target` to
expose upstream secrets to fork code.

## GitGuardian PR findings

GitGuardian scans every commit in a pull request, so editing a file does not
clear a finding recorded in an earlier commit. PR #304 reported two false
positives, both runtime interpolations without a literal value:

- Username Password in `docker-compose.ci.yml` (incident 36715412). The line
  comes from `develop` and interpolates `REDIS_PASSWORD`. The incident belongs
  to upstream workspace 763787, where a maintainer should still resolve it.
- Generic Password in `docker-compose.yml`, from `${VAR:?message}`
  interpolation. The file now uses `${VAR:?}`, which Compose rejects with
  "required variable … is missing a value".

The branch was squashed onto `develop`, so no PR commit carries either line.
Do not disable the detector or exclude the compose files to hide a warning.
