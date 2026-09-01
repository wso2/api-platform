#!/usr/bin/env bash
# Merge and render coverage artifacts collected from framework test blocks.
set -euo pipefail

here="$(cd "$(dirname "$0")/.." && pwd)"
repo_root="$(cd "$here/../.." && pwd)"
out="${COVERAGE_OUT:-$here/suites/it/coverage-out}"
tools="$here/tools/coverage"

die() { echo "coverage-report: $*" >&2; exit 1; }
[ -d "$out" ] || die "no coverage output at $out — run the suite with -coverage first"

go_inputs=""
while IFS= read -r dir; do
	if ls "$dir"/covmeta.* >/dev/null 2>&1 && ls "$dir"/covcounters.* >/dev/null 2>&1; then
		go_inputs="${go_inputs:+$go_inputs,}$dir"
	fi
done < <(find "$out" -mindepth 2 -maxdepth 2 -type d | sort)

node_files=()
while IFS= read -r file; do
	node_files+=("$file")
done < <(find "$out" -type f -name 'coverage-*.json' \
	-not -path "$out/node-v8-input/*" -not -path "$out/node-v8-report/*" | sort)

[ -n "$go_inputs" ] || [ "${#node_files[@]}" -gt 0 ] || die "no Go or Node/V8 coverage artifacts found under $out"

if [ -n "$go_inputs" ]; then
	echo "merging Go coverage: $go_inputs"
	rm -rf "$out/merged-go"
	mkdir -p "$out/merged-go"
	go tool covdata merge -i="$go_inputs" -o "$out/merged-go"
	go tool covdata textfmt -i="$out/merged-go" -o "$out/coverage-go.raw.txt"
	cp "$out/coverage-go.raw.txt" "$out/coverage-go.txt"
	# Go profiles may contain generated files or packages supplied from outside this
	# checkout. Keep available source files in the HTML profile and retain the complete
	# text profile for upload and statement totals.
	html_profile="$out/coverage-go.html.profile"
	{
		head -n 1 "$out/coverage-go.raw.txt"
		tail -n +2 "$out/coverage-go.raw.txt" | while IFS= read -r line; do
			file="${line%%:*}"
			case "$file" in
				github.com/wso2/api-platform/*)
					source="$repo_root/${file#github.com/wso2/api-platform/}"
					;;
				ai-workspace-bff/*)
					source="$repo_root/portals/ai-workspace/bff/${file#ai-workspace-bff/}"
					;;
				*)
					source="$repo_root/$file"
					;;
			esac
			[ -f "$source" ] && printf '%s\n' "$line"
		done || true
	} > "$html_profile"
	go tool cover -html="$html_profile" -o "$out/coverage-go.html"
	covered=$(awk 'NR>1 && $NF>0 {n++} END {print n+0}' "$out/coverage-go.txt")
	[ "$covered" -gt 0 ] || die "Go coverage contains no executed statements"
	awk 'NR>1 {total+=$(NF-1); if ($NF>0) covered+=$(NF-1)}
		 END {if (total>0) printf "Go coverage: %.1f%% of statements (%d/%d)\n", 100*covered/total, covered, total}' \
		"$out/coverage-go.txt"
fi

if [ "${#node_files[@]}" -gt 0 ]; then
	command -v npm >/dev/null 2>&1 || die "Node/V8 artifacts found, but npm is unavailable"
	[ -f "$tools/package-lock.json" ] || die "Node/V8 report tool is not locked at $tools/package-lock.json"
	rm -rf "$out/node-v8-input" "$out/node-v8-report"
	mkdir -p "$out/node-v8-input"
	i=0
	for file in "${node_files[@]}"; do
		node "$tools/normalize-v8.js" "$file" "$out/node-v8-input/coverage-$i.json" \
			"$repo_root/portals/api-portal"
		i=$((i + 1))
	done
	echo "merging Node/V8 coverage: ${#node_files[@]} files"
	node_include="${COVERAGE_INCLUDE:-$repo_root/portals/api-portal/src/**/*.js}"
	npm --prefix "$tools" ci --ignore-scripts --no-audit --no-fund >/dev/null
	npm --prefix "$tools" exec -- c8 report \
		--temp-directory="$out/node-v8-input" \
		--report-dir="$out/node-v8-report" \
		--include="$node_include" \
		--allowExternal \
		--reporter=text --reporter=html --reporter=json-summary --reporter=lcov
	[ -s "$out/node-v8-report/coverage-summary.json" ] || die "Node/V8 reporter produced no summary"
	node -e 'const s=require(process.argv[1]); if (!s.total || s.total.statements.total === 0) process.exit(1)' \
		"$out/node-v8-report/coverage-summary.json" \
		|| die "Node/V8 coverage contains no instrumented statements"
fi

echo "reports under $out"
