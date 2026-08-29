#!/usr/bin/env bash
# Report what NextChapter costs on disk, and which make target reclaims each
# line. Read-only: this script never deletes anything.
#
# Run it as `make disk-report` from the repository root.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

total_kb=0

bold=$'\033[1m'
dim=$'\033[2m'
cyan=$'\033[36m'
reset=$'\033[0m'
[[ -t 1 ]] || { bold=''; dim=''; cyan=''; reset=''; }

# du -sk is the portable spelling (GNU's -b is not on macOS). Missing paths
# and unreadable subtrees report 0 rather than failing the whole report.
size_kb() {
	local total=0 path
	for path in "$@"; do
		[[ -e $path ]] || continue
		total=$((total + $(du -sk "$path" 2>/dev/null | awk '{s += $1} END {print s + 0}')))
	done
	printf '%s' "$total"
}

human() {
	awk -v kb="$1" 'BEGIN {
		split("KB MB GB TB", u, " ")
		i = 1
		while (kb >= 1024 && i < 4) { kb /= 1024; i++ }
		printf (kb < 10 && i > 1) ? "%.1f%s" : "%.0f%s", kb, u[i]
	}'
}

# Docker reports human strings ("15.42GB", "495.6kB"); normalise to KB so the
# grand total means something.
docker_kb() {
	awk -v s="$1" 'BEGIN {
		n = s + 0
		if (s ~ /TB$/)      n *= 1024 * 1024 * 1024
		else if (s ~ /GB$/) n *= 1024 * 1024
		else if (s ~ /MB$/) n *= 1024
		else if (s ~ /kB$/) n *= 1
		else                n /= 1024
		printf "%d", n
	}'
}

row() { # row <label> <kb> <reclaim-hint>
	local kb=$2
	[[ $kb -gt 0 ]] || return 0
	total_kb=$((total_kb + kb))
	printf '  %-34s %8s   %s%s%s\n' "$1" "$(human "$kb")" "$dim" "$3" "$reset"
}

section() { printf '\n%s%s%s\n' "$bold" "$1" "$reset"; }

printf '%sNextChapter disk report%s  %s%s%s\n' \
	"$bold" "$reset" "$dim" "$(date '+%Y-%m-%d %H:%M')" "$reset"

section 'Working tree'
row 'node_modules (pnpm workspace)' \
	"$(size_kb node_modules frontend/node_modules web/node_modules packages/*/node_modules)" \
	'make clean-deps'
row 'backend/bin' "$(size_kb backend/bin)" 'make clean'
row 'frontend/.output + .wxt' "$(size_kb frontend/.output frontend/.wxt)" 'make clean'
row 'web/dist' "$(size_kb web/dist)" 'make clean'
row 'dist/ (release artefacts)' "$(size_kb dist)" 'make clean'
row 'Playwright reports' \
	"$(size_kb frontend/test-results frontend/playwright-report web/test-results web/playwright-report)" \
	'make clean'
row 'local SQLite databases' "$(size_kb ./*.db backend/*.db)" 'make clean'

section 'Shared caches'
printf '  %s(used by NextChapter, shared with every other project on this machine)%s\n' "$dim" "$reset"
if command -v go >/dev/null 2>&1; then
	row 'Go build cache' "$(size_kb "$(go env GOCACHE)")" 'make clean-caches'
	row 'Go module cache' "$(size_kb "$(go env GOMODCACHE)")" 'make clean-caches DEEP=1'
fi
if command -v pnpm >/dev/null 2>&1; then
	row 'pnpm content store' "$(size_kb "$(pnpm store path 2>/dev/null || true)")" 'make clean-caches DEEP=1'
fi
row 'Playwright browsers' "$(size_kb "${HOME}/.cache/ms-playwright")" \
	'pinned; only stale versions are waste'

section 'Docker'
if docker info >/dev/null 2>&1; then
	while IFS=$'\t' read -r repo tag size; do
		row "image ${repo}:${tag}" "$(docker_kb "$size")" 'make clean-docker'
	done < <(docker images --format '{{.Repository}}\t{{.Tag}}\t{{.Size}}' 2>/dev/null |
		grep -E '^nextchapter|/nextchapter' || true)

	build_cache="$(docker system df --format '{{.Type}}\t{{.Size}}' 2>/dev/null |
		awk -F'\t' '$1 == "Build Cache" {print $2}')"
	row 'build cache (all projects)' "$(docker_kb "${build_cache:-0}")" 'make clean-docker'

	others="$(docker images --format '{{.Repository}}' 2>/dev/null |
		grep -cvE '^nextchapter|/nextchapter' || true)"
	printf '  %s%s other images on this daemon are not NextChapter'"'"'s — no clean target touches them.%s\n' \
		"$dim" "${others:-0}" "$reset"
else
	printf '  %sdocker unavailable — skipped%s\n' "$dim" "$reset"
fi

printf '\n  %s%-34s %8s%s\n' "$bold" 'TOTAL attributable' "$(human "$total_kb")" "$reset"
printf '  %sEverything above is regenerable. %smake clean-all%s%s rebuilds from source.%s\n\n' \
	"$dim" "$cyan" "$reset" "$dim" "$reset"
