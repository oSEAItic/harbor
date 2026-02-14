"""Core SDK functions for Harbor Python connectors."""

import argparse
import json
import os
import sys
import uuid
from datetime import datetime, timezone
from typing import Any


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


def output(response: dict[str, Any]) -> None:
    """Write response JSON to stdout."""
    sys.stdout.write(json.dumps(response) + "\n")
    sys.stdout.flush()


def error_response(source: str, code: str, message: str) -> dict:
    """Build an error response."""
    return {
        "data": [],
        "meta": build_meta(source, "0.0.0", ""),
        "raw": None,
        "errors": [{"code": code, "message": message}],
    }


def handle_describe(schemas: list[dict]) -> bool:
    """If --describe was passed, emit tool schemas and return True."""
    if "--describe" in sys.argv:
        sys.stdout.write(json.dumps(schemas) + "\n")
        sys.stdout.flush()
        return True
    return False
