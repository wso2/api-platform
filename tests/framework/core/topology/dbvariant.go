/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 */

package topology

import (
	"fmt"

	"github.com/wso2/api-platform/tests/framework/core/components"
)

// DBVariant identifies a supported database engine and its server image.
type DBVariant struct {
	Engine components.DBType
	Image  components.ImageRef
}

// SupportedDBVariants contains the database variants accepted by suite files.
var SupportedDBVariants = map[string]DBVariant{
	"sqlite": {
		Engine: components.SQLite,
	},
	"postgres:16-alpine": {
		Engine: components.Postgres,
		Image:  components.ImageRef{Ref: "postgres:16-alpine"},
	},
	"postgres:17-alpine": {
		Engine: components.Postgres,
		Image:  components.ImageRef{Ref: "postgres:17-alpine"},
	},
	"sqlserver:2022-latest": {
		Engine: components.SQLServer,
		Image: components.ImageRef{
			Ref:    "mcr.microsoft.com/mssql/server:2022-latest",
			ByArch: map[string]string{"arm64": "mcr.microsoft.com/azure-sql-edge:latest"},
		},
	},
}

// DefaultDBVariants maps engine shorthand to its default supported variant.
var DefaultDBVariants = map[components.DBType]string{
	components.SQLite:    "sqlite",
	components.Postgres:  "postgres:16-alpine",
	components.SQLServer: "sqlserver:2022-latest",
}

func resolveDBVariant(value string) (string, DBVariant, error) {
	if variant, ok := SupportedDBVariants[value]; ok {
		return value, variant, nil
	}
	if engine := components.DBType(value); engine.Valid() {
		key, ok := DefaultDBVariants[engine]
		if ok {
			return key, SupportedDBVariants[key], nil
		}
	}
	return "", DBVariant{}, fmt.Errorf("unsupported database variant %q", value)
}

func defaultVariant(engine components.DBType) (string, DBVariant, bool) {
	key, ok := DefaultDBVariants[engine]
	if !ok {
		return "", DBVariant{}, false
	}
	variant, ok := SupportedDBVariants[key]
	return key, variant, ok
}
