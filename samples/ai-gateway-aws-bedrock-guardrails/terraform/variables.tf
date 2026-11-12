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
  description = "Prefix for the IAM role name."
  type        = string
  default     = "wso2-ai-gateway"
}
