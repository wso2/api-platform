# The Amazon EKS cluster for the gateway. The caller who runs Terraform becomes a cluster admin.
locals {
  namespace            = "wso2-ai-gateway"
  service_account_name = "wso2-ai-gateway"
}

resource "aws_kms_key" "eks" {
  #checkov:skip=CKV2_AWS_64:The default key policy keeps administration in this account.
  description         = "Encrypts Kubernetes secrets in the ${var.name_prefix} cluster"
  enable_key_rotation = true
}

resource "aws_iam_role" "cluster" {
  name = "${var.name_prefix}-cluster"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "eks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "cluster" {
  role       = aws_iam_role.cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}

resource "aws_eks_cluster" "main" {
  #checkov:skip=CKV_AWS_39:Readers need kubectl without a VPN. Access is limited to eks_public_access_cidrs and IAM.
  #checkov:skip=CKV_AWS_38:A variable validation rejects 0.0.0.0/0.
  name                      = var.name_prefix
  version                   = var.eks_version
  role_arn                  = aws_iam_role.cluster.arn
  enabled_cluster_log_types = ["api", "audit", "authenticator", "controllerManager", "scheduler"]

  access_config {
    authentication_mode                         = "API"
    bootstrap_cluster_creator_admin_permissions = true
  }

  vpc_config {
    subnet_ids              = aws_subnet.private[*].id
    endpoint_private_access = true
    endpoint_public_access  = true
    public_access_cidrs     = var.eks_public_access_cidrs
  }

  encryption_config {
    resources = ["secrets"]
    provider {
      key_arn = aws_kms_key.eks.arn
    }
  }

  # The log group must exist first, or Amazon EKS creates its own without a retention period.
  depends_on = [aws_iam_role_policy_attachment.cluster, aws_cloudwatch_log_group.eks]
}

resource "aws_iam_role" "node" {
  name = "${var.name_prefix}-node"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "node" {
  for_each = toset([
    "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
    "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
    "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
  ])
  role       = aws_iam_role.node.name
  policy_arn = each.value
}

resource "aws_eks_node_group" "main" {
  cluster_name    = aws_eks_cluster.main.name
  node_group_name = "gateway"
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.private[*].id
  instance_types  = [var.node_instance_type]

  scaling_config {
    desired_size = 2
    min_size     = 2
    max_size     = 3
  }

  depends_on = [aws_iam_role_policy_attachment.node, aws_route_table_association.private]
}

resource "aws_eks_addon" "pod_identity" {
  cluster_name = aws_eks_cluster.main.name
  addon_name   = "eks-pod-identity-agent"
  depends_on   = [aws_eks_node_group.main]
}

# Gives the gateway's pods the role in iam.tf, through the service account that the Helm chart creates.
resource "aws_eks_pod_identity_association" "gateway" {
  cluster_name    = aws_eks_cluster.main.name
  namespace       = local.namespace
  service_account = local.service_account_name
  role_arn        = aws_iam_role.gateway.arn
  depends_on      = [aws_eks_addon.pod_identity]
}
