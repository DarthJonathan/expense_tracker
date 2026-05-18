#!/bin/sh
set -eu

API_BASE_URL="${API_BASE_URL:-${VITE_API_BASE_URL:-}}"
ESCAPED_API_BASE_URL="$(printf '%s' "$API_BASE_URL" | sed 's/\\/\\\\/g; s/"/\\"/g')"

cat > /app/build/client/runtime-config.js <<EOF
window.__APP_CONFIG__ = window.__APP_CONFIG__ || {};
window.__APP_CONFIG__.API_BASE_URL = "$ESCAPED_API_BASE_URL";
EOF

exec node build
