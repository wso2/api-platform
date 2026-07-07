# End-to-End Workflow

This guide shows the full lifecycle with the `ap` CLI: create an API **project**, **deploy** it to a gateway, then **build** a portal artifact and **publish** it — either to the **Developer Portal** or to an **AI Workspace**.

A single API project is the source of truth for all destinations:

- `runtime.yaml` → deployed to the **gateway** (where the API is served).
- `metadata.yaml` + `definition.yaml` → used to generate the default Developer Portal artifact.
- `metadata.yaml` + `runtime.yaml` + `definition.yaml` → bundled into the **AI Workspace** artifact.

## Flow

```mermaid
flowchart LR
    A["<b>0 · Set up once</b><br/>connect + select<br/>gateway · devportal · ai-workspace"] --> B["<b>1 · Create</b><br/>ap project init"]
    B --> C["<b>Author</b><br/>metadata.yaml · runtime.yaml · definition.yaml"]
    C --> D["<b>2 · Deploy</b><br/>ap gateway apply -f runtime.yaml"]
    D --> E{"Publish to?"}

    subgraph DP["Developer Portal"]
        direction LR
        F["<b>3a · Generate</b><br/>ap devportal gen"] --> G["<b>4a · Build</b><br/>ap devportal build"] --> H["<b>5a · Apply</b><br/>ap devportal apply"]
    end

    subgraph AW["AI Workspace"]
        direction LR
        I["<b>3b · Build</b><br/>ap ai-workspace build"] --> J["<b>4b · Apply</b><br/>ap ai-workspace apply"]
    end

    E -->|REST API| F
    E -->|LLM proxy/ provider / MCP proxy| I

    classDef dp fill:#e8f0fe,stroke:#4285f4,color:#1a3d7c;
    classDef aiws fill:#e6f4ea,stroke:#34a853,color:#1e4620;
    class F,G,H dp;
    class I,J aiws;
```

## Steps

### 0. Configure connections (one-time)

Register and select the servers the CLI talks to. Each connection lives under the active platform.

```shell
ap platform add --display-name <name> --control-plane <url>   # optional; if you use platforms
ap gateway   add --display-name <gw>   --server <gw-url>      && ap gateway   use --display-name <gw>
ap devportal add --display-name <dp>   --server <dp-url> --auth api-key  && ap devportal use --display-name <dp>
ap ai-workspace     add --display-name <aiws> --server <aiws-url> --auth api-key && ap ai-workspace use --display-name <aiws>
```

Commands resolve the **active** gateway / devportal / ai-workspace of the active platform unless you pass `--display-name` (and `--platform`). See [Gateway](gateway/README.md), [DevPortal](devportal/README.md), and [AI-Workspace](ai-workspace/README.md) references.

### 1. Create the project

```shell
ap project init --display-name echo-api --type rest --version v2.0 --context /ping
cd echo-api
```

Scaffolds `metadata.yaml`, `runtime.yaml`, `definition.yaml`, `docs/`, `tests/`, and `.api-platform/config.yaml`. See the [API Project reference](apiproject/README.md).

### 2. Deploy to the gateway

Edit `runtime.yaml` (real upstream, policies, operations), then deploy:

```shell
ap gateway apply -f runtime.yaml
```

The response includes the gateway-assigned **API ID**. Re-read it any time with
`ap gateway rest-api get --display-name "<Display Name>" --version <version>`.

### 3+. Build the portal artifact and publish

Pick the destination for the deployed API.

#### Developer Portal

```shell
# Set spec.referenceID in metadata.yaml to the gateway API ID from step 2, then:
ap devportal gen                                                    # generate ./devportal (devportal.yaml, definition, docs, content)
ap devportal build                                                  # package ./devportal → build/devportal.zip
ap devportal apply -f build/devportal.zip --org <org-id>            # kind read from the zip's devportal.yaml -> RestApi
```

`gen` generates the devportal artifact source and registers it in the project config; edit `./devportal/devportal.yaml` to customize before `build`. `build` only packages the generated folder (run `gen` first). `apply` routes by the artifact **kind** (a `.zip` → REST API; a YAML CR → its kind). Follow-ups once published: `ap devportal apply -f sub_plan.yaml --org <org-id>`, `ap devportal api-key generate`, `ap devportal subscription create`.

#### AI Workspace

```shell
ap ai-workspace build                                    # validate the project's artifact
ap ai-workspace apply --project-id <project-id>           # generate the payload and create or update the artifact
# (--project-id is required for LlmProxy/Mcp kinds, not for LlmProvider)
# apply creates or updates automatically: it looks the artifact up by metadata.name and PUTs when it exists, else POSTs
# the endpoint is chosen by the artifact kind; the organization comes from the auth token — no --org flag
```

`ap ai-workspace build` reads the ai-workspace entry in `.api-platform/config.yaml` and **validates** the artifact (files present, metadata/runtime kinds align, name matches). `ap ai-workspace apply` runs the same validation, then generates the payload (folding the OpenAPI spec from `definition.yaml` into it) and **creates or updates** the artifact on the server — like `ap gateway apply`, it decides which from `metadata.name` (no separate `edit` command).

## Notes

- `ap devportal gen`, `ap devportal build`, and `ap ai-workspace build`/`apply` all operate on an API project (they require `.api-platform/config.yaml`).
- Developer Portal is two stages: `gen` **generates** the editable artifact source under `./devportal`, then `build` **packages** it into `build/devportal.zip`.
- Add `--insecure` to any portal/gateway command when talking to a local or self-signed HTTPS endpoint.
