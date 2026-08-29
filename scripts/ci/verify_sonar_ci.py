#!/usr/bin/env python3
"""Static DoD checks for the SonarCloud CI-hardening tasks (T-2..T-9).

Each `check_*` function maps to one task's definition of done. Run from the
repository root:  python3 scripts/ci/verify_sonar_ci.py
Exit code is non-zero if any check fails, so CI tooling can gate on it too.
"""

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
FAILED = []


def check(name, condition, detail=""):
    if condition:
        print(f"PASS  {name}")
    else:
        FAILED.append(name)
        print(f"FAIL  {name}  {detail}")


def read(rel):
    return (ROOT / rel).read_text(encoding="utf-8")


compose = read("docker-compose.ci.yml")
action = read(".github/actions/ci-env/action.yml")
pr = read(".github/workflows/pr.yml")
weekly = read(".github/workflows/security-weekly.yml")
stub = read("scripts/ci/magiclink-stub.mjs")
docs_api = read("docs/api/AGENTS.md")
readme = read(".github/workflows/README.md")


def indented_run_block_lines(text):
    """Yield (step, snippet) for every `run: |` step whose body contains `${{ secrets.`."""
    lines = text.splitlines()
    out = []
    for i, line in enumerate(lines):
        m = re.match(r"^(\s*)run:\s*\|", line)
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
        if any("${{ secrets." in b for b in body):
            out.append((line.strip(), "\n".join(body)))
    return out


# ---- T-2: magiclink-stub.mjs RegExp.exec (findings 12-13) ----
check("T2 stub uses no path.match()", "path.match(" not in stub)
check("T2 stub uses .exec() for routes", ".exec(path)" in stub)

# ---- T-3: docker-compose.ci.yml secret interpolation (findings 3-4) ----
CRED_LITERALS = [
    "supertokens@postgres",
    "BEGIN PRIVATE KEY",
    "super-unique-password",
    "ci-callback-token",
    "redispass",
    "POSTGRES_PASSWORD: supertokens",
    "RABBITMQ_DEFAULT_PASS: password",
    'API_KEYS: "password',
    'SUPERTOKEN_API_KEY: "password',
    "APP_XENDIT_WEBHOOK_TOKEN:",
]
for literal in CRED_LITERALS:
    check(f"T3 compose has no {literal!r}", literal not in compose)
for var in [
    "POSTGRES_PASSWORD",
    "REDIS_PASSWORD",
    "RABBITMQ_DEFAULT_PASS",
    "SUPERTOKEN_API_KEY",
    "SUPERADMIN_API_KEY",
    "XENDIT_CALLBACK_TOKEN",
    "JWT_HOOK_KEY",
]:
    check(f"T3 compose interpolates ${{{var}}}", f"${{{var}:" in compose or f"${{{var}}}" in compose)
# no hardcoded credential-value environment lines remain
CRED_KEYS = [
    "POSTGRES_PASSWORD:",
    "REDIS_PASSWORD:",
    "RABBITMQ_DEFAULT_PASS:",
    "RABBITMQ_PASSWORD:",
    "SUPERTOKEN_API_KEY:",
    "SUPERADMIN_API_KEY:",
    "API_KEYS:",
    "JWT_HOOK_KEY:",
    "APP_XENDIT_WEBHOOK_TOKEN:",
]
hardcoded = []
for line in compose.splitlines():
    for key in CRED_KEYS:
        if line.lstrip().startswith(key):
            value = line.split(":", 1)[1].strip()
            if "$" not in value:
                hardcoded.append(line.strip())
check("T3 compose has no hardcoded credential values", not hardcoded, str(hardcoded))
uri_line = next(l for l in compose.splitlines() if "POSTGRESQL_CONNECTION_URI" in l)
check(
    "T3 supertokens URI embeds ${POSTGRES_PASSWORD}",
    "${POSTGRES_PASSWORD}" in uri_line,
    uri_line.strip(),
)
check(
    "T3 compose header documents .env.ci injection",
    "env.ci" in compose.splitlines()[0:40] and "secrets" in " ".join(compose.splitlines()[0:40]),
)

# ---- T-4: ci-env action inputs / writer / readiness / npm (finding 11) ----
inputs = re.findall(r"^  [a-z][a-z0-9-]*:$", action.split("inputs:")[1].split("runs:")[0], re.M)
check("T4 action has 9 inputs", len(inputs) == 9, str(inputs))
for key in [
    "ci-postgres-password",
    "ci-redis-password",
    "ci-rabbitmq-password",
    "ci-supertoken-api-key",
    "ci-superadmin-api-key",
    "ci-xendit-callback-token",
    "ci-jwt-hook-key",
]:
    check(f"T4 action input {key}", key in action)
