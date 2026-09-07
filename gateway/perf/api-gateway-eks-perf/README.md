# `api-gateway-eks-perf` — Jenkins slave orchestrator

This directory lives on the **Jenkins slave** perf workspace. It is the job entrypoint; it does **not** contain JMeter scenarios or RestApi deploy logic.

Those live in **`../performance-test-scripts/`** (in git: `gateway/perf/performance-test-scripts`), which this orchestrator sparse-clones via `PERF_SCRIPTS` on every run.

| File | Role |
|------|------|
| `run-api-gateway-eks-tests.sh` | Full pipeline (EKS → gateway → JMeter → results → cleanup) |
| `create-jmeter-ec2s.sh` | 1 client + 2 server EC2s in the EKS VPC |
| `eks-cluster-perf.yaml.template` | eksctl cluster template |
| `cleanup.sh` | EXIT trap: terminate EC2s, delete cluster |

**To extend scenarios / RestApis / JMX:** edit `performance-test-scripts` and point `PERF_SCRIPTS` at your branch. See that folder’s [README](../performance-test-scripts/README.md).

**Change this folder only when:** job infra changes (EC2 counts, VPC wiring, clone paths, `env.eks` generation defaults, cleanup).
