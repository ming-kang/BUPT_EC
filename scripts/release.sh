#!/usr/bin/env bash
# Cut a stable release: roll CHANGELOG.md, bump the frontend version,
# commit, tag, and push. The Release workflow does the rest.
# Usage: scripts/release.sh vX.Y.Z
set -euo pipefail

REPO_URL="https://github.com/ming-kang/BUPT_EC"

version="${1:?usage: release.sh vX.Y.Z}"
if [[ ! "${version}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Version must look like v0.1.4, got: ${version}" >&2
  exit 1
fi
bare="${version#v}"

branch="$(git rev-parse --abbrev-ref HEAD)"
if [[ "${branch}" != "main" ]]; then
  echo "Releases are cut from main; current branch is ${branch}." >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "Working tree is not clean; commit or stash first." >&2
  exit 1
fi

# --force lets the rolling nightly tag update instead of failing the fetch.
git fetch --force origin main --tags
if [[ "$(git rev-parse HEAD)" != "$(git rev-parse origin/main)" ]]; then
  echo "Local main is not in sync with origin/main; pull or push first." >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/${version}" >/dev/null; then
  echo "Tag ${version} already exists." >&2
  exit 1
fi

echo "Release notes for ${version} (from the Unreleased section):"
echo "----------------------------------------------------------"
scripts/extract-changelog.sh Unreleased
echo "----------------------------------------------------------"

prev="$(sed -n 's#^\[Unreleased\]: .*/compare/\(v[0-9.]*\)\.\.\.HEAD$#\1#p' CHANGELOG.md)"
if [[ -z "${prev}" ]]; then
  echo "Could not find the previous version in the [Unreleased] compare link." >&2
  exit 1
fi

today="$(date +%Y-%m-%d)"

# Move the Unreleased content under the new version heading.
awk -v ver="${bare}" -v today="${today}" '
  /^## \[Unreleased\]$/ && !done {
    print
    print ""
    print "## [" ver "] - " today
    done = 1
    next
  }
  { print }
' CHANGELOG.md > CHANGELOG.md.tmp && mv CHANGELOG.md.tmp CHANGELOG.md

# Refresh the compare links at the bottom.
sed -i \
  "s#^\[Unreleased\]: .*#[Unreleased]: ${REPO_URL}/compare/${version}...HEAD\n[${bare}]: ${REPO_URL}/compare/${prev}...${version}#" \
  CHANGELOG.md

sed -i "s/\"version\": \"[^\"]*\"/\"version\": \"${bare}\"/" frontend/package.json

git add CHANGELOG.md frontend/package.json
git commit -m "chore: release ${version}"
git tag "${version}"

echo
echo "Created commit and tag ${version}."
read -r -p "Push main and ${version} to origin now? [y/N] " answer
if [[ "${answer}" =~ ^[Yy]$ ]]; then
  git push origin main "${version}"
  echo "Pushed. The Release workflow will publish ${version}."
else
  echo "Not pushed. When ready: git push origin main ${version}"
fi
