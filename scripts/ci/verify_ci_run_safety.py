#!/usr/bin/env python3
"""Definition-of-done checks for the run-block injection hardening
(Semgrep findings 947594694 and 947594695).

Each check_* group maps to one task in the implementation plan:

  T-1  pr.yml gofumpt step passes base_ref through env (no ${{ in run:)
  T-2  ci-env "Write CI env files" env-maps all 9 inputs, keeps every
       credential in the generated files, and adds umask 077
  T-3  ci-env readiness probe passes the redis password through env

Repo-wide rule: no ${{ may appear inside any run: block under .github/.

The behavior check executes the extracted "Write CI env files" script in a
temp directory with hostile values (quotes, $(), backticks, semicolons,
embedded newlines) and asserts:
  - generated .env.ci / docs/api/.env / GITHUB_ENV are byte-identical to the
    contract (values land literally, nothing is evaluated)
  - no marker file appears (no command injection)
  - files are created under umask 077 -> mode 0600

Run from repo root:  python3 scripts/ci/verify_ci_run_safety.py
Exit code is non-zero if any check fails.
"""

import os
import re
import stat
import subprocess  # nosec B404 - check_behavior executes repo-owned scripts in a temp dir
import sys
import tempfile
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[2]
GH = ROOT / ".github"
OPEN = "${{"

WORKFLOW_RELS = sorted(str(p.relative_to(ROOT)) for p in (GH / "workflows").glob("*.yml"))
ACTION_RELS = sorted(str(p.relative_to(ROOT)) for p in (GH / "actions").glob("**/action.y*ml"))
ALL_RELS = WORKFLOW_RELS + ACTION_RELS
CI_ENV_ACTION_REL = ".github/actions/ci-env/action.yml"

FAILED = []


def check(name, condition, detail=""):
    if condition:
        print("PASS  " + name)
    else:
        FAILED.append(name)
        print("FAIL  " + name + ("  " + detail if detail else ""))


def load_yaml(rel):
    with (ROOT / rel).open(encoding="utf-8") as fh:
        return yaml.safe_load(fh)


def all_steps(doc):
    """Yield (step_name, step_dict) for workflow jobs and composite runs.steps."""
    for job in (doc.get("jobs") or {}).values():
        for step in job.get("steps") or []:
            if isinstance(step, dict):
                yield step.get("name") or step.get("uses") or "(step)", step
    for step in (doc.get("runs") or {}).get("steps") or []:
        if isinstance(step, dict):
            yield step.get("name") or "(composite step)", step


def get_step(rel, want):
    for name, step in all_steps(load_yaml(rel)):
        if name == want:
            return step
    return None


def indented_run_blocks(text):
    """Yield run: bodies via indentation, independent of YAML parsing."""
    lines = text.splitlines()
    runs = []
    for i, line in enumerate(lines):
        m = re.match(r"^(\s*)run:\s*[|>]", line)
        if not m:
            continue
        indent = len(m.group(1))
        body = []
        for j in range(i + 1, len(lines)):
            nxt = lines[j]
            if nxt.strip() == "" or len(nxt) - len(nxt.lstrip()) > indent:
                body.append(nxt)
            else:
                break
        runs.append("\n".join(body))
    return runs


def bare_refs(text):
    """$VAR references that appear outside double quotes, single quotes, or # comments."""
    lines = [ln for ln in text.splitlines() if not ln.lstrip().startswith("#")]
    stripped = re.sub(r'"[^"]*"', '""', "\n".join(lines))
    stripped = re.sub(r"'[^']*'", "''", stripped)
    return re.findall(r"\$[A-Z_][A-Z0-9_]*", stripped)


def check_yaml_valid():
    for rel in ALL_RELS:
        try:
            load_yaml(rel)
            check("yaml parses [" + rel + "]", True)
        except Exception as exc:  # noqa: BLE001 - any parse failure is a DoD break
            check("yaml parses [" + rel + "]", False, str(exc))


