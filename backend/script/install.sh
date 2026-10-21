#!/usr/bin/env bash
# Platform install hook (copied to plugin root as install.sh at bundle time).
# The GMSSH host invokes this after extracting the package, before launching
# app/bin/main. Keep it idempotent and non-interactive.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
echo "RunScore plugin installed at ${ROOT}"

# Make sure runtime dirs exist and are writable by the launching user.
mkdir -p "${ROOT}/tmp" "${ROOT}/logs" "${ROOT}/data"
chmod 755 "${ROOT}/tmp" "${ROOT}/logs" "${ROOT}/data" 2>/dev/null || true

echo "install hook finished"
exit 0
