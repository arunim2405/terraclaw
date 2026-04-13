#!/usr/bin/env bash
set -euo pipefail

# ============================================================================
# Terraclaw Docker Entrypoint
#
# Environment variables:
#   TERRACLAW_CMD_B64   (required)  Base64-encoded terraclaw CLI command.
#                                   Example (before encoding):
#                                     terraclaw generate --resources arn:aws:s3:::my-bucket --schema aws
#
#   OPENCLAW_CREDS_B64  (required)  Base64-encoded JSON content for OpenCode
#                                   provider auth, written to
#                                   ~/.local/share/opencode/auth.json
#
#   AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN, AWS_REGION
#                       (optional)  Standard AWS credential env vars.
#                                   Steampipe auto-detects these.
#
#   AZURE_SUBSCRIPTION_ID, AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET
#                       (optional)  Standard Azure service principal credentials.
#                                   Steampipe Azure plugin auto-detects these.
#
#   STEAMPIPE_PLUGINS   (optional)  Comma-separated extra plugins to install
#                                   at startup, e.g. "azure,gcp"
#
#   CLOUD_PROVIDER      (optional)  "aws" or "azure". Determines which default
#                                   Steampipe plugin to install. Default: "aws".
# ============================================================================

log() { echo "[entrypoint] $(date '+%H:%M:%S') $*"; }

# ---------------------------------------------------------------------------
# 1. Decode and write OpenCode auth credentials
# ---------------------------------------------------------------------------
if [ -z "${OPENCLAW_CREDS_B64:-}" ]; then
    log "ERROR: OPENCLAW_CREDS_B64 is not set. Provide base64-encoded auth.json content."
    exit 1
fi

AUTH_DIR="$HOME/.local/share/opencode"
mkdir -p "$AUTH_DIR"
echo "$OPENCLAW_CREDS_B64" | base64 -d > "$AUTH_DIR/auth.json"
log "OpenCode auth written to $AUTH_DIR/auth.json"

# ---------------------------------------------------------------------------
# 2. Decode terraclaw command
# ---------------------------------------------------------------------------
if [ -z "${TERRACLAW_CMD_B64:-}" ]; then
    log "ERROR: TERRACLAW_CMD_B64 is not set. Provide base64-encoded terraclaw CLI command."
    exit 1
fi

TERRACLAW_CMD=$(echo "$TERRACLAW_CMD_B64" | base64 -d)
log "Decoded command: $TERRACLAW_CMD"

# ---------------------------------------------------------------------------
# 3. Install extra Steampipe plugins if requested
# ---------------------------------------------------------------------------
if [ -n "${STEAMPIPE_PLUGINS:-}" ]; then
    IFS=',' read -ra PLUGINS <<< "$STEAMPIPE_PLUGINS"
    for plugin in "${PLUGINS[@]}"; do
        plugin=$(echo "$plugin" | xargs)  # trim whitespace
        log "Installing steampipe plugin: $plugin"
        steampipe plugin install "$plugin" || log "Warning: failed to install plugin $plugin"
    done
fi

# ---------------------------------------------------------------------------
# 4. Install default Steampipe plugin based on CLOUD_PROVIDER
# ---------------------------------------------------------------------------
CLOUD_PROVIDER="${CLOUD_PROVIDER:-aws}"
case "$CLOUD_PROVIDER" in
    azure)
        if ! steampipe plugin list 2>/dev/null | grep -q azure; then
            log "Installing steampipe Azure plugin..."
            steampipe plugin install azure
        fi
        ;;
    *)
        if ! steampipe plugin list 2>/dev/null | grep -q aws; then
            log "Installing steampipe AWS plugin..."
            steampipe plugin install aws
        fi
        ;;
esac

# ---------------------------------------------------------------------------
# 5. Start Steampipe service
# ---------------------------------------------------------------------------
log "Starting Steampipe service..."
steampipe service start --database-listen local --database-port "${STEAMPIPE_PORT:-9193}"

# Wait for Steampipe to accept connections.
MAX_RETRIES=30
for i in $(seq 1 $MAX_RETRIES); do
    if steampipe service status >/dev/null 2>&1; then
        log "Steampipe is ready."
        break
    fi
    if [ "$i" -eq "$MAX_RETRIES" ]; then
        log "ERROR: Steampipe failed to start within ${MAX_RETRIES}s"
        steampipe service status || true
        exit 1
    fi
    sleep 1
done

# ---------------------------------------------------------------------------
# 6. Start OpenCode headless server in the background
# ---------------------------------------------------------------------------
log "Starting OpenCode headless server on port ${OPENCODE_PORT:-4096}..."
opencode serve --port "${OPENCODE_PORT:-4096}" &
OPENCODE_PID=$!

# Wait for OpenCode to accept connections.
OC_MAX_RETRIES=30
for i in $(seq 1 $OC_MAX_RETRIES); do
    if curl -sf "http://127.0.0.1:${OPENCODE_PORT:-4096}/session" >/dev/null 2>&1; then
        log "OpenCode is ready (pid=$OPENCODE_PID)."
        break
    fi
    if [ "$i" -eq "$OC_MAX_RETRIES" ]; then
        log "ERROR: OpenCode failed to start within ${OC_MAX_RETRIES}s"
        exit 1
    fi
    sleep 1
done

# ---------------------------------------------------------------------------
# 7. Run terraclaw
# ---------------------------------------------------------------------------
log "Running: $TERRACLAW_CMD"
eval "$TERRACLAW_CMD"
EXIT_CODE=$?

# ---------------------------------------------------------------------------
# 8. Package output on success
# ---------------------------------------------------------------------------
OUTPUT_DIR="${OUTPUT_DIR:-/home/terraclaw/app/output}"
if [ "$EXIT_CODE" -eq 0 ]; then
    log "Import successful — creating imported-resource.zip"
    (cd "$OUTPUT_DIR" && zip -r /home/terraclaw/app/imported-resource.zip .)
    log "Archive created: /home/terraclaw/app/imported-resource.zip"
    if [ -n "${LOCAL_ARTIFACTS_DIR:-}" ]; then
        mkdir -p "$LOCAL_ARTIFACTS_DIR"
        cp /home/terraclaw/app/imported-resource.zip "$LOCAL_ARTIFACTS_DIR/"
        log "Copied zip to $LOCAL_ARTIFACTS_DIR/imported-resource.zip"
    fi
else
    log "Terraclaw exited with code $EXIT_CODE — skipping zip"
fi

# ---------------------------------------------------------------------------
# 9. Cleanup
# ---------------------------------------------------------------------------
log "Stopping OpenCode server..."
kill "$OPENCODE_PID" 2>/dev/null || true
wait "$OPENCODE_PID" 2>/dev/null || true

log "Stopping Steampipe service..."
steampipe service stop --force 2>/dev/null || true

log "Done (exit code: $EXIT_CODE)"
exit $EXIT_CODE