def check_repo_wide_no_interpolation():
    for rel in ALL_RELS:
        text = (ROOT / rel).read_text(encoding="utf-8")
        bad = [b for b in indented_run_blocks(text) if OPEN in b]
        detail = bad[0].splitlines()[0].strip() if bad else ""
        check("no " + OPEN + " in run: blocks [" + rel + "]", not bad, detail)


def check_task1():
    step = get_step(".github/workflows/pr.yml", "gofumpt formatting check (changed files)")
    check("T-1 gofumpt step found", step is not None)
    if step is None:
        return
    run = step.get("run") or ""
    env = step.get("env") or {}
    check("T-1 BASE_REF env mapping", env.get("BASE_REF") == OPEN + " github.base_ref }}")
    check("T-1 merge-base uses quoted $BASE_REF", 'git merge-base HEAD "origin/$BASE_REF"' in run)
    check("T-1 gofumpt stays pinned via go tool", "go tool gofumpt -l" in run)
    check("T-1 no " + OPEN + " in gofumpt run", OPEN not in run)
    check("T-1 BASE_REF only ever double-quoted", "BASE_REF" not in bare_refs(run))


ENV_MAP = {
    "XENDIT_SANDBOX_API_KEY": OPEN + " inputs.xendit-sandbox-api-key }}",
    "CI_POSTGRES_PASSWORD": OPEN + " inputs.ci-postgres-password }}",
    "CI_REDIS_PASSWORD": OPEN + " inputs.ci-redis-password }}",
    "CI_RABBITMQ_PASSWORD": OPEN + " inputs.ci-rabbitmq-password }}",
    "CI_SUPERTOKEN_API_KEY": OPEN + " inputs.ci-supertoken-api-key }}",
    "CI_SUPERADMIN_API_KEY": OPEN + " inputs.ci-superadmin-api-key }}",
    "CI_XENDIT_CALLBACK_TOKEN": OPEN + " inputs.ci-xendit-callback-token }}",
    "CI_JWT_HOOK_KEY": OPEN + " inputs.ci-jwt-hook-key }}",
    "ORGANIZATION": OPEN + " inputs.organization }}",
}
ENV_CI_KEYS = (
    "POSTGRES_PASSWORD",
    "REDIS_PASSWORD",
    "RABBITMQ_DEFAULT_PASS",
    "SUPERTOKEN_API_KEY",
    "SUPERADMIN_API_KEY",
    "XENDIT_CALLBACK_TOKEN",
)


def check_task2():
    rel = CI_ENV_ACTION_REL
    step = get_step(rel, "Write CI env files")
    check("T-2 write-ci-env step found", step is not None)
    if step is None:
        return
    run = step.get("run") or ""
    env = step.get("env") or {}
    missing = [k for k, v in ENV_MAP.items() if env.get(k) != v]
    check("T-2 all 9 inputs mapped in env", not missing, str(missing))
    check("T-2 run block has no " + OPEN, OPEN not in run)
    first = run.lstrip().splitlines()[0].strip() if run.strip() else ""
    check("T-2 umask 077 is the first command", first == "umask 077", repr(first))
    check(
        "T-2 .env.ci keeps all 7 credential keys",
        all("'" + k + "=%s" in run for k in ENV_CI_KEYS),
    )
    check("T-2 no hardcoded superadmin placeholder", "super-unique-password" not in run)
    check("T-2 no hardcoded callback placeholder", "ci-callback-token" not in run)
    check("T-2 every env reference double-quoted", bare_refs(run) == [])


def check_task3():
    rel = CI_ENV_ACTION_REL
    step = get_step(rel, "Wait for infrastructure readiness")
    check("T-3 readiness step found", step is not None)
    if step is None:
        return
    run = step.get("run") or ""
    env = step.get("env") or {}
    check("T-3 CI_REDIS_PASSWORD env mapping", env.get("CI_REDIS_PASSWORD") == OPEN + " inputs.ci-redis-password }}")
    check("T-3 redis-cli uses quoted $CI_REDIS_PASSWORD", 'redis-cli -a "$CI_REDIS_PASSWORD"' in run)
    check("T-3 run block has no " + OPEN, OPEN not in run)


