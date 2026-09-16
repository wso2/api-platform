#!/usr/bin/env bash
# Merge and render coverage artifacts collected from framework test blocks.
set -euo pipefail

here="$(cd "$(dirname "$0")/.." && pwd)"
repo_root="$(cd "$here/../.." && pwd)"
out="${COVERAGE_OUT:-$here/suites/it/coverage-out}"
tools="$here/tools/coverage"
# A literal newline: command substitution strips trailing newlines, so it cannot build one.
newline='
'

die() { echo "coverage-report: $*" >&2; exit 1; }
[ -d "$out" ] || die "no coverage output at $out — run the suite with -coverage first"

# Rejected up front rather than left to fail further down: "go tool covdata -i" takes a
# comma-separated list with no escape for a comma inside a path, and the newest-run lookup
# below reads one path per line. Either character truncates a path into something that only
# surfaces later as a confusing missing-directory error.
case "$out" in
	*,*) die "COVERAGE_OUT must not contain a comma: go tool covdata reads -i as a comma-separated list" ;;
	*"$newline"*) die "COVERAGE_OUT must not contain a newline" ;;
esac

# Every artifact this script consumes is written by the sink under <run>/raw. COVERAGE_OUT
# may name a run directory or the parent that collects them, so resolve the single run to
# report on: merging counters from two runs yields a percentage belonging to neither.
# Reports are still written under COVERAGE_OUT, keeping their published paths stable.
run_root="$out"
if [ ! -d "$run_root/raw" ]; then
	# Read whole lines: COVERAGE_OUT may contain spaces, which word splitting would tear
	# apart. ls supplies the newest-first ordering that find cannot express portably, and
	# the run directories it lists are named .run-<digits>, so a line is always one path.
	while IFS= read -r candidate; do
		[ -d "${candidate}raw" ] || continue
		run_root="${candidate%/}"
		echo "coverage-report: reporting on run $(basename "$run_root")"
		break
	done < <(ls -td "$out"/.run-*/ 2>/dev/null)
fi
raw_root="$run_root/raw"
[ -d "$raw_root" ] || die "no coverage artifacts under $out — run the suite with -coverage first"

# Generated reports are disposable. Clear previous layouts so an index never links to
# reports from an earlier run or an obsolete directory structure.
rm -rf "$out/platform-gateway" "$out/platform-api" "$out/ai-workspace" \
	"$out/api-portal/server" "$out/api-portal/ui" "$out/browser-report" "$out/node-v8-report" "$out/merged-go" \
	"$out/coverage-go.txt" "$out/coverage-go.raw.txt" "$out/coverage-go.html" \
	"$out/coverage-go.html.profile" "$out/index.html"

# Counter directories are located by the files they hold rather than by their depth: the
# sink nests them under raw/<block>/<service>, and a service may appear in several blocks.
go_counter_dirs() {
	find "$raw_root" -type f -name 'covmeta.*' -print | while IFS= read -r meta; do
		dir="$(dirname "$meta")"
		if ls "$dir"/covcounters.* >/dev/null 2>&1; then
			printf '%s\n' "$dir"
		fi
	done | sort -u
}

go_inputs=""
while IFS= read -r dir; do
	go_inputs="${go_inputs:+$go_inputs,}$dir"
done < <(go_counter_dirs)

node_files=()
while IFS= read -r file; do
	node_files+=("$file")
done < <(find "$raw_root" -type f -name 'coverage-*.json' | sort)

browser_files=()
while IFS= read -r file; do
	browser_files+=("$file")
done < <(find "$raw_root" -type f -name 'raw-istanbul.json' | sort)

[ -n "$go_inputs" ] || [ "${#node_files[@]}" -gt 0 ] || [ "${#browser_files[@]}" -gt 0 ] || die "no Go, Node/V8, or browser coverage artifacts found under $raw_root"

