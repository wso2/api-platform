# Prompt Compressor Policy

`prompt-compressor` is a Python executor policy for the WSO2 AI Gateway that
compresses prompt text before the upstream LLM call.

## Install From GitHub

From this monorepo, the package can be installed directly with:

```bash
pip install "git+https://github.com/<org>/<repo>.git@<tag-or-commit>#subdirectory=gateway/sample-policies/prompt-compressor"
```

Example:

```bash
pip install "git+https://github.com/wso2/api-platform.git@v0.1.0#subdirectory=gateway/sample-policies/prompt-compressor"
```

## Use In `build.yaml`

The gateway builder can consume the same Git reference through `pipPackage`:

```yaml
version: v1
policies:
  - name: prompt-compressor
    pipPackage: "git+https://github.com/<org>/<repo>.git@<tag-or-commit>#subdirectory=gateway/sample-policies/prompt-compressor"
```

Keep the API configuration policy version as the major-only version:

```yaml
policies:
  - name: prompt-compressor
    version: v0
```

The package metadata version is `0.1.0`, while the gateway policy definition
version remains `v0.1.0`.
