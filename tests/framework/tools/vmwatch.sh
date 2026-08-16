#!/usr/bin/env bash
# Sample the Colima VM while a suite runs. DIAGNOSTIC ONLY — nothing in the framework calls
# this, and nothing should. It was written to investigate full-suite runs that stop making
# progress, on the assumption the Docker daemon was dying.
#
# THAT ASSUMPTION WAS WRONG, and this script is what disproved it. journalctl shows dockerd
# alive and idle across the entire stall — no panic, no OOM, no restart — with several minutes
# of ZERO API activity before the VM was manually restarted. The daemon was not dying; nothing
# was asking it to do anything. Every clean reading below (peak memory 7%, load 0.22, dockerd
# RSS ~116MB, 8 containers) is consistent with that, and was misread as "the cause is elsewhere"
# when it actually meant "there is no resource failure here at all".
#
# So the open question is NOT why the daemon dies. It is why the SUITE stops calling it, and
# the answer is not on this side of the socket. Reach for these two first instead:
#   kill -QUIT <test-pid>          — Go dumps every goroutine; names the blocked line directly
#   colima ssh -- docker ps        — works from inside the VM even if host->VM forwarding broke
# Use this sampler only to rule resource pressure in or out; it cannot see a blocked test.
#
#   ./vmwatch.sh                  # samples every 5s to vmwatch-<timestamp>.csv
#   ./vmwatch.sh 2 /tmp/run.csv   # every 2s, to a named file
#
# Run it in one terminal, the suite in another. Ctrl-C when the suite ends: it prints a summary
# and, crucially, dumps any OOM kills from the SAME boot — the check that was missed before,
# because a VM restart wipes the evidence and dmesg from a later boot proves nothing.
set -uo pipefail

INTERVAL="${1:-5}"
OUT="${2:-vmwatch-$(date +%Y%m%d-%H%M%S).csv}"

command -v colima >/dev/null || { echo "colima not found" >&2; exit 1; }
colima ssh -- true >/dev/null 2>&1 || { echo "cannot reach the colima VM" >&2; exit 1; }

# Boot id, so a summary can say whether the VM restarted underneath the run — which is itself a
# finding, and different from the daemon crashing.
BOOT_AT="$(colima ssh -- uptime -s 2>/dev/null | tr -d '\r')"

echo "sampling every ${INTERVAL}s -> ${OUT}"
echo "vm booted at: ${BOOT_AT:-unknown}"
echo "time,mem_total_mb,mem_used_mb,mem_available_mb,load1,containers,dockerd_rss_mb" > "$OUT"

summary() {
  echo
  echo "=== summary ==="
  echo "samples: $(( $(wc -l < "$OUT") - 1 ))   file: $OUT"
  # Peak used and lowest available are the two numbers that matter for an OOM hypothesis.
  awk -F, 'NR>1 && $3>m {m=$3} END {printf "peak mem used:      %s MB\n", m}' "$OUT"
  awk -F, 'NR>1 {if (n=="" || $4<n) n=$4} END {printf "lowest available:   %s MB\n", n}' "$OUT"
  awk -F, 'NR>1 && $6>c {c=$6} END {printf "peak containers:    %s\n", c}' "$OUT"
  awk -F, 'NR>1 && $7>d {d=$7} END {printf "peak dockerd rss:   %s MB\n", d}' "$OUT"

  NOW_BOOT="$(colima ssh -- uptime -s 2>/dev/null | tr -d '\r')"
  if [ -n "$NOW_BOOT" ] && [ "$NOW_BOOT" != "$BOOT_AT" ]; then
    echo "!! THE VM REBOOTED DURING THE RUN ($BOOT_AT -> $NOW_BOOT)"
    echo "   dmesg below is from the NEW boot and cannot describe the failure."
  fi

  echo
  echo "=== oom kills this boot ==="
  colima ssh -- sudo dmesg 2>/dev/null \
    | grep -iE "out of memory|oom-kill|killed process" | tail -20 \
    || echo "(none found)"
  exit 0
}
trap summary INT TERM

while true; do
  # One ssh round trip per sample: free, load, and dockerd's own RSS. dockerd is included
  # deliberately — a daemon that balloons is a different story from containers filling the VM.
  # POSIX sh only — colima ssh does not give us bash, so no process substitution here.
  line="$(colima ssh -- sh -c 'free -m | awk "/^Mem:/ {printf \"%s,%s,%s,\", \$2, \$3, \$7}"; cut -d" " -f1 /proc/loadavg | tr -d "\n"; printf ","; ps -eo rss,comm 2>/dev/null | awk "\$2==\"dockerd\" {s+=\$1} END {printf \"%d\", s/1024}"' 2>/dev/null | tr -d '\r')"

  # Container count from the host side: it is the docker CLI view, which is what the suite sees.
  containers="$(docker ps -q 2>/dev/null | wc -l | tr -d ' ')"

  if [ -n "$line" ]; then
    IFS=, read -r tot used avail load1 rss <<< "$line"
    printf "%s,%s,%s,%s,%s,%s,%s\n" \
      "$(date -u +%H:%M:%S)" "$tot" "$used" "$avail" "$load1" "$containers" "$rss" >> "$OUT"
    printf "\r%s  used=%sMB avail=%sMB load=%s containers=%s dockerd=%sMB    " \
      "$(date -u +%H:%M:%S)" "$used" "$avail" "$load1" "$containers" "$rss"
  else
    # A failed sample is DATA, not an error to hide: it usually means the VM or its ssh has
    # gone away, which is exactly the event being hunted.
    printf "%s,,,,,%s,\n" "$(date -u +%H:%M:%S)" "$containers" >> "$OUT"
    printf "\r%s  SAMPLE FAILED (vm unreachable) containers=%s    " \
      "$(date -u +%H:%M:%S)" "$containers"
  fi

  sleep "$INTERVAL"
done
