# Common Module

This module contains shared utilities, models, and helpers used across the API Platform services.

## Structure

- `logger/` - Common logging utilities
- `config/` - Configuration helpers
- `errors/` - Custom error types and error handling
- `models/` - Shared data models
- `utils/` - General utility functions
- `constants/` - Shared constants

## Usage

Import this module in your Go projects:

```go
import "github.com/wso2/api-platform/common/logger"
import "github.com/wso2/api-platform/common/models"
```

## Local Development

When developing locally, use `replace` directive in your `go.mod`:

```go
replace github.com/wso2/api-platform/common => ../common
```
