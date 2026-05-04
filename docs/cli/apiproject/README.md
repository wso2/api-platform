# API Project CLI Reference

This guide covers the API project commands currently implemented under `cli/src/cmd/apiproject`.

## Commands

### `ap apiproject init`

Initializes a new API project in the current working directory.

```shell
ap apiproject init --display-name <name> --type <rest|soap> --version <version> --context <context> [--no-interactive]
```

Examples:

```shell
ap apiproject init --display-name foo-api --type rest --version 1.0 --context /foo
ap apiproject init
```

Behavior:

- If `--no-interactive` is not used, the command prompts for any missing values.
- The project directory is created under the current directory using the `display-name` value.
- The directory name cannot be empty and cannot contain path separators.
- API type validation accepts `rest` and `soap`, but project scaffolding is currently implemented only for `rest`.

## Generated Project Structure

Running `ap apiproject init --display-name FooAPI --type rest --version 1.0.0 --context /petstore` creates:

```text
FooAPI/
├── .api-platform/
│   └── config.yaml
├── api.yaml
├── gateway.yaml
├── definition.yaml
├── docs/
└── tests/
```

## Generated Files

### `.api-platform/config.yaml`

Created with default project file paths:

```yaml
version: 1.0.0

filePaths:
  deploymentArtifact: ./gateway.yaml
  apiMetadata: ./api.yaml
  apiDefinition: ./definition.yaml
  docs: ./docs
  tests: ./tests

governanceRulesets: []

autoSync:
  gatewayArtifactFromDefinition: true
```

### `api.yaml`

- Contains API metadata for management-plane usage.
- The resource name is derived from the display name and version.

### `gateway.yaml`

- Contains a default `RestApi` gateway deployment artifact.
- Includes:
  - `displayName`
  - `version`
  - `context`
  - a sample backend URL
  - default `GET`, `POST`, `PUT`, and `DELETE` operations on `/*`

### `definition.yaml`

- Contains a starter OpenAPI 3.0.3 document.
- Includes `GET`, `POST`, `PUT`, `DELETE`, and `OPTIONS` operations on `/*`.

## Notes

- If the target project directory already exists, the command fails.
- Resource names are normalized to a safe lower-case form using the display name and version.
- SOAP project scaffolding is not yet implemented, even though `soap` is accepted during initial validation.

## Related Commands

- `ap devportal build`
