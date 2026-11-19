locals {
  harmful_content_filters = ["HATE", "INSULTS", "SEXUAL", "VIOLENCE", "MISCONDUCT"]
}

# Baseline guardrail. The gateway attaches it to the shared Amazon Bedrock provider,
# so it runs on every request from every application.
resource "aws_bedrock_guardrail" "baseline" {
  name                      = "org-baseline"
  description               = "Organization-wide baseline for every generative AI application"
  blocked_input_messaging   = "Blocked by the organization content policy."
  blocked_outputs_messaging = "Blocked by the organization content policy."

  content_policy_config {
    filters_config {
      type            = "PROMPT_ATTACK"
      input_strength  = "HIGH"
      output_strength = "NONE"
    }

    dynamic "filters_config" {
      for_each = local.harmful_content_filters
      content {
        type            = filters_config.value
        input_strength  = "HIGH"
        output_strength = "HIGH"
      }
    }
  }

  sensitive_information_policy_config {
    pii_entities_config {
      type   = "PHONE"
      action = "ANONYMIZE"
    }
    pii_entities_config {
      type   = "CREDIT_DEBIT_CARD_NUMBER"
      action = "ANONYMIZE"
    }
  }
}

# App-specific guardrail. The gateway attaches it to the support assistant's proxy only.
resource "aws_bedrock_guardrail" "support_assistant" {
  name                      = "support-assistant"
  description               = "Extra rules for the customer-facing support assistant"
  blocked_input_messaging   = "The support assistant can't help with that."
  blocked_outputs_messaging = "The support assistant can't help with that."

  topic_policy_config {
    topics_config {
      name       = "LegalAdvice"
      type       = "DENY"
      definition = "Advice about legal rights, lawsuits, or legal action against the company or anyone else."
      examples   = ["Can I sue you if my refund is late?"]
    }
  }
}

# Numbered versions: editing a guardrail doesn't change what the gateway enforces
# until you publish a new version and point the gateway at it.
resource "aws_bedrock_guardrail_version" "baseline" {
  guardrail_arn = aws_bedrock_guardrail.baseline.guardrail_arn
  description   = "Baseline v1"
}

resource "aws_bedrock_guardrail_version" "support_assistant" {
  guardrail_arn = aws_bedrock_guardrail.support_assistant.guardrail_arn
  description   = "Support assistant v1"
}
