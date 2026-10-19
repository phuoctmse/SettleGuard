"""PreToolUse guard: blocks destructive git, raw schema changes, and edits
to migrations that are already on origin/main.

settings.json deny rules match by prefix, so `git -C . push -f` or
`rtk git reset --hard` slip past them; this hook matches by regex instead.
A denied call gets a reason back so the agent can take the safe route.
"""

import json
import os
import re
import subprocess
import sys

# One shell segment: stop at ; & | or newline so `git status && rm x` does not
# smear one command's flags onto the next.
SEG = r"[^;&|\n]*"

GIT_RULES = [
    (
        rf"\bgit\b{SEG}\bpush\b{SEG}(--force\b|--force-with-lease|\s-[a-zA-Z]*f[a-zA-Z]*\b|\s\+\S)",
        "force push rewrites shared history. Push normally; if the remote "
        "has diverged, fetch and rebase or merge first.",
    ),
    (
        rf"\bgit\b{SEG}\breset\b{SEG}--hard\b",
        "git reset --hard discards work. Use `git stash`, `git restore <file>`, "
        "or `git reset --soft` / a new revert commit.",
    ),
    (
        rf"\bgit\b{SEG}\b(checkout|switch)\s+(-\S+\s+)*(main|master)\b(?![-/\w.])(?!\s+--\s)",
        "work happens on step/* or service/* branches; main only changes via "
        "squash-merged PRs. Branch from it with "
        "`git fetch && git switch -c <branch> origin/main`.",
    ),
    (
        rf"\bgit\b{SEG}\bbranch\b{SEG}\s(-d|-D|--delete)\b",
        "branches are kept after merge in this repo, never deleted.",
    ),
    (
        rf"\bgit\b{SEG}\bpush\b{SEG}(--delete\b|\s:\S)",
        "remote branches are kept after merge in this repo, never deleted.",
    ),
    (
        rf"\bgit\b{SEG}\bclean\b{SEG}\s-[a-zA-Z]*f",
        "git clean -f deletes untracked files (including local .env files). "
        "Remove specific files by name instead.",
    ),
]

DB_CLIENT = re.compile(r"\b(psql|pgcli)\b", re.IGNORECASE)
SCHEMA_SQL = re.compile(
    r"\b(DROP\s+(TABLE|SCHEMA|DATABASE|INDEX|TYPE|VIEW|COLUMN|CONSTRAINT)"
    r"|TRUNCATE\b|ALTER\s+(TABLE|TYPE|SCHEMA|DATABASE|INDEX)"
    r"|CREATE\s+(TABLE|INDEX|TYPE|SCHEMA|UNIQUE\s+INDEX)"
    r"|DELETE\s+FROM|UPDATE\s+\w+\s+SET)",
    re.IGNORECASE,
)
SCHEMA_REASON = (
    "schema and data changes go through golang-migrate: add the next numbered "
    "NNNNNN_<name>.up.sql + .down.sql pair under services/<svc>/internal/db/"
    "migrations/ and let the service apply it on startup. Read-only SELECTs "
    "via psql are fine."
)

MIGRATION_PATH = re.compile(r"[\\/]migrations[\\/][^\\/]+\.sql$")

# A commit message or PR body that *mentions* `git push -f` is prose, not a
# command. Blank message text only for message-bearing commands, so that
# `bash -c "git push -f"` is still caught.
MESSAGE_CMD = re.compile(r"\b(git\s+(commit|tag)|gh\s+(pr|issue|release))\b")
MESSAGE_ARG = re.compile(
    r"""((?:-m|--message|-b|--body|-t|--title|--notes)[=\s]+)("(?:[^"\\]|\\.)*"|'[^']*')""",
    re.DOTALL,
)
HEREDOC = re.compile(r"<<-?\s*(['\"]?)(\w+)\1.*?\n\2\b", re.DOTALL)


def strip_messages(cmd):
    if not MESSAGE_CMD.search(cmd):
        return cmd
    cmd = HEREDOC.sub("<<heredoc", cmd)
    return MESSAGE_ARG.sub(r'\1""', cmd)


def deny(reason):
    print(
        json.dumps(
            {
                "hookSpecificOutput": {
                    "hookEventName": "PreToolUse",
                    "permissionDecision": "deny",
                    "permissionDecisionReason": "Blocked by .claude/hooks/guard.py: " + reason,
                }
            }
        )
    )
    sys.exit(0)


def check_command(cmd):
    for pattern, reason in GIT_RULES:
        if re.search(pattern, cmd):
            deny(reason)
    if re.search(r"\bdropdb\b", cmd):
        deny(SCHEMA_REASON)
    if DB_CLIENT.search(cmd):
        if SCHEMA_SQL.search(cmd):
            deny(SCHEMA_REASON)
        if re.search(r"(\s-f\s*|--file[=\s])\S*migrations", cmd):
            deny("running a migration file by hand skips golang-migrate's "
                 "schema_migrations bookkeeping. " + SCHEMA_REASON)
    if re.search(rf"\bmigrate\b{SEG}\s(force|drop|down|goto)\b", cmd):
        deny("migrate force/drop/down/goto rewrites the database's migration "
             "state. Ask the user to run it if it is truly needed.")


def check_edit(path, project_dir):
    if not MIGRATION_PATH.search(path):
        return
    try:
        rel = os.path.relpath(os.path.abspath(path), project_dir).replace("\\", "/")
    except ValueError:
        return
    merged = subprocess.run(
        ["git", "-C", project_dir, "cat-file", "-e", f"origin/main:{rel}"],
        capture_output=True,
    )
    if merged.returncode == 0:
        deny(f"{rel} is on origin/main, so it has already run against real "
             "databases. Change the schema with a new numbered migration "
             "instead of editing this one.")


def main():
    try:
        data = json.load(sys.stdin)
    except ValueError:
        return
    tool = data.get("tool_name", "")
    tool_input = data.get("tool_input") or {}
    project_dir = os.environ.get("CLAUDE_PROJECT_DIR") or data.get("cwd") or os.getcwd()

    if tool in ("Bash", "PowerShell"):
        check_command(strip_messages(tool_input.get("command", "")))
    elif tool in ("Edit", "Write", "MultiEdit", "NotebookEdit"):
        path = tool_input.get("file_path") or tool_input.get("notebook_path") or ""
        if path:
            check_edit(path, project_dir)


if __name__ == "__main__":
    main()