if [ -n "$go_inputs" ]; then
	echo "merging Go coverage: $go_inputs"
	# Render each instrumented service independently. The service directory is the
	# ownership boundary because one block can contain several product processes.
	render_go_report() {
		local service="$1" inputs="$2" report_dir="$3"
		rm -rf "$report_dir" "$out/.go-$service"
		mkdir -p "$report_dir" "$out/.go-$service"
		go tool covdata merge -i="$inputs" -o="$out/.go-$service"
		go tool covdata textfmt -i="$out/.go-$service" -o="$report_dir/coverage.raw.txt"
		cp "$report_dir/coverage.raw.txt" "$report_dir/coverage.txt"
		local html_profile="$report_dir/coverage.html.profile"
		{
			head -n 1 "$report_dir/coverage.raw.txt"
			tail -n +2 "$report_dir/coverage.raw.txt" | while IFS= read -r line; do
				local file="${line%%:*}" source
				case "$file" in
					github.com/wso2/api-platform/*) source="$repo_root/${file#github.com/wso2/api-platform/}" ;;
					ai-workspace-bff/*) source="$repo_root/portals/ai-workspace/bff/${file#ai-workspace-bff/}" ;;
					*) source="$repo_root/$file" ;;
				 esac
				[ -f "$source" ] && printf '%s\n' "$line"
			done || true
		} > "$html_profile"
		go tool cover -html="$html_profile" -o "$report_dir/coverage.html"
		local covered total
		covered=$(awk 'NR>1 && $NF>0 {n+=$(NF-1)} END {print n+0}' "$report_dir/coverage.txt")
		total=$(awk 'NR>1 {n+=$(NF-1)} END {print n+0}' "$report_dir/coverage.txt")
		[ "$covered" -gt 0 ] || die "$service Go coverage contains no executed statements"
		awk -v covered="$covered" -v total="$total" 'BEGIN {printf "Go coverage: %.1f%% of statements (%d/%d)\n", 100*covered/total, covered, total}'
		printf '{"type":"go","covered":%d,"total":%d,"percent":%.2f}\n' "$covered" "$total" "$(awk -v c="$covered" -v t="$total" 'BEGIN {print 100*c/t}')" > "$report_dir/summary.json"
	}

	for service in gateway-controller gateway-runtime platform-api ai-workspace; do
		service_inputs=""
		while IFS= read -r dir; do
			[ "$(basename "$dir")" = "$service" ] || continue
			service_inputs="${service_inputs:+$service_inputs,}$dir"
		done < <(go_counter_dirs)
		[ -n "$service_inputs" ] || continue
		case "$service" in
			gateway-controller) report_dir="$out/platform-gateway/controller" ;;
			gateway-runtime) report_dir="$out/platform-gateway/runtime" ;;
			platform-api) report_dir="$out/platform-api" ;;
			ai-workspace) report_dir="$out/ai-workspace/bff" ;;
			*) continue ;;
		esac
		render_go_report "$service" "$service_inputs" "$report_dir"
	done
	rm -rf "$out/.go-"*
fi

if [ "${#node_files[@]}" -gt 0 ]; then
	command -v npm >/dev/null 2>&1 || die "Node/V8 artifacts found, but npm is unavailable"
	[ -f "$tools/package-lock.json" ] || die "Node/V8 report tool is not locked at $tools/package-lock.json"
	rm -rf "$out/node-v8-input" "$out/api-portal/server"
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
		--report-dir="$out/api-portal/server" \
		--include="$node_include" \
		--allowExternal \
		--reporter=text --reporter=html --reporter=json-summary --reporter=lcov
	[ -s "$out/api-portal/server/coverage-summary.json" ] || die "Node/V8 reporter produced no summary"
	node -e 'const s=require(process.argv[1]); if (!s.total || s.total.statements.total === 0) process.exit(1)' \
		"$out/api-portal/server/coverage-summary.json" \
		|| die "Node/V8 coverage contains no instrumented statements"
fi

if [ "${#browser_files[@]}" -gt 0 ]; then
	command -v npm >/dev/null 2>&1 || die "browser coverage artifacts found, but npm is unavailable"
	[ -f "$tools/package-lock.json" ] || die "browser coverage report tool is not locked at $tools/package-lock.json"
	npm --prefix "$tools" ci --ignore-scripts --no-audit --no-fund >/dev/null
	set -- "$tools/browser-to-istanbul.js" "$raw_root" "$out/browser-report" "$repo_root" \
		--source-root portals/ai-workspace/src \
		--source-root portals/api-portal/src/scripts \
		--inventory-root portals/ai-workspace/src \
		--inventory-root portals/api-portal/src/scripts \
		--include 'portals/ai-workspace/src/**/*.js' \
		--include 'portals/ai-workspace/src/**/*.jsx' \
		--include 'portals/ai-workspace/src/**/*.ts' \
		--include 'portals/ai-workspace/src/**/*.tsx' \
		--include 'portals/api-portal/src/scripts/**/*.js' \
		--include 'portals/api-portal/src/scripts/**/*.jsx' \
		--include 'portals/api-portal/src/scripts/**/*.ts' \
		--include 'portals/api-portal/src/scripts/**/*.tsx' \
		--exclude '**/*.test.*' \
		--exclude '**/node_modules/**'
	echo "merging browser coverage: ${#browser_files[@]} files"
	node "$@"
	[ -s "$out/browser-report/lcov.info" ] || die "browser coverage reporter produced no LCOV report"
	[ -s "$out/browser-report/summary.json" ] || die "browser coverage reporter produced no summary"
	command -v rg >/dev/null 2>&1 || die "ripgrep (rg) is unavailable"

	# Keep product reports separate so a combined UI percentage cannot hide which
	# frontend produced it. The combined report above remains useful for one upload.
	run_product_browser_report() {
		local name="$1" source_root="$2" inventory_root="$3" pattern="$4"
		local report_dir
		case "$name" in
			ai-workspace-ui) report_dir="$out/ai-workspace/ui" ;;
			api-portal-ui) report_dir="$out/api-portal/ui" ;;
			*) die "unknown browser report $name" ;;
		esac
		local -a report_args=("$tools/browser-to-istanbul.js" "$raw_root" "$report_dir" "$repo_root"
			--source-root "$source_root" --inventory-root "$inventory_root" --include "$pattern"
			--exclude '**/*.test.*' --exclude '**/node_modules/**')
		echo "merging $name browser coverage"
		node "${report_args[@]}"
		[ -s "$report_dir/lcov.info" ] || die "$name browser coverage reporter produced no LCOV report"
	}

	if find "$raw_root/blocks" -type f -name 'raw-istanbul.json' -print -quit | grep -q . \
		&& rg -l 'AIWorkspace|/web/src/(App|Components|pages)/' "$raw_root/blocks" >/dev/null; then
		run_product_browser_report ai-workspace-ui portals/ai-workspace/src portals/ai-workspace/src 'portals/ai-workspace/src/**'
	fi
	if find "$raw_root/blocks" -type f -name 'raw-istanbul.json' -print -quit | grep -q . \
		&& rg -l '/web/src/scripts/' "$raw_root/blocks" >/dev/null; then
		run_product_browser_report api-portal-ui portals/api-portal/src/scripts portals/api-portal/src/scripts 'portals/api-portal/src/scripts/**/*.js'
	fi
fi

node "$tools/index.js" "$out"

echo "reports under $out"
