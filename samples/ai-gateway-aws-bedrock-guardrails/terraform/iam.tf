data "aws_caller_identity" "current" {}

locals {
  # A cross-Region inference profile ID such as us.amazon.nova-2-lite-v1:0 routes to the
  # foundation model amazon.nova-2-lite-v1:0 in several Regions. IAM must allow the profile
  # and the model in every Region the profile can route to. A global.* profile also needs the
  # Region-agnostic model ARN.
  is_inference_profile  = can(regex("^(us|eu|apac|jp|au|in|global)\\.", var.model_id))
  foundation_model_id   = replace(var.model_id, "/^(us|eu|apac|jp|au|in|global)\\./", "")
  inference_profile_arn = "arn:aws:bedrock:${var.region}:${data.aws_caller_identity.current.account_id}:inference-profile/${var.model_id}"
  model_arns = local.is_inference_profile ? concat(
    [for r in var.model_regions : "arn:aws:bedrock:${r}::foundation-model/${local.foundation_model_id}"],
    startswith(var.model_id, "global.") ? ["arn:aws:bedrock:::foundation-model/${local.foundation_model_id}"] : [],
  ) : ["arn:aws:bedrock:${var.region}::foundation-model/${var.model_id}"]
}

# Least-privilege role for the gateway. EKS Pod Identity gives it to the gateway's service account, and
# every Amazon Bedrock call runs under its temporary credentials.
resource "aws_iam_role" "gateway" {
  name = "${var.name_prefix}-bedrock"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "pods.eks.amazonaws.com" }
      Action    = ["sts:AssumeRole", "sts:TagSession"]
      Condition = {
        StringEquals = { "aws:SourceAccount" = data.aws_caller_identity.current.account_id }
      }
    }]
  })
}

resource "aws_iam_role_policy" "gateway_bedrock" {
  name = "bedrock-converse-and-guardrails"
  role = aws_iam_role.gateway.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = concat(
      [{
        Sid      = "ConverseOneModel"
        Effect   = "Allow"
        Action   = "bedrock:InvokeModel"
        Resource = concat(local.is_inference_profile ? [local.inference_profile_arn] : [], local.model_arns)
      }],
      local.is_inference_profile ? [{
        Sid      = "ReadInferenceProfile"
        Effect   = "Allow"
        Action   = "bedrock:GetInferenceProfile"
        Resource = local.inference_profile_arn
      }] : [],
      [{
        Sid    = "ApplyApprovedGuardrails"
        Effect = "Allow"
        Action = "bedrock:ApplyGuardrail"
        Resource = [
          aws_bedrock_guardrail.baseline.guardrail_arn,
          aws_bedrock_guardrail.support_assistant.guardrail_arn,
        ]
      }]
    )
  })
}