for key in [
    "XENDIT_SANDBOX_API_KEY=%s",
    "POSTGRES_PASSWORD=%s",
    "REDIS_PASSWORD=%s",
    "RABBITMQ_DEFAULT_PASS=%s",
    "SUPERTOKEN_API_KEY=%s",
    "SUPERADMIN_API_KEY=%s",
    "XENDIT_CALLBACK_TOKEN=%s",
]:
    check(f"T4 .env.ci writes {key.split('=')[0]}", key in action)
check("T4 JWT_HOOK_KEY exported via GITHUB_ENV heredoc", "JWT_HOOK_KEY<<EOF" in action)
check(
    "T4 readiness uses input redis password",
    'redis-cli -a "${{ inputs.ci-redis-password }}"' in action,
)
check("T4 npm install has --ignore-scripts", "--ignore-scripts" in action)
for literal in ["super-unique-password", "ci-callback-token", "redispass"]:
    check(f"T4 action has no {literal!r}", literal not in action)

# ---- T-5: pr.yml permissions + no secrets in run blocks (findings 5,6,10) ----
check("T5 pr.yml no workflow-level security-events write", not re.search(r"^  security-events: write(?:\s+#.*)?$", pr, re.M))
se_write_jobs = re.findall(r"^      security-events: write$", pr, re.M)
check("T5 pr.yml job-level security-events write x3", len(se_write_jobs) == 3, str(se_write_jobs))
check("T5 pr.yml no secrets expanded in run blocks", not indented_run_block_lines(pr), str(indented_run_block_lines(pr)))

# ---- T-6: pr.yml integration environment + fork-PR skip + secret plumbing ----
check('T6 pr.yml integration references environment', 'environment: "Pull Request Screening"' in pr)
check(
    "T6 pr.yml integration forks skipped",
    "github.event.pull_request.head.repo.full_name == github.repository" in pr,
)
for key in [
    "ci-postgres-password",
    "ci-redis-password",
    "ci-rabbitmq-password",
    "ci-supertoken-api-key",
    "ci-superadmin-api-key",
    "ci-xendit-callback-token",
    "ci-jwt-hook-key",
]:
    check(f"T6 pr.yml passes {key}", f"{key}: ${{{{ secrets.{key.upper().replace('-', '_')} }}}}" in pr)

# ---- T-7: security-weekly.yml permissions + env wiring ----
check("T7 weekly no workflow-level security-events write", not re.search(r"^  security-events: write(?:\s+#.*)?$", weekly, re.M))
weekly_se = re.findall(r"^      security-events: write$", weekly, re.M)
check("T7 weekly job-level security-events write x1", len(weekly_se) == 1, str(weekly_se))
check(
    "T7 weekly zap-and-regression references environment",
    'environment: "Pull Request Screening"' in weekly,
)
for key in [
    "ci-postgres-password",
    "ci-redis-password",
    "ci-rabbitmq-password",
    "ci-supertoken-api-key",
    "ci-superadmin-api-key",
    "ci-xendit-callback-token",
    "ci-jwt-hook-key",
]:
    check(f"T7 weekly passes {key}", f"{key}: ${{{{ secrets.{key.upper().replace('-', '_')} }}}}" in weekly)

# ---- T-8: Go tool lockfile enforcement (findings 7-9) ----
check("T8 no go install gofumpt", "go install mvdan.cc/gofumpt@" not in pr)
check("T8 no go install govulncheck", "go install golang.org/x/vuln" not in pr + weekly)
check("T8 uses go tool gofumpt", "go tool gofumpt" in pr)
check("T8 uses go tool govulncheck", "go tool govulncheck" in pr and "go tool govulncheck" in weekly)
check("T8 tools/tools.go exists", (ROOT / "tools" / "tools.go").exists())

# ---- T-9: docs ----
check("T9 docs/api/AGENTS.md no stale credential literal", "super-unique-password" not in docs_api)
check(
    "T9 README documents fork-PR policy",
    any(w in readme.lower() for w in ["fork", "pull request"])
    and "integration" in readme
    and any(w in readme.lower() for w in ["skip", "skipped"]),
)

print()
if FAILED:
    print(f"{len(FAILED)} check(s) failing: {', '.join(FAILED)}")
    sys.exit(1)
print("All static DoD checks passed.")
