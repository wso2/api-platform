# Expose a REST API as MCP Tools via WSO2 AI Gateway

This sample takes a plain REST service that knows nothing about AI agents and
puts it in front of one as callable MCP tools, with the WSO2 AI Gateway
handling authentication, rate limiting and observability on the traffic.

The backend is `samples/sample-service`, unchanged. No new backend code.
`setup.sh` builds it from `../sample-service`, so run this sample from inside
a clone of the repository rather than copying the folder out on its own.

Everything runs locally in Docker. No cloud account and no API keys are needed.

## The problem

REST APIs answer specific URLs. An AI agent meets a service it has never seen
and needs to *ask* what that service can do before it can use it. The Model
Context Protocol (MCP) is the convention for that conversation: the agent asks
for a list of tools, then calls one.

A plain REST service has no such list to offer. Something has to translate.

## How this sample does it

```
  client.py
     |  MCP
     v
  WSO2 AI Gateway  ......  auth, rate limiting, observability
     |  MCP
     v
  Generated MCP server  ..  built by arazzo-mcp-gen from the specs in arazzo/
     |  HTTP
     v
  sample-service  ........  the echo backend, unchanged
```

The translation is done by [`arazzo-mcp-gen`](https://github.com/wso2/arazzo-mcp-generator),
a WSO2 CLI tool. You give it two specs and it emits a Dockerised MCP server:

- **An OpenAPI spec** describing the REST API's operations
- **An Arazzo spec** describing workflows over those operations

**Each Arazzo workflow becomes one MCP tool.** That is the key idea, and it is
why a tool here can do more than one REST call.

The gateway then fronts that generated server with an MCP proxy. It never needs
to know about REST, OpenAPI or Arazzo. It governs a server that already speaks
MCP.

## Which endpoints became which tools

| MCP tool | Arazzo workflow | REST endpoints it calls |
|---|---|---|
| `echo_message` | `echoMessage`: send a message, return what the service received | `POST /` |
| `echo_and_verify` | `echoAndVerify`: send a message, then read it back to confirm it arrived | `POST /` then `GET /captured-request` |

Note the renaming: `arazzo-mcp-gen` converts each camelCase `workflowId` into a
snake_case tool name. The workflow is `echoMessage`; the tool an agent calls is
`echo_message`.

`echo_and_verify` is the interesting row. It is **one tool made of two API calls**.
The agent asks once; the workflow handles the sequencing. A mapping that turned
each endpoint into its own tool could not express that.

---

## Prerequisites

Make sure the following are installed before running the sample:

| Tool | Notes |
|---|---|
| Docker with Compose plugin | Version 24 or later. Docker must be running. `setup.sh` checks and stops if it is not. |
| Python 3.10 or later | Required by the `mcp` SDK that `client.py` uses. |
| `curl` and `unzip` | Used by `setup.sh` to download the gateway and the generator. |
| A POSIX shell | The scripts are `sh`. On Windows, run them under WSL2 or Git Bash. |

## Getting started

```bash
./setup.sh
```

The first run takes a few minutes while Docker pulls and builds the images.
Later runs are faster.

`setup.sh` does the following, in order:

1. Downloads `arazzo-mcp-gen` into `./bin`
2. Builds and starts `sample-service` on a Docker network
3. Validates `arazzo/` and generates the MCP server: `mcp_server.py`, a
   `Dockerfile`, and a built image
4. Runs the generated MCP server
5. Downloads and starts the WSO2 AI Gateway
6. Registers `mcp.yaml`, which points the gateway's MCP proxy at the generated
   server

Then check every layer of the chain:

```bash
./test.sh
```

Then run the client. It needs two Python packages, so install them into a
virtual environment first:

**macOS, Linux, WSL2 or Git Bash**

```bash
python3 -m venv .venv
source .venv/bin/activate
python3 -m pip install -r requirements.txt
python3 client.py
```

**Windows PowerShell**

```powershell
py -m venv .venv
.venv\Scripts\Activate.ps1
py -m pip install -r requirements.txt
py client.py
```

You should see the gateway hand back two tools, then one of them being called:

```
Connected to: Sample Service Echo Workflows 4.0.0

--- Tools the gateway is exposing ---
  echo_message
      Send a message to the echo service and return what it received.
      takes: message
  echo_and_verify
      Send a message, then read it back from the service to confirm it arrived.
      takes: message

--- Calling echo_and_verify ---
  message = "hello from client.py"
```

Neither tool was written by hand. Both came from the workflows in
`arazzo/echo-workflow.yaml`.

That is the sample working end to end. From here you can
[add a tool](#adding-a-tool), or run `./teardown.sh` to stop everything.

## Configuration

These ports must be free before you start. `setup.sh` creates `.env` from
`.env.example` on first run. Edit `.env` and re-run `setup.sh` to change any
of them:

| Variable | Default | What it is |
|---|---|---|
| `TRAFFIC_PORT` | `8443` | Gateway HTTPS port, where MCP traffic goes in |
| `MGMT_PORT` | `9090` | Gateway management API, used to register `mcp.yaml` |
| `HEALTH_PORT` | `9094` | Gateway health endpoint, polled during startup |
| `MCP_PORT` | `5050` | Host port the generated MCP server is published on |
| `MAX_RETRIES` | `45` | How long to wait for the gateway, in 2-second attempts |

The backend port `8090` is fixed in `setup.sh` rather than configurable here.

## What client.py shows

It does exactly what an AI agent does: connects, asks `tools/list`, picks a
tool, calls it.

```bash
python3 client.py --list                # just the tool list
python3 client.py --tool echo_message   # call a specific tool
```

## What test.sh proves

The tests walk the chain one layer at a time, so a failure points at the layer
that broke:

1. `sample-service` answers HTTP, and **does not** answer `tools/list`. It is
   plain REST
2. The generated MCP server exposes both workflows as tools
3. The same tools are reachable through the gateway
4. Calling a tool through the gateway reaches `sample-service`, confirmed by
   asking the backend what request it last received

Step 1 matters: it establishes that nothing in the backend changed.

## Endpoints

| What | Where |
|---|---|
| Gateway MCP endpoint | `https://localhost:8443/sample-service/mcp` |
| Generated MCP server | `http://localhost:5050/mcp` |
| REST backend | `http://localhost:8090` |
| Gateway management API | `http://localhost:9090` |
| Gateway health | `http://localhost:9094/health` |

## File structure

| File / folder | Purpose |
|---|---|
| `arazzo/sample-service-openapi.yaml` | What the REST API can do |
| `arazzo/echo-workflow.yaml` | Workflows over those operations, one MCP tool each |
| `mcp.yaml` | The gateway's MCP proxy config, registered by `setup.sh` |
| `client.py` | An MCP client, the agent's side of the conversation |
| `requirements.txt` | Python dependencies for `client.py` (`mcp`, `httpx`) |
| `.env.example` | Template for ports and credentials, copied to `.env` on first run |
| `setup.sh` | One-command setup: downloads, builds, starts and registers everything |
| `test.sh` | Checks each layer of the chain in turn |
| `teardown.sh` | Stops the containers and removes the sample network |
| `bin/` | Created by `setup.sh`: the downloaded `arazzo-mcp-gen` binary |
| `generated/` | Created by `setup.sh`: the generated MCP server, worth reading |

## Adding a tool

Add a workflow to `arazzo/echo-workflow.yaml`, then re-run `setup.sh`. Write the
`summary` and `description` as if explaining the tool to a colleague. The model
uses them to decide when to call it.

## Troubleshooting

| Symptom | Likely cause and fix |
|---|---|
| `docker is required but not installed` / `Docker is not running` | Start your Docker runtime (Docker Desktop, Rancher Desktop or colima) and re-run `./setup.sh`. |
| `port is already allocated` | Something else holds the port. `setup.sh` names which one. Free it, or if it appears in [Configuration](#configuration), change it in `.env` and re-run. Find the holder with `docker ps \| grep <port>` or `lsof -nP -iTCP:<port> -sTCP:LISTEN`. |
| Port `5000` taken by a container you don't recognise | `arazzo-mcp-gen` prints `docker run -p 5000:5000 …` as a suggestion after generating. Running it leaves a randomly-named container holding the port, which `teardown.sh` cannot clean up by name. Remove it with `docker rm -f <name>`. |
| macOS refuses to run `arazzo-mcp-gen` | The released binaries are unsigned. `setup.sh` clears the quarantine flag; if Gatekeeper still blocks it, allow it under System Settings → Privacy & Security. |
| `Gateway did not become healthy` | The gateway needs longer on a slow machine. Raise `MAX_RETRIES` in `.env`, or inspect `cd wso2apip-ai-gateway-1.2.0 && docker compose logs`. |
| `Failed to register the MCP proxy` | Usually a basic auth mismatch. `setup.sh` provisions the gateway admin credential (defaults to `admin`/`admin`); if you set `ADMIN_USERNAME`/`ADMIN_PASSWORD`, use the same values in `.env`. |
| `No tools returned` from `client.py` | The generated MCP server is not answering. Check `docker logs rest-to-mcp-server`. |
| Route returns a non-200 after setup | The proxy may still be propagating. Wait a few seconds and run `./test.sh`. |
| `error: externally-managed-environment` from `pip` | Your Python blocks installs outside a virtual environment (PEP 668, common on macOS Homebrew and Debian/Ubuntu). Create one as shown in [Getting started](#getting-started) rather than passing `--break-system-packages`. |
| `Could not import a required package` from `client.py` | The virtual environment isn't active, or the install failed. Re-activate with `. .venv/bin/activate` and re-run the install. |
| Anything behaving oddly after repeated runs | Run `./teardown.sh` and start again from a clean slate. |

## Tearing down

```bash
./teardown.sh
```

This stops the containers and removes the sample network. It prints the command
to also delete the downloaded distribution, `bin/` and `generated/` if you want
the folder back to its original state.

## Notes

- The gateway uses a self-signed certificate, so `client.py` and the `curl`
  commands skip verification. Don't carry that into anything real.
- `mcp.yaml` has a commented-out policy block. Uncomment it to restrict which
  tools callers may invoke.
