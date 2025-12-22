# WSO2 API Platform CLI (AP)

`ap` is a command-line tool for managing and interacting with the WSO2 API Platform.

## Quick Start

```bash
# Build the CLI
cd cli/src
make build # Build 
make build-all # Build all OS binaries
make build-skip-tests # Build without tests
make test # Run tests
make clean # Clean build artifacts
```

## CLI Project Structure
```
cli/
├── README.md             # This file (single source of truth)
└── src/ 
    ├── cmd/              # Command definitions
    │   ├── root.go       # Root command implementation
    │   └── gateway/      # Gateway subcommands
    ├── internal/         # Internal packages
    ├── utils/            # Shared utilities
    └── main.go           # Entry point
```

In the cmd directory, each subcommand has its own folder, and the root.go file within that folder contains the implementation of the corresponding subcommand.

All command verbs are implemented as separate Go files, each serving as the base implementation for that specific command.

For detailed command documentation, see [`docs/cli/reference.md`](../docs/cli/reference.md) in this repository.
