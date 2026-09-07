#!/bin/bash -e
# Create 3 JMeter EC2s (1 client + 2 servers) in the EKS VPC and write state to a file.
#
# Required env vars (set by run-api-gateway-eks-tests.sh):
#   EKS_CLUSTER_NAME, AWS_REGION, JMETER_SG_TAG, JMETER_KEY_NAME,
#   JMETER_CLIENT_EC2_INSTANCE_TYPE, JMETER_SERVER_EC2_INSTANCE_TYPE,
#   STATE_FILE (path to write instance IDs and IPs)

set -euo pipefail

: "${EKS_CLUSTER_NAME:?}"
: "${AWS_REGION:?}"
: "${JMETER_KEY_NAME:?}"
: "${JMETER_CLIENT_EC2_INSTANCE_TYPE:?}"
: "${JMETER_SERVER_EC2_INSTANCE_TYPE:?}"
: "${STATE_FILE:?}"

AWS="aws --region ${AWS_REGION}"

echo "==> Getting EKS VPC info for cluster ${EKS_CLUSTER_NAME}"
VPC_ID=$(${AWS} eks describe-cluster --name "${EKS_CLUSTER_NAME}" \
    --query "cluster.resourcesVpcConfig.vpcId" --output text)
echo "    VPC: ${VPC_ID}"

# Get a public subnet in the EKS VPC (tagged by eksctl for public load balancer).
PUBLIC_SUBNET_ID=$(${AWS} ec2 describe-subnets \
    --filters "Name=vpc-id,Values=${VPC_ID}" \
              "Name=tag:kubernetes.io/role/elb,Values=1" \
              "Name=state,Values=available" \
    --query "Subnets[0].SubnetId" --output text)

if [[ -z "${PUBLIC_SUBNET_ID}" || "${PUBLIC_SUBNET_ID}" == "None" ]]; then
    echo "ERROR: No public subnet (tag kubernetes.io/role/elb=1) found in VPC ${VPC_ID}." >&2
    echo "       eksctl usually creates these; check VPC subnet tags." >&2
    exit 1
fi
echo "    Subnet: ${PUBLIC_SUBNET_ID}"

# Create security group for JMeter EC2s.
JMETER_SG_NAME="${JMETER_SG_TAG:-jmeter-perf}"
echo "==> Creating JMeter security group: ${JMETER_SG_NAME}"
JMETER_SG_ID=$(${AWS} ec2 create-security-group \
    --group-name "${JMETER_SG_NAME}" \
    --description "JMeter perf test instances for ${EKS_CLUSTER_NAME}" \
    --vpc-id "${VPC_ID}" \
    --query "GroupId" --output text)
echo "    JMeter SG: ${JMETER_SG_ID}"

# Allow SSH from anywhere (Jenkins slave can SSH to JMeter EC2s).
${AWS} ec2 authorize-security-group-ingress \
    --group-id "${JMETER_SG_ID}" \
    --protocol tcp --port 22 --cidr 0.0.0.0/0

# Allow all TCP between JMeter instances themselves (RMI + results callbacks).
${AWS} ec2 authorize-security-group-ingress \
    --group-id "${JMETER_SG_ID}" \
    --ip-permissions \
    "IpProtocol=tcp,FromPort=0,ToPort=65535,UserIdGroupPairs=[{GroupId=${JMETER_SG_ID}}]"

# Get EKS cluster security group and allow JMeter access on NodePort range.
NODE_SG=$(${AWS} eks describe-cluster --name "${EKS_CLUSTER_NAME}" \
    --query "cluster.resourcesVpcConfig.clusterSecurityGroupId" --output text)
echo "    EKS cluster SG: ${NODE_SG}"

${AWS} ec2 authorize-security-group-ingress \
    --group-id "${NODE_SG}" \
    --ip-permissions \
    "IpProtocol=tcp,FromPort=0,ToPort=65535,UserIdGroupPairs=[{GroupId=${JMETER_SG_ID}}]"