HOSTILE = {
    "XINJECT_XENDIT": 'x"; touch injected-marker; echo "',
    "XINJECT_POSTGRES": "p'w$d",
    "XINJECT_REDIS": "r3$()",
    "XINJECT_RABBITMQ": "r4`id`",
    "XINJECT_SUPERTOKEN": "S${T}ecret",
    "XINJECT_SUPERADMIN": 'sa"dmin "$(id)"',
    "XINJECT_XENDIT_TOKEN": "tok$};x",
    "XINJECT_JWT_HOOK": 'line1\nline2 "$(id)" $USER',
    "XINJECT_ORGANIZATION": 'org"; date > owned; #',
}

# Bind each hostile payload to the env var name the ci-env action's run
# blocks read. Values are injection-identifiers, never secrets themselves.
HOSTILE_ENV = {
    "XENDIT_SANDBOX_API_KEY": "XINJECT_XENDIT",
    "CI_POSTGRES_PASSWORD": "XINJECT_POSTGRES",
    "CI_REDIS_PASSWORD": "XINJECT_REDIS",
    "CI_RABBITMQ_PASSWORD": "XINJECT_RABBITMQ",
    "CI_SUPERTOKEN_API_KEY": "XINJECT_SUPERTOKEN",
    "CI_SUPERADMIN_API_KEY": "XINJECT_SUPERADMIN",
    "CI_XENDIT_CALLBACK_TOKEN": "XINJECT_XENDIT_TOKEN",
    "CI_JWT_HOOK_KEY": "XINJECT_JWT_HOOK",
    "ORGANIZATION": "XINJECT_ORGANIZATION",
}

EXPECTED_ENV_CI = (
    "XENDIT_SANDBOX_API_KEY=" + HOSTILE["XINJECT_XENDIT"]
    + "\nPOSTGRES_PASSWORD=" + HOSTILE["XINJECT_POSTGRES"]
    + "\nREDIS_PASSWORD=" + HOSTILE["XINJECT_REDIS"]
    + "\nRABBITMQ_DEFAULT_PASS=" + HOSTILE["XINJECT_RABBITMQ"]
    + "\nSUPERTOKEN_API_KEY=" + HOSTILE["XINJECT_SUPERTOKEN"]
    + "\nSUPERADMIN_API_KEY=" + HOSTILE["XINJECT_SUPERADMIN"]
    + "\nXENDIT_CALLBACK_TOKEN=" + HOSTILE["XINJECT_XENDIT_TOKEN"]
    + "\n"
)

EXPECTED_DOCS_ENV = (
    "APP_BASE_URL=http://localhost:3200"
    + "\nBLAZE_BASE_URL=http://localhost:8080"
    + "\nSUPERADMIN_API_KEY=" + HOSTILE["XINJECT_SUPERADMIN"]
    + "\nORGANIZATION=" + HOSTILE["XINJECT_ORGANIZATION"]
    + "\nMAILINATOR_BASE_URL=http://localhost:8081/api/v2"
    + "\nXENDIT_CALLBACK_TOKEN=" + HOSTILE["XINJECT_XENDIT_TOKEN"]
    + "\n"
)

EXPECTED_GITHUB_ENV = (
    "PREEXISTING=1"
    + "\nJWT_HOOK_KEY<<EOF"
    + "\n" + HOSTILE["XINJECT_JWT_HOOK"]
    + "\nEOF"
    + "\n"
)


