#!/usr/bin/env python3
"""Write a build file containing only the named policies, for targeted runtime verification.

The gateway builder compiles every policy a build file lists — 64 of them for a full build,
most of which a given scenario never touches. Trimming to the handful a scenario uses cuts the
policy-compiler stage by roughly 40% (measured: 470s for 64 policies, 284s for 4). The saving
is in compilation; discovery costs only about 4s either way, and the remainder is the Go build
of the policy engine itself, which trimming cannot avoid.

The result is NOT a shippable build: an API referencing an omitted policy fails validation at
deploy. Use it for verifying one scenario, never for the IT suite or a release.

    ./trim-build-file.py gateway-runtime/target/build.yaml model-failover set-headers ...
"""
import os
import sys

import yaml

if len(sys.argv) < 3:
    sys.exit(__doc__)

SOURCE = "build.yaml"

out, keep = sys.argv[1], set(sys.argv[2:])

# Refuse to write over the source. The output path is argv[1] while the input is implicit,
# which reads naturally as "trim THIS file" — and passing build.yaml for both replaces the
# 64-policy source with the trimmed copy, silently losing every policy not named on the
# command line along with the local dev entries.
if os.path.realpath(out) == os.path.realpath(SOURCE):
    sys.exit(f"refusing to overwrite {SOURCE}: argv[1] is the OUTPUT path, and {SOURCE} is "
             f"always the input. Write to gateway-runtime/target/build.yaml instead.")

spec = yaml.safe_load(open(SOURCE))

all_names = {p["name"] for p in spec.get("policies", [])}
missing = keep - all_names
if missing:
    sys.exit(f"not in {SOURCE}: {', '.join(sorted(missing))}")

spec["policies"] = [p for p in spec["policies"] if p["name"] in keep]

# Rebase relative filePath entries onto the OUTPUT directory.
#
# The builder resolves a relative filePath against the directory holding the build file it was
# given. The trimmed copy is written somewhere else — gateway-runtime/target/ in the documented
# usage — so a local entry like ./dev-policies/model-failover would resolve under that directory
# instead, where nothing exists, and discovery fails for exactly the policy the developer trimmed
# the file in order to iterate on. Absolute paths are left alone.
out_dir = os.path.dirname(os.path.realpath(out)) or "."
src_dir = os.path.dirname(os.path.realpath(SOURCE)) or "."
rebased = []
for policy in spec["policies"]:
    path = policy.get("filePath")
    if not path or os.path.isabs(path):
        continue
    resolved = os.path.realpath(os.path.join(src_dir, path))
    policy["filePath"] = os.path.relpath(resolved, out_dir)
    rebased.append(f"{policy['name']} -> {policy['filePath']}")

with open(out, "w") as fh:
    yaml.safe_dump(spec, fh, default_flow_style=False, sort_keys=False)

if rebased:
    print(f"rebased {len(rebased)} local filePath entries onto {out_dir}: {', '.join(rebased)}")

print(f"{out}: {len(spec['policies'])} of {len(all_names)} policies "
      f"({', '.join(sorted(keep))})")