# Get latest Amazon Linux 2023 AMI.
AMI_ID=$(${AWS} ssm get-parameter \
    --name /aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64 \
    --query "Parameter.Value" --output text)
echo "==> Using AMI: ${AMI_ID}"

launch_ec2() {
    local role="$1" instance_type="$2"
    ${AWS} ec2 run-instances \
        --image-id "${AMI_ID}" \
        --instance-type "${instance_type}" \
        --key-name "${JMETER_KEY_NAME}" \
        --security-group-ids "${JMETER_SG_ID}" \
        --subnet-id "${PUBLIC_SUBNET_ID}" \
        --associate-public-ip-address \
        --block-device-mappings "DeviceName=/dev/xvda,Ebs={VolumeSize=50,VolumeType=gp3,DeleteOnTermination=true}" \
        --tag-specifications \
        "ResourceType=instance,Tags=[{Key=Name,Value=${JMETER_SG_NAME}-${role}},{Key=project,Value=${EKS_CLUSTER_NAME}},{Key=jmeter-perf-tag,Value=${JMETER_SG_NAME}}]" \
        --query "Instances[0].InstanceId" --output text
}

echo "==> Launching JMeter EC2s"
CLIENT_ID=$(launch_ec2 "client" "${JMETER_CLIENT_EC2_INSTANCE_TYPE}")
SERVER1_ID=$(launch_ec2 "server-1" "${JMETER_SERVER_EC2_INSTANCE_TYPE}")
SERVER2_ID=$(launch_ec2 "server-2" "${JMETER_SERVER_EC2_INSTANCE_TYPE}")
echo "    client:   ${CLIENT_ID}"
echo "    server-1: ${SERVER1_ID}"
echo "    server-2: ${SERVER2_ID}"

echo "==> Waiting for instances to be running (~60s)..."
${AWS} ec2 wait instance-running --instance-ids "${CLIENT_ID}" "${SERVER1_ID}" "${SERVER2_ID}"

# Get public + private IPs.
get_ip() {
    local id="$1" type="$2"
    ${AWS} ec2 describe-instances --instance-ids "${id}" \
        --query "Reservations[0].Instances[0].${type}IpAddress" --output text
}

CLIENT_PUBLIC_IP=$(get_ip "${CLIENT_ID}" Public)
SERVER1_PUBLIC_IP=$(get_ip "${SERVER1_ID}" Public)
SERVER2_PUBLIC_IP=$(get_ip "${SERVER2_ID}" Public)
SERVER1_PRIVATE_IP=$(get_ip "${SERVER1_ID}" Private)
SERVER2_PRIVATE_IP=$(get_ip "${SERVER2_ID}" Private)

echo ""
echo "    client   ${CLIENT_ID}   public=${CLIENT_PUBLIC_IP}"
echo "    server-1 ${SERVER1_ID}  public=${SERVER1_PUBLIC_IP}  private=${SERVER1_PRIVATE_IP}"
echo "    server-2 ${SERVER2_ID}  public=${SERVER2_PUBLIC_IP}  private=${SERVER2_PRIVATE_IP}"

# Write state file for use by main script and cleanup.
cat >"${STATE_FILE}" <<EOF
JMETER_SG_ID=${JMETER_SG_ID}
NODE_SG=${NODE_SG}
VPC_ID=${VPC_ID}
CLIENT_ID=${CLIENT_ID}
SERVER1_ID=${SERVER1_ID}
SERVER2_ID=${SERVER2_ID}
CLIENT_PUBLIC_IP=${CLIENT_PUBLIC_IP}
SERVER1_PUBLIC_IP=${SERVER1_PUBLIC_IP}
SERVER2_PUBLIC_IP=${SERVER2_PUBLIC_IP}
SERVER1_PRIVATE_IP=${SERVER1_PRIVATE_IP}
SERVER2_PRIVATE_IP=${SERVER2_PRIVATE_IP}
EOF
echo "==> State written to ${STATE_FILE}"
