/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 */

// Package version exposes build-time version metadata for the gateway-controller.
// The variables in this package are populated via -ldflags at build time.
package version

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)
