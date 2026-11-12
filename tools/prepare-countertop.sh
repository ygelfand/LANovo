#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
version='v0.1.1-0.20261110231711-78ac0ed14934'
target=third_party/libcountertop
if [ -e "$target" ]; then
  echo "Already exists: $target; remove or move it manually before regenerating." >&2
  exit 1
fi
json=$(GOWORK=off go mod download -json "github.com/ygelfand/libcountertop@$version")
source=$(printf '%s\n' "$json" | sed -n 's/^[[:space:]]*"Dir": "\([^"]*\)",*$/\1/p')
[ -n "$source" ] && [ -d "$source" ] || { echo 'Could not locate module cache' >&2; exit 1; }
mkdir -p third_party
cp -R "$source" "$target"
chmod -R u+w "$target"
(cd "$target" && patch -p1 < ../../patches/libcountertop-x501f.patch)
echo "Prepared $target from $version"
