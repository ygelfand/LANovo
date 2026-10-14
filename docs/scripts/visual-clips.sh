#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
out="$root/docs/public/visuals"
list="$root/docs/src/data/visuals.json"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

width=${WIDTH:-640}
height=${HEIGHT:-400}
seconds=${SECONDS_LONG:-4}
fps=${FPS:-15}
quality=${QUALITY:-55}
delay=$((1000 / fps))
remote=/data/local/tmp/lanovo-clip

command -v img2webp >/dev/null || { echo "install webp first: brew install webp" >&2; exit 1; }
mkdir -p "$out" "$(dirname "$list")"

kinds=${*:-$(ls "$root/internal/ui/visual/thumbs" | grep -v -- -portrait | sed 's/\.jpg$//')}

echo "[" >"$list.tmp"
first=1
for kind in $kinds; do
	label=$(awk -F': ' -v k="visual.$kind" '$1 == k { print $2 }' "$root/internal/lib/say/messages/en.yaml")
	printf '%-18s ' "$kind"
	adb shell "rm -rf $remote && lanovod ctl clip $kind $remote $width $height $seconds $fps" >/dev/null
	rm -rf "${work:?}/$kind"
	adb pull "$remote" "$work/$kind" >/dev/null 2>&1
	img2webp -loop 0 -lossy -q "$quality" -d "$delay" "$work/$kind"/*.png -o "$out/$kind.webp" >/dev/null 2>&1
	du -h "$out/$kind.webp" | cut -f1
	[ $first -eq 1 ] || echo "," >>"$list.tmp"
	printf '  {"kind": "%s", "label": "%s"}' "$kind" "${label:-$kind}" >>"$list.tmp"
	first=0
done
printf '\n]\n' >>"$list.tmp"
mv "$list.tmp" "$list"
adb shell "rm -rf $remote"
du -sh "$out"
