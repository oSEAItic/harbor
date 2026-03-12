#!/usr/bin/env bash
set -euo pipefail

# Harbor setup script — installs CLI + configures MCP
# Called by agents via the Harbor skill

HARBOR_BIN="$(command -v harbor 2>/dev/null || true)"

# ── Install Harbor CLI ──────────────────────────────────────────────
if [ -z "$HARBOR_BIN" ]; then
  echo "Installing Harbor CLI..."
  curl -fsSL https://harbor.oseaitic.com/install | bash
  HARBOR_BIN="$(command -v harbor 2>/dev/null || true)"
  if [ -z "$HARBOR_BIN" ]; then
    # Installer may put it in ~/bin or ~/.local/bin
    for candidate in "$HOME/bin/harbor" "$HOME/.local/bin/harbor" "/usr/local/bin/harbor"; do
      if [ -x "$candidate" ]; then
        HARBOR_BIN="$candidate"
        break
      fi
    done
  fi
  if [ -z "$HARBOR_BIN" ]; then
    echo "ERROR: Harbor installation failed. Try manually: curl -fsSL https://harbor.oseaitic.com/install | bash"
    exit 1
  fi
  echo "Harbor installed: $HARBOR_BIN"
else
  echo "Harbor already installed: $HARBOR_BIN"
fi

# ── Show version ────────────────────────────────────────────────────
"$HARBOR_BIN" version 2>/dev/null || true

# ── Configure MCP (detect agent) ───────────────────────────────────
MCP_CONFIG=""
MCP_ENTRY='{"harbor":{"command":"harbor","args":["mcp"]}}'

# Claude Code
if [ -f "$HOME/.claude/mcp.json" ]; then
  MCP_CONFIG="$HOME/.claude/mcp.json"
elif [ -d "$HOME/.claude" ]; then
  MCP_CONFIG="$HOME/.claude/mcp.json"
fi

# Cursor (project-level)
if [ -f ".cursor/mcp.json" ]; then
  MCP_CONFIG=".cursor/mcp.json"
fi

if [ -n "$MCP_CONFIG" ]; then
  if grep -q '"harbor"' "$MCP_CONFIG" 2>/dev/null; then
    echo "Harbor already configured in $MCP_CONFIG"
  else
    echo "Note: Add Harbor to your MCP config ($MCP_CONFIG):"
    echo '  "mcpServers": '"$MCP_ENTRY"
    echo ""
    echo "Or add manually:"
    echo '  { "mcpServers": { "harbor": { "command": "harbor", "args": ["mcp"] } } }'
  fi
else
  echo "MCP config not detected. Add Harbor to your agent's MCP config:"
  echo '  { "mcpServers": { "harbor": { "command": "harbor", "args": ["mcp"] } } }'
fi

# ── List available connectors ───────────────────────────────────────
echo ""
echo "Available connectors:"
"$HARBOR_BIN" list 2>/dev/null || echo "  Run 'harbor list' to see available connectors"

echo ""
echo "Setup complete. Harbor is ready."
echo "Next: install a connector with 'harbor install <name>'"
