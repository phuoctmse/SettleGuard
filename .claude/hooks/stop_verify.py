"""Stop hook: lint and unit-test the code touched in this turn before the
agent is allowed to report done.

    stop_verify.py mark   # UserPromptSubmit: record the turn's start (time + HEAD)
    stop_verify.py check  # Stop: verify everything touched since the mark

"Touched" = files committed since the marked HEAD, plus uncommitted/untracked
files modified after the mark (mtime), so edits made through Bash count too.

A failure exits 2, which sends the output back to the agent to fix. It blocks
once per stop: when stop_hook_active is set (the agent is already continuing
because of this hook) a second failure is reported to the user instead, so
the hook can never loop forever.
"""

import json
import os
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path

GO_SERVICES = {"ledger-service", "accounts-service", "settlement-engine", "gateway"}
JS_EXT = (".ts", ".tsx", ".js", ".jsx")
STATE_DIR = Path(tempfile.gettempdir()) / "claude-settleguard-hooks"
CMD_TIMEOUT = 600
TAIL_LINES = 40


def git(root, *args):
    r = subprocess.run(["git", "-C", str(root), *args], capture_output=True,
                       text=True, encoding="utf-8", errors="replace")
    return r.stdout.splitlines() if r.returncode == 0 else []


def state_file(session_id):
    return STATE_DIR / f"{session_id or 'default'}.json"


def mark(root, session_id):
    STATE_DIR.mkdir(parents=True, exist_ok=True)
    head = git(root, "rev-parse", "HEAD")
    state_file(session_id).write_text(
        json.dumps({"start": time.time(), "head": head[0] if head else None}))


def touched_files(root, session_id):
    try:
        state = json.loads(state_file(session_id).read_text())
    except (OSError, ValueError):
        state = None

    working = set(git(root, "diff", "--name-only", "HEAD"))
    working |= set(git(root, "ls-files", "--others", "--exclude-standard"))
    if state is None:
        return {f for f in working if (root / f).exists()}

    touched = set()
    head = git(root, "rev-parse", "HEAD")
    if state.get("head") and head and state["head"] != head[0]:
        touched |= set(git(root, "diff", "--name-only", state["head"], "HEAD"))
    for f in working:
        p = root / f
        if p.exists() and p.stat().st_mtime >= state["start"] - 1:
            touched.add(f)
    return {f for f in touched if (root / f).exists()}


def venv_python(directory):
    for rel in (".venv/Scripts/python.exe", ".venv/bin/python"):
        if (directory / rel).exists():
            return str(directory / rel)
    return sys.executable


