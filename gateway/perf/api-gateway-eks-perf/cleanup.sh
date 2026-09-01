#!/bin/bash
# Clean up all resources created by run-api-gateway-eks-tests.sh.
# Called from the EXIT trap; must not fail hard (|| true guards).
#
# Required env vars:
#   EKS_CLUSTER_NAME, AWS_REGION
#   STATE_FILE  path to the jmeter state file written by create-jmeter-ec2s.sh

set -uo pipefail

AWS="aws --region ${AWS_REGION:-us-east-1}"

echo "==> Cleanup: EKS cluster ${EKS_CLUSTER_NAME:-<not set>}, region ${AWS_REGION:-us-east-1}"

# Load JMeter EC2 state if it exists.
if [[ -f "${STATE_FILE:-}" ]]; then
    # shellcheck source=/dev/null
    source "${STATE_FILE}"
fi

# Terminate JMeter EC2s first.
INSTANCE_IDS=()
for var in CLIENT_ID SERVER1_ID SERVER2_ID; do
    id="${!var:-}"
    [[ -n "$id" && "$id" != "None" ]] && INSTANCE_IDS+=("$id")
done

if [[ ${#INSTANCE_IDS[@]} -gt 0 ]]; then
    echo "==> Terminating JMeter EC2s: ${INSTANCE_IDS[*]}"
    ${AWS} ec2 terminate-instances --instance-ids "${INSTANCE_IDS[@]}" >/dev/null 2>&1 || true
    echo "    Waiting for termination..."
    ${AWS} ec2 wait instance-terminated --instance-ids "${INSTANCE_IDS[@]}" 2>/dev/null || true
    echo "    JMeter EC2s terminated."
fi

# Remove JMeter SG rule from EKS node SG.
if [[ -n "${NODE_SG:-}" && -n "${JMETER_SG_ID:-}" ]]; then
    echo "==> Removing JMeter SG rule from EKS node SG ${NODE_SG}"
    ${AWS} ec2 revoke-security-group-ingress \
        --group-id "${NODE_SG}" \
        --ip-permissions \
        "IpProtocol=tcp,FromPort=0,ToPort=65535,UserIdGroupPairs=[{GroupId=${JMETER_SG_ID}}]" \
        2>/dev/null || true
fi

# Delete JMeter security group (must wait for EC2s to terminate first).
if [[ -n "${JMETER_SG_ID:-}" ]]; then
    echo "==> Deleting JMeter security group ${JMETER_SG_ID}"
    ${AWS} ec2 delete-security-group --group-id "${JMETER_SG_ID}" 2>/dev/null || \
        echo "    (SG delete failed — may have dependent resources still detaching; manual cleanup needed)"
fi

# Delete EKS cluster.
if [[ -n "${EKS_CLUSTER_NAME:-}" ]]; then
    echo "==> Deleting EKS cluster ${EKS_CLUSTER_NAME} (this takes 10-20 min, runs async)..."
    eksctl delete cluster --name "${EKS_CLUSTER_NAME}" --region "${AWS_REGION:-us-east-1}" \
        --wait 2>/dev/null || true
    echo "    EKS cluster deletion initiated."
fi

# Remove per-run state file.
if [[ -f "${STATE_FILE:-}" ]]; then
    rm -f "${STATE_FILE}"
    echo "    Removed state file ${STATE_FILE}"
fi

echo "==> Cleanup complete."
