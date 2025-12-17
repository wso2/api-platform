/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package utils

import "github.com/spf13/cobra"

const (
	FlagName        = "name"
	FlagServer      = "server"
	FlagToken       = "token"
	FlagEnvToken    = "token-env"
	FlagUsername    = "username"
	FlagPassword    = "password"
	FlagPasswordEnv = "password-env"
	FlagInsecure    = "insecure"
	FlagOutput      = "output"
	FlagFile        = "file"
	FlagFormat      = "format"
	FlagVersion     = "version"
	FlagID          = "id"
)

var shortFlags = map[string]string{
	FlagName:    "n",
	FlagServer:  "s",
	FlagToken:   "t",
	FlagOutput:  "o",
	FlagFile:    "f",
	FlagVersion: "v",
}

func GetShortFlags() []string {
	values := make([]string, 0, len(shortFlags))
	for _, v := range shortFlags {
		values = append(values, v)
	}
	return values
}

func AddStringFlag(cmd *cobra.Command, flagName string, p *string, defaultValue, usage string) {
	if short, hasShort := shortFlags[flagName]; hasShort {
		cmd.Flags().StringVarP(p, flagName, short, defaultValue, usage)
	} else {
		cmd.Flags().StringVar(p, flagName, defaultValue, usage)
	}
}

func AddBoolFlag(cmd *cobra.Command, flagName string, p *bool, defaultValue bool, usage string) {
	if short, hasShort := shortFlags[flagName]; hasShort {
		cmd.Flags().BoolVarP(p, flagName, short, defaultValue, usage)
	} else {
		cmd.Flags().BoolVar(p, flagName, defaultValue, usage)
	}
}