def plan(files, root):
    """Group touched files into (label, cwd, argv) checks."""
    go_pkgs, go_mods = {}, set()
    mobile_js, mobile_any = [], []
    notif_py, tests_py = [], []

    for f in sorted(files):
        parts = f.split("/")
        if any(p in (".venv", "node_modules", ".expo", "__pycache__") for p in parts):
            continue
        if parts[0] == "services" and len(parts) > 2 and parts[1] in GO_SERVICES:
            svc, rest = parts[1], "/".join(parts[2:])
            if f.endswith(".go"):
                go_pkgs.setdefault(svc, set()).add("./" + (os.path.dirname(rest) or "."))
            elif f.endswith(".sql") and "/migrations/" in f:
                # Migrations are embedded in the db package and exercised by its tests.
                go_pkgs.setdefault(svc, set()).add("./internal/db")
            elif rest in ("go.mod", "go.sum"):
                go_mods.add(svc)
        elif parts[:2] == ["services", "notification-service"] and f.endswith(".py"):
            notif_py.append("/".join(parts[2:]))
        elif parts[0] == "mobile-app" and len(parts) > 1:
            rel = "/".join(parts[1:])
            mobile_any.append(rel)
            if rel.endswith(JS_EXT):
                mobile_js.append(rel)
        elif parts[0] == "tests" and f.endswith(".py"):
            tests_py.append("/".join(parts[1:]))

    checks = []
    docker = docker_up()
    notes = []

    for svc in sorted(set(go_pkgs) | go_mods):
        cwd = root / "services" / svc
        pkgs = sorted(go_pkgs.get(svc, ()))
        checks.append((f"{svc}: go build", cwd, ["go", "build", "./..."]))
        checks.append((f"{svc}: go vet", cwd, ["go", "vet", "./..."]))
        if pkgs and shutil.which("golangci-lint"):
            checks.append((f"{svc}: golangci-lint {' '.join(pkgs)}", cwd,
                           ["golangci-lint", "run", "--config", str(root / ".golangci.yml"), *pkgs]))
        elif pkgs:
            notes.append("golangci-lint not installed (CI uses v1.64.8): lint skipped")
        if pkgs and docker:
            checks.append((f"{svc}: go test {' '.join(pkgs)}", cwd,
                           ["go", "test", "-count=1", "-p", "1", *pkgs]))
        elif pkgs:
            notes.append(f"Docker not running: {svc} tests (testcontainers) skipped")

    if notif_py:
        cwd = root / "services" / "notification-service"
        py = venv_python(cwd)
        checks.append(("notification-service: py_compile", cwd, [py, "-m", "py_compile", *notif_py]))
        tests = set()
        for rel in notif_py:
            p = cwd / rel
            if p.name.endswith("_test.py"):
                tests.add(rel)
            else:
                sibling = p.with_name(p.stem + "_test.py")
                if sibling.exists():
                    tests.add(sibling.relative_to(cwd).as_posix())
                else:
                    tests.update(t.relative_to(cwd).as_posix() for t in p.parent.glob("*_test.py"))
        if tests and docker:
            checks.append((f"notification-service: pytest {' '.join(sorted(tests))}", cwd,
                           [py, "-m", "pytest", "-q", *sorted(tests)]))
        elif tests:
            notes.append("Docker not running: notification-service tests skipped")

    if mobile_any:
        cwd = root / "mobile-app"
        npx = shutil.which("npx") or "npx"
        if mobile_js:
            checks.append(("mobile-app: eslint", cwd, [npx, "eslint", *mobile_js]))
        checks.append(("mobile-app: prettier --check", cwd,
                       [npx, "prettier", "--check", "--ignore-unknown", *mobile_any]))
        if mobile_js:
            checks.append(("mobile-app: jest --findRelatedTests", cwd,
                           [npx, "jest", "--ci", "--passWithNoTests", "--findRelatedTests", *mobile_js]))

    if tests_py:
        # tests/ runs against a live stack (and local venvs often lack playwright/
        # locust), so the Stop gate only checks syntax; run the suite per tests/README.md.
        cwd = root / "tests"
        checks.append(("tests/: py_compile", cwd, [venv_python(cwd), "-m", "py_compile", *tests_py]))
        notes.append("tests/ suites need a live stack: only syntax-checked")

    return checks, sorted(set(notes))


def docker_up():
    if not shutil.which("docker"):
        return False
    try:
        return subprocess.run(["docker", "info"], capture_output=True, timeout=20).returncode == 0
    except subprocess.TimeoutExpired:
        return False


def run(label, cwd, argv):
    try:
        r = subprocess.run(argv, cwd=cwd, capture_output=True, text=True,
                           encoding="utf-8", errors="replace", timeout=CMD_TIMEOUT)
    except subprocess.TimeoutExpired:
        return f"{label}: timed out after {CMD_TIMEOUT}s"
    except OSError as e:
        return f"{label}: could not start ({e})"
    if r.returncode == 0:
        return None
    out = (r.stdout + r.stderr).strip().splitlines()
    return f"{label} (exit {r.returncode}) in {cwd}:\n" + "\n".join(out[-TAIL_LINES:])


def check(root, data):
    session_id = data.get("session_id")
    checks, notes = plan(touched_files(root, session_id), root)
    if not checks:
        return 0

    failures = [f for f in (run(*c) for c in checks) if f]
    note_text = ("\nNotes: " + "; ".join(notes)) if notes else ""

    if not failures:
        mark(root, session_id)  # later stops in this turn only re-check new edits
        print(json.dumps({"systemMessage": f"stop-verify: {len(checks)} checks passed"
                                           + (f" ({'; '.join(notes)})" if notes else "")}))
        return 0

    report = "\n\n".join(failures)
    if data.get("stop_hook_active"):
        print(json.dumps({"systemMessage": "stop-verify: checks STILL failing after one "
                                           "fix attempt; not blocking again.\n\n" + report[:4000]}))
        return 0
    sys.stderr.write(
        "stop-verify: lint/unit tests failed on the code touched this turn. Fix the "
        "cause (not the test) and re-run the failing command before finishing.\n\n"
        + report + note_text + "\n")
    return 2


def main():
    mode = sys.argv[1] if len(sys.argv) > 1 else "check"
    try:
        data = json.load(sys.stdin)
    except ValueError:
        data = {}
    root = Path(os.environ.get("CLAUDE_PROJECT_DIR") or data.get("cwd") or os.getcwd())
    if mode == "mark":
        mark(root, data.get("session_id"))
        return 0
    return check(root, data)


if __name__ == "__main__":
    sys.exit(main())
