output "region" {
  value = var.region
}

output "model_id" {
  value = var.model_id
}

output "gateway_role_arn" {
  value = aws_iam_role.gateway.arn
}

output "baseline_guardrail_id" {
  value = aws_bedrock_guardrail.baseline.guardrail_id
}

output "baseline_guardrail_version" {
  value = aws_bedrock_guardrail_version.baseline.version
}

output "support_guardrail_id" {
  value = aws_bedrock_guardrail.support_assistant.guardrail_id
}

output "support_guardrail_version" {
  value = aws_bedrock_guardrail_version.support_assistant.version
}

output "baseline_guardrail_arn" {
  value = aws_bedrock_guardrail.baseline.guardrail_arn
}

output "support_guardrail_arn" {
  value = aws_bedrock_guardrail.support_assistant.guardrail_arn
}

output "dashboard_url" {
  description = "Amazon CloudWatch dashboard for the guardrails, or null when create_dashboard is false."
  value = var.create_dashboard ? join("", [
    "https://${var.region}.console.aws.amazon.com/cloudwatch/home?region=${var.region}",
    "#dashboards/dashboard/${aws_cloudwatch_dashboard.guardrails[0].dashboard_name}",
  ]) : null
}

output "cluster_name" {
  value = aws_eks_cluster.main.name
}

output "namespace" {
  value = local.namespace
}

output "service_account_name" {
  value = local.service_account_name
}

output "ecr_registry" {
  value = split("/", aws_ecr_repository.gateway["controller"].repository_url)[0]
}

output "image_repository" {
  description = "The repository prefix to pass to the ap CLI."
  value       = "${split("/", aws_ecr_repository.gateway["controller"].repository_url)[0]}/${var.name_prefix}"
}

output "controller_image_repo" {
  value = aws_ecr_repository.gateway["controller"].repository_url
}

output "runtime_image_repo" {
  value = aws_ecr_repository.gateway["runtime"].repository_url
}

output "db_host" {
  value = aws_db_instance.main.address
}

output "db_name" {
  value = aws_db_instance.main.db_name
}

output "db_user" {
  value = aws_db_instance.main.username
}

output "db_secret_arn" {
  value = aws_db_instance.main.master_user_secret[0].secret_arn
}
