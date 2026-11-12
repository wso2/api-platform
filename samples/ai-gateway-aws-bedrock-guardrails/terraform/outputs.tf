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