def check_behavior():
    step = get_step(CI_ENV_ACTION_REL, "Write CI env files")
    script = step["run"] if step else ""
    exe = "bash" if Path("/bin/bash").exists() else "sh"
    with tempfile.TemporaryDirectory() as d:
        d = Path(d)
        (d / "docs/api").mkdir(parents=True)
        (d / "GITHUB_ENV").write_text("PREEXISTING=1\n", encoding="utf-8")
        env = dict(os.environ)
        env.update({name: HOSTILE[payload] for name, payload in HOSTILE_ENV.items()})
        env["GITHUB_ENV"] = str(d / "GITHUB_ENV")
        proc = subprocess.run(  # nosec B603 - script from this repo's ci-env action, isolated temp dir, no shell
            [exe, "-c", script], cwd=d, env=env,
            capture_output=True, text=True, timeout=30,
        )
        tail = proc.stderr.strip().splitlines()
        check("behavior script exits 0", proc.returncode == 0, tail[-1] if tail else "")
        envci = d / ".env.ci"
        docsenv = d / "docs/api/.env"
        check("behavior .env.ci byte-identical", envci.read_text(encoding="utf-8") == EXPECTED_ENV_CI)
        check("behavior docs/api/.env byte-identical", docsenv.read_text(encoding="utf-8") == EXPECTED_DOCS_ENV)
        check(
            "behavior GITHUB_ENV append byte-identical",
            (d / "GITHUB_ENV").read_text(encoding="utf-8") == EXPECTED_GITHUB_ENV,
        )
        check("behavior no injection marker file", not (d / "injected-marker").exists())
        check(
            "behavior umask 077 -> files 0600",
            stat.S_IMODE(envci.stat().st_mode) == 0o600
            and stat.S_IMODE(docsenv.stat().st_mode) == 0o600,
        )


def check_task4():
    """Task 4: Trivy image scan must ignore unfixed CVEs."""
    step = get_step(".github/workflows/pr.yml", "Trivy image scan (blocking + SARIF)")
    check("T-4 Trivy image scan step found", step is not None)
    if step is None:
        return
    with_ = step.get("with") or {}
    check("T-4 ignore-unfixed is true", str(with_.get("ignore-unfixed")).lower() == "true")


def check_task5():
    """Task 5: CodeQL actions must be v4, not v3."""
    text = (ROOT / ".github/workflows/pr.yml").read_text(encoding="utf-8")
    v3_refs = re.findall(r"codeql-action/\w+@v3", text)
    check("T-5 no codeql-action v3 refs in pr.yml", len(v3_refs) == 0, str(v3_refs))


def check_task6():
    """Task 6: pr.yml on: block must include both push and pull_request triggers."""
    doc = load_yaml(".github/workflows/pr.yml")
    # PyYAML parses YAML 'on:' as boolean True, not string 'on'
    on_block = doc.get("on") or doc.get(True) or {}
    check("T-6 push trigger present", "push" in on_block)
    check("T-6 pull_request trigger present", "pull_request" in on_block)
    if "push" in on_block:
        push_branches = on_block["push"].get("branches") or []
        check("T-6 push branches include develop and main",
              "develop" in push_branches and "main" in push_branches)


def check_task7():
    """Task 7: Dockerfile-vendor Go version must be 1.26 to match go.mod."""
    line1 = (ROOT / "Dockerfile-vendor").read_text(encoding="utf-8").splitlines()[0]
    check("T-7 Dockerfile-vendor uses golang:1.26", "golang:1.26" in line1, line1)


def main():
    print("YAML validity")
    check_yaml_valid()
    print("\nRepo-wide run-block rule")
    check_repo_wide_no_interpolation()
    print("\nTask-level definition-of-done")
    check_task1()
    check_task2()
    check_task3()
    check_task4()
    check_task5()
    check_task6()
    check_task7()
    print("\nBehavior check (hostile values)")
    check_behavior()
    print()
    if FAILED:
        print("FAILED: " + ", ".join(FAILED))
        sys.exit(1)
    print("ALL CHECKS PASSED")
    sys.exit(0)


if __name__ == "__main__":
    main()
