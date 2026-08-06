#!/usr/bin/env python3
"""Fail-open Codex hook that detects Git HEAD transitions without reading chat data."""

from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
from typing import Any


def run(command: list[str], cwd: str) -> str | None:
    try:
        completed = subprocess.run(
            command,
            cwd=cwd,
            check=True,
            capture_output=True,
            text=True,
            timeout=2,
        )
    except (OSError, subprocess.SubprocessError):
        return None
    return completed.stdout.strip()


def git_facts(cwd: str) -> dict[str, str] | None:
    root = run(["git", "rev-parse", "--show-toplevel"], cwd)
    head = run(["git", "rev-parse", "HEAD"], cwd)
    if not root or not head:
        return None
    branch = run(["git", "branch", "--show-current"], root) or ""
    common_dir = run(["git", "rev-parse", "--path-format=absolute", "--git-common-dir"], root) or ""
    project = Path(common_dir).parent.name if Path(common_dir).name == ".git" else Path(root).name
    return {"repo_path": root, "head_sha": head.lower(), "branch": branch, "project": project}


def state_path(session_id: str) -> Path | None:
    data_root = os.environ.get("PLUGIN_DATA", "").strip()
    if not data_root:
        return None
    root = Path(data_root) / "checkpoint-state"
    root.mkdir(mode=0o700, parents=True, exist_ok=True)
    digest = hashlib.sha256(session_id.encode("utf-8")).hexdigest()
    return root / f"{digest}.json"


def read_state(path: Path | None) -> dict[str, Any]:
    if path is None:
        return {}
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return {}
    return value if isinstance(value, dict) else {}


def write_state(path: Path | None, state: dict[str, Any]) -> None:
    if path is None:
        return
    handle, temporary = tempfile.mkstemp(prefix="checkpoint-", suffix=".json", dir=path.parent)
    try:
        with os.fdopen(handle, "w", encoding="utf-8") as stream:
            json.dump(state, stream, separators=(",", ":"))
        os.chmod(temporary, 0o600)
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def feature_context(harbor: str, session_id: str, facts: dict[str, str]) -> dict[str, Any] | None:
    output = run(
        [
            harbor,
            "--output",
            "json",
            "feature",
            "context",
            "--session",
            session_id,
            "--repo",
            facts["repo_path"],
            "--branch",
            facts["branch"],
            "--project",
            facts["project"],
        ],
        facts["repo_path"],
    )
    if not output:
        return None
    try:
        value = json.loads(output)
    except ValueError:
        return None
    return value if isinstance(value, dict) else None


def emit_context(event: str, message: str) -> None:
    print(
        json.dumps(
            {
                "hookSpecificOutput": {
                    "hookEventName": event,
                    "additionalContext": message,
                }
            }
        )
    )


def main() -> None:
    try:
        payload = json.load(sys.stdin)
        if not isinstance(payload, dict):
            return
        event = str(payload.get("hook_event_name", ""))
        session_id = str(payload.get("session_id", "")).strip()
        cwd = str(payload.get("cwd", "")).strip()
        if event not in {"SessionStart", "PostToolUse"} or not session_id or not cwd:
            return
        facts = git_facts(cwd)
        harbor = shutil.which("harbor")
        if facts is None or harbor is None:
            return

        path = state_path(session_id)
        state = read_state(path)
        if state.get("repo_path") != facts["repo_path"]:
            state = {"repo_path": facts["repo_path"], "last_head": facts["head_sha"]}

        context = feature_context(harbor, session_id, facts)
        detail = context.get("detail") if context else None
        feature = detail.get("feature") if isinstance(detail, dict) else None

        if event == "SessionStart":
            state.setdefault("last_head", facts["head_sha"])
            write_state(path, state)
            if isinstance(feature, dict) and feature.get("id"):
                emit_context(
                    event,
                    "Harbor loaded Feature "
                    f"{feature['id']} ({feature.get('title', 'untitled')}) via {context.get('match', 'context')}. "
                    "Keep Git as the durable evidence source. If Harbor later reports a HEAD transition, "
                    "use $summarize-harbor-checkpoint before the final response.",
                )
            return

        previous = str(state.get("last_head", "")).lower()
        current = facts["head_sha"]
        if not previous or previous == current:
            state["last_head"] = current
            write_state(path, state)
            return
        state["last_head"] = current
        write_state(path, state)
        if isinstance(feature, dict) and feature.get("id"):
            emit_context(
                event,
                "Harbor detected a stable Git checkpoint for Feature "
                f"{feature['id']}: {previous}..{current}. "
                "Use $summarize-harbor-checkpoint before the final response. Summarize durable outcomes, "
                "decisions, actual verification, and remaining work; do not store transcript content.",
            )
    except Exception:
        return


if __name__ == "__main__":
    main()
