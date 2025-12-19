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

package image

import (
	"github.com/spf13/cobra"
)

const (
	ImageCmdLiteral = "image"
	ImageCmdExample = `# Build gateway image
apipctl gateway image build --image-tag v0.2.0-policy1

# Build with custom manifest file
apipctl gateway image build --image-tag v0.2.0 -f custom-manifest.yaml

# Build in offline mode
apipctl gateway image build --image-tag v0.2.0 --offline`
)

// ImageCmd represents the image command
var ImageCmd = &cobra.Command{
	Use:     ImageCmdLiteral,
	Short:   "Manage gateway Docker images",
	Long:    "This command allows you to build and manage WSO2 API Platform Gateway Docker images with policies.",
	Example: ImageCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	// Register subcommands
	ImageCmd.AddCommand(buildCmd)
}
