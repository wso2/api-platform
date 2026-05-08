# Design and govern

This guide explains how to use API Designer for two core activities:

- Design your OpenAPI specification with AI-assisted workflows and form-based editing in the designer.
- Analyze your specification with governance reports and fix findings.

## Design

Use the `api-design` skill to create your API specification, and use the Design view to refine and improve it.

### AI-first design workflow

API Designer includes the `api-design` Chat skill. Use it to design and refine your API faster.

You can use the skill to:

- Generate an initial OpenAPI draft from requirements.
- Improve operation descriptions, examples, and API behavior details.
- Evolve an existing spec with guided prompts instead of manual rewrites.

Example prompts:

- "Design an OpenAPI spec for an Orders API with CRUD operations."
- "Improve descriptions and examples for all POST operations."
- "Add pagination parameters for all list endpoints."

### What you do in Design view

- Browse paths, operations, request bodies, responses, and components.
- Make targeted edits to operation details, parameters, and schema structure.
- Keep updates in the same OpenAPI file so the spec stays the source of truth.

## Analyze

Use report cards to evaluate API quality and readiness.

### What reports show

API Designer runs bundled Spectral-based rulesets and shows:

- Scores and summary metrics.
- Violations grouped by rule, severity, and affected spec area.
- Category-level breakdown so you can prioritize fixes.

### Built-in rulesets

| Ruleset | Focus |
|---------|-------|
| WSO2 REST API AI Readiness Guidelines | Agent and automation readiness |
| WSO2 REST API Design Guidelines | REST design quality and consistency |
| OWASP API Security Top 10 | API security best-practice checks |

### AI-assisted analysis and fixes

For AI readiness, you can combine rule-based findings with AI-assisted explanations to understand impact.

Recommended fix order:

1. Resolve quick, safe fixes first.
2. Review possible breaking changes (for example path renames) before applying.
3. Keep environment-specific security values for manual confirmation.

You can use Copilot Chat and the `api-design` skill together to apply fixes and then re-check the report.

## Related topics

- [Getting started](./getting-started.md)
- [End-to-end tutorial](./end-to-end-tutorial.md)
