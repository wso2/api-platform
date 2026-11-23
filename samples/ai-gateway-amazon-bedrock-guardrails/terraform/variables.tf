variable "region" {
  description = "AWS Region for the guardrails and the model. Amazon Bedrock and Amazon Bedrock Guardrails must be available in it."
  type        = string
}

variable "model_id" {
  description = "Model ID or cross-Region inference profile ID that the gateway may invoke, for example us.amazon.nova-2-lite-v1:0."
  type        = string
}

variable "model_regions" {
  description = "Regions that the inference profile routes to. For us.* profiles called from us-east-2, these are us-east-1, us-east-2, and us-west-2."
  type        = list(string)
  default     = ["us-east-1", "us-east-2", "us-west-2"]
}

variable "name_prefix" {
  description = "Prefix for resource names."
  type        = string
  default     = "wso2-ai-gateway"
}

variable "eks_public_access_cidrs" {
  description = "CIDR ranges allowed to reach the Kubernetes API endpoint, for example your own IP address as a /32."
  type        = list(string)

  validation {
    condition     = length(var.eks_public_access_cidrs) > 0 && !contains(var.eks_public_access_cidrs, "0.0.0.0/0")
    error_message = "Set at least one CIDR range, and don't use 0.0.0.0/0."
  }
}

variable "eks_version" {
  description = "Kubernetes version for the Amazon EKS cluster."
  type        = string
  default     = "1.35"
}

variable "node_instance_type" {
  description = "Instance type for the EKS managed node group."
  type        = string
  default     = "m5.xlarge"
}

variable "vpc_cidr" {
  description = "CIDR range for the VPC."
  type        = string
  default     = "10.20.0.0/16"
}

variable "db_instance_class" {
  description = "Instance class for the gateway's Amazon RDS for PostgreSQL database."
  type        = string
  default     = "db.t4g.small"
}

variable "db_multi_az" {
  description = "Run the database across Availability Zones. Turn this on for production."
  type        = bool
  default     = false
}

variable "log_retention_days" {
  description = "Days to keep the EKS control plane, database, and VPC flow logs in CloudWatch Logs."
  type        = number
  default     = 30
}
