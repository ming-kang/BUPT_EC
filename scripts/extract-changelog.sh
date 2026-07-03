#!/usr/bin/env bash
# Print the CHANGELOG.md section for one version to stdout.
# Usage: extract-changelog.sh <vX.Y.Z | X.Y.Z | Unreleased> [changelog-path]
set -euo pipefail

section="${1:?usage: extract-changelog.sh <vX.Y.Z | Unreleased> [changelog-path]}"
changelog="${2:-CHANGELOG.md}"

if [[ "${section}" != "Unreleased" ]]; then
  section="${section#v}"
fi

notes="$(awk -v section="${section}" '
  $0 == "## [" section "]" || index($0, "## [" section "] ") == 1 { found = 1; next }
  found && /^## \[/ { exit }
  found { print }
' "${changelog}")"

# Trim leading and trailing blank lines.
notes="$(printf "%s" "${notes}" | sed -e '/./,$!d')"

if [[ -z "${notes//[[:space:]]/}" ]]; then
  echo "No changelog content found for section [${section}] in ${changelog}." >&2
  exit 1
fi

printf "%s\n" "${notes}"
