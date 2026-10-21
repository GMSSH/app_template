#!/usr/bin/env bash
# Platform uninstall hook (copied to plugin root as uninstall.sh at bundle time).
# The GMSSH host invokes this when removing the plugin.
#
# Keep this script idempotent and non-interactive.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"

echo "uninstalling plugin at ${ROOT} ..."

# Remove runtime data that should not survive uninstall.
# Keep the script conservative: only delete known volatile dirs.
for d in logs tmp; do
  if [[ -d "${ROOT}/${d}" ]]; then
    rm -rf "${ROOT:?}/${d:?}"
    mkdir -p "${ROOT}/${d}"
  fi
done

echo "uninstall hooks finished"
exit 0
