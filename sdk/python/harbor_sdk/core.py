"""Core SDK functions for Harbor Python connectors."""

import argparse
import json
import os
import subprocess
import sys
import uuid
from datetime import datetime, timezone
from typing import Any, Optional


# ── Protocol types ──────────────────────────────────────────────

class HarborPageInfo:
    """Cursor-based pagination info."""
    cursor: Optional[str]
    has_more: bool
    total: Optional[int]


class HarborMemoryRef:
    """Lightweight reference to a related memory entry in meta.recalls."""
    id: str
    resource: str
    age: str      # e.g. "2 days ago", "5 hours ago"
    summary: str
    fresh: bool   # True if within the freshness TTL window


class HarborContextRef:
    """
    Pinned connector-level note written via harbor_remember.
    Appears as meta.context on every future access to the connector.
    """
    summary: str
    age: str


class HarborMeta:
    """Provenance and versioning information returned with every response."""
    source: str
    connector_version: str
    schema: str
    tool_schema_version: Optional[str]
    fetched_at: str
    request_id: str
    pagination: Optional[HarborPageInfo]
    # Memory fields (present when Harbor memory pipeline is active)
    memory_id: Optional[str]
    from_memory: Optional[bool]
    context: Optional[HarborContextRef]      # pinned note from harbor_remember
    recalls: Optional[list]                  # list[HarborMemoryRef]
    memory_hint: Optional[str]               # shown when no context exists yet


class HarborContextOverview:
    """Structural summary of the data payload."""
    total_items: int
    fields: list[str]


class HarborError:
    """Structured error detail."""
    code: str
    message: str
    detail: Optional[str]


class HarborResponse:
    """Canonical response envelope returned by every connector."""
    data: list
    meta: HarborMeta
    raw: Optional[Any]
    errors: list       # list[HarborError]
    overview: Optional[HarborContextOverview]
    summary: Optional[str]


# ── Argument parsing ────────────────────────────────────────────

def parse_args() -> dict:
    """Parse standard Harbor connector arguments."""
    parser = argparse.ArgumentParser(description="Harbor connector")
    parser.add_argument("--resource", required=True, help="Resource name")
    parser.add_argument("--params", default="{}", help="JSON params")
    parser.add_argument("--raw", action="store_true", help="Include raw response")
    parser.add_argument("--describe", action="store_true", help="Emit tool schemas")
    args = parser.parse_args()

    return {
        "resource": args.resource,
        "params": json.loads(args.params),
        "auth": os.environ.get("HARBOR_AUTH", ""),
        "raw": args.raw,
        "describe": args.describe,
    }


# ── Output helper ───────────────────────────────────────────────

def output(response: dict[str, Any]) -> None:
    """Write response JSON to stdout."""
    sys.stdout.write(json.dumps(response) + "\n")
    sys.stdout.flush()


# ── Convenience builders ────────────────────────────────────────

def build_meta(source: str, connector_version: str, schema: str) -> dict:
    """Build a standard meta object."""
    return {
        "source": source,
        "connector_version": connector_version,
        "schema": schema,
        "tool_schema_version": "harbor.tools.v1",
        "fetched_at": datetime.now(timezone.utc).isoformat(),
        "request_id": str(uuid.uuid4()),
    }


def error_response(source: str, code: str, message: str) -> dict:
    """Build an error response."""
    return {
        "data": [],
        "meta": build_meta(source, "0.0.0", ""),
        "raw": None,
        "errors": [{"code": code, "message": message}],
    }


# ── CLI adapter ─────────────────────────────────────────────────

def exec_cli(
    command: str,
    args: list[str],
    *,
    cwd: Optional[str] = None,
    env: Optional[dict[str, str]] = None,
    timeout: int = 30,
    parse_json: bool = True,
) -> Any:
    """
    Execute an external CLI tool and capture its output.
    Uses subprocess without shell=True to avoid command injection.

    Example — wrapping a Notion CLI::

        raw = exec_cli("notion", ["search", "--query", params["query"], "--format", "json"])
    """
    merged_env = None
    if env is not None:
        merged_env = {**os.environ, **env}

    result = subprocess.run(
        [command, *args],
        capture_output=True,
        text=True,
        cwd=cwd,
        env=merged_env,
        timeout=timeout,
    )
    if result.returncode != 0:
        msg = result.stderr.strip() or f"exit code {result.returncode}"
        raise RuntimeError(f"{command} failed: {msg}")

    text = result.stdout.strip()
    if parse_json:
        try:
            return json.loads(text)
        except json.JSONDecodeError:
            return text
    return text


# ── Describe helper ─────────────────────────────────────────────

def handle_describe(schemas: list[dict]) -> bool:
    """If --describe was passed, emit tool schemas and return True."""
    if "--describe" in sys.argv:
        sys.stdout.write(json.dumps(schemas) + "\n")
        sys.stdout.flush()
        return True
    return False
