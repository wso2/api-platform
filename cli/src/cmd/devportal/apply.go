/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
package devportal

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	internaldevportal "github.com/wso2/api-platform/cli/internal/devportal"
	"github.com/wso2/api-platform/cli/utils"
	"gopkg.in/yaml.v3"
)

const (
	ApplyCmdLiteral = "apply"
	ApplyCmdExample = `# Apply an organization from a YAML CR file (kind: Organization)
ap devportal apply -f org.yaml

# Apply subscription plan(s) from a YAML CR file (kind: SubscriptionPolicy / SubscriptionPolicyList)
ap devportal apply -f sub_plan.yaml --org org_1

# Apply a REST API from a built artifact zip (devportal.yaml -> kind: RestApi)
ap devportal apply -f build/devportal.zip --org org_1

# Apply using a specific devportal without relying on the active devportal
ap devportal apply -f org.yaml --display-name my-portal --platform eu`
)

// DevPortal CR kinds that `apply` routes. The kind (read from the YAML CR or from
// the artifact zip's devportal.yaml) selects the create/update endpoint.
const (
	kindOrganization           = "Organization"
	kindSubscriptionPolicy     = "SubscriptionPolicy"
	kindSubscriptionPolicyList = "SubscriptionPolicyList"
	kindRestAPI                = "RestApi"
)

var (
	applyFilePath string
	applyOrgID    string
	applyName     string
	applyPlatform string
	applyInsecure bool
)

var applyCmd = &cobra.Command{
	Use:   ApplyCmdLiteral,
	Short: "Apply a resource to the DevPortal",
	Long: "Create or update a DevPortal resource from a single file. The file is either a YAML CR " +
		"(kind: Organization, SubscriptionPolicy, or SubscriptionPolicyList) or a built REST API artifact " +
		"zip (whose devportal.yaml declares kind: RestApi). The kind selects the target endpoint, and — for " +
		"kinds that support it (Organization, RestApi) — apply checks whether the resource already exists and " +
		"updates it (PUT) or creates it (POST) accordingly. --org is required for org-scoped kinds " +
		"(RestApi, SubscriptionPolicy/SubscriptionPolicyList).",
	Example: ApplyCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runApplyCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(applyCmd, utils.FlagFile, &applyFilePath, "", "Path to the resource file: a YAML CR file or a REST API .zip artifact")
	utils.AddStringFlag(applyCmd, utils.FlagOrgID, &applyOrgID, "", "Organization ID (required for RestApi and SubscriptionPolicy kinds)")
	utils.AddStringFlag(applyCmd, utils.FlagName, &applyName, "", "DevPortal display name")
	utils.AddStringFlag(applyCmd, utils.FlagPlatform, &applyPlatform, "", "Platform name")
	applyCmd.Flags().BoolVar(&applyInsecure, "insecure", false, "Skip TLS certificate verification")
	_ = applyCmd.MarkFlagRequired(utils.FlagFile)
}

// applyTarget describes how a resolved kind maps to its DevPortal endpoint: the
// multipart field the file is uploaded in, whether the endpoint is
// organization-scoped (so --org is required), whether the resource supports an
// existence check + update (create-or-update), and how to build the collection
// path.
type applyTarget struct {
	multipartField string
	orgScoped      bool
	// supportsUpdate is true for resources addressable by handle with a PUT
	// endpoint (Organization, RestApi): apply probes existence and PUTs an update
	// or POSTs a create. Subscription plans have no per-plan PUT — their publish
	// endpoint upserts — so they are always POSTed.
	supportsUpdate bool
	collection     func(orgID string) string
}

func runApplyCommand() error {
	filePath, err := resolveApplyFilePath(applyFilePath)
	if err != nil {
		return err
	}

	kind, handle, err := detectApplyResource(filePath)
	if err != nil {
		return err
	}

	target, err := resolveApplyTarget(kind)
	if err != nil {
		return err
	}

	orgID := strings.TrimSpace(applyOrgID)
	if target.orgScoped && orgID == "" {
		return fmt.Errorf("organization ID is required for kind %q (use --org)", kind)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	devPortal, resolvedPlatform, err := internaldevportal.ResolveDevPortal(cfg, applyName, applyPlatform)
	if err != nil {
		return err
	}

	client := internaldevportal.NewClientWithOptions(devPortal, applyInsecure)
	collection := target.collection(orgID)

	resp, action, err := applyResource(client, target, collection, handle, filePath, kind)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return utils.FormatHTTPError(fmt.Sprintf("%s %s", action, kind), resp, "DevPortal")
	}

	fmt.Println("Status: success")
	fmt.Printf("Message: %s %s successfully\n", kind, action)
	if handle != "" {
		fmt.Printf("ID: %s\n", handle)
	}
	fmt.Printf("DevPortal: %s (platform: %s)\n", devPortal.Name, resolvedPlatform)
	return internaldevportal.PrintJSONResponse(resp)
}

// applyResource creates or updates the resource. For kinds that support update
// it probes existence by handle (GET {collection}/{handle}) and PUTs an update
// or POSTs a create, mirroring `ap gateway apply`; other kinds are always POSTed
// to the collection. It returns the response and the past-tense action verb
// ("applied" for create, "updated" for update).
func applyResource(client *internaldevportal.Client, target applyTarget, collection, handle, filePath, kind string) (*http.Response, string, error) {
	if target.supportsUpdate {
		exists, err := resourceExists(client, collection+"/"+url.PathEscape(handle))
		if err != nil {
			return nil, "", internaldevportal.WrapRequestError(fmt.Sprintf("check %s existence", kind), err, applyInsecure)
		}
		if exists {
			resp, err := client.PutMultipartFile(collection+"/"+url.PathEscape(handle), target.multipartField, filePath)
			if err != nil {
				return nil, "", internaldevportal.WrapRequestError(fmt.Sprintf("update %s", kind), err, applyInsecure)
			}
			return resp, "updated", nil
		}
	}

	resp, err := client.PostMultipartFile(collection, target.multipartField, filePath)
	if err != nil {
		return nil, "", internaldevportal.WrapRequestError(fmt.Sprintf("apply %s", kind), err, applyInsecure)
	}
	return resp, "applied", nil
}

// resourceExists reports whether the resource at endpoint already exists. The
// DevPortal client returns the response for both 2xx and 404, so a 404 means
// "does not exist" and any 2xx means "exists"; other statuses surface as an error.
func resourceExists(client *internaldevportal.Client, endpoint string) (bool, error) {
	resp, err := client.Get(endpoint)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode != http.StatusNotFound, nil
}

// resolveApplyTarget maps a CR kind to its DevPortal endpoint.
func resolveApplyTarget(kind string) (applyTarget, error) {
	switch kind {
	case kindOrganization:
		// Organization management lives outside the org-scoped prefix and is
		// addressable by handle (metadata.name), so it supports create-or-update.
		return applyTarget{
			multipartField: "organization",
			orgScoped:      false,
			supportsUpdate: true,
			collection:     func(string) string { return "/organizations" },
		}, nil
	case kindSubscriptionPolicy, kindSubscriptionPolicyList:
		// Subscription plans have no per-plan PUT; the publish endpoint upserts
		// (and SubscriptionPolicyList is a bulk upload), so always POST.
		return applyTarget{
			multipartField: "subscriptionPolicy",
			orgScoped:      true,
			supportsUpdate: false,
			collection:     func(orgID string) string { return internaldevportal.OrgScopedPath(orgID, "subscription-policies") },
		}, nil
	case kindRestAPI:
		// APIs are addressable by handle (metadata.name) with a PUT endpoint.
		return applyTarget{
			multipartField: "artifact",
			orgScoped:      true,
			supportsUpdate: true,
			collection:     func(orgID string) string { return internaldevportal.OrgScopedPath(orgID, "apis") },
		}, nil
	default:
		return applyTarget{}, fmt.Errorf("unsupported kind %q (supported: %s, %s, %s, %s)",
			kind, kindOrganization, kindSubscriptionPolicy, kindSubscriptionPolicyList, kindRestAPI)
	}
}

// detectApplyResource resolves the CR kind and its handle (metadata.name) from
// the input file. A .zip is a built REST API artifact whose kind/handle are read
// from its devportal.yaml; a .yaml/.yml is a CR read directly. It guards the
// file-type ⇄ kind pairing so a REST API is supplied as a zip and CRs as YAML.
func detectApplyResource(filePath string) (kind, handle string, err error) {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".zip":
		kind, handle, err = resourceFromArtifactZip(filePath)
		if err != nil {
			return "", "", err
		}
		if kind != kindRestAPI {
			return "", "", fmt.Errorf("artifact zip declares unsupported kind %q; expected %s", kind, kindRestAPI)
		}
		if handle == "" {
			return "", "", fmt.Errorf("%s in artifact is missing metadata.name", archiveMetadataFileName)
		}
		return kind, handle, nil
	case ".yaml", ".yml":
		kind, handle, err = resourceFromYAMLCR(filePath)
		if err != nil {
			return "", "", err
		}
		if kind == kindRestAPI {
			return "", "", fmt.Errorf("a %s must be provided as a built .zip artifact, not a YAML file", kindRestAPI)
		}
		return kind, handle, nil
	default:
		return "", "", fmt.Errorf("unsupported file type %q: provide a .yaml CR file or a .zip artifact", filepath.Ext(filePath))
	}
}

// yamlCR is the minimal shape used to read a CR's kind/handle and validate it
// locally before upload (single-item kinds need metadata.name; a list needs items).
type yamlCR struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Items []struct {
		Metadata struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
	} `yaml:"items"`
}

// resourceFromYAMLCR reads and lightly validates a YAML CR, returning its kind
// and handle (metadata.name; empty for a SubscriptionPolicyList, which is bulk).
func resourceFromYAMLCR(filePath string) (kind, handle string, err error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read resource file: %w", err)
	}

	var cr yamlCR
	if err := yaml.Unmarshal(content, &cr); err != nil {
		return "", "", fmt.Errorf("invalid resource YAML: %w", err)
	}

	kind = strings.TrimSpace(cr.Kind)
	if kind == "" {
		return "", "", fmt.Errorf("'kind' field is required in the resource file")
	}

	switch kind {
	case kindSubscriptionPolicyList:
		if len(cr.Items) == 0 {
			return "", "", fmt.Errorf("invalid %s: items must contain at least one entry", kind)
		}
		for i, item := range cr.Items {
			if strings.TrimSpace(item.Metadata.Name) == "" {
				return "", "", fmt.Errorf("invalid %s: items[%d].metadata.name is required", kind, i)
			}
		}
		return kind, "", nil
	default:
		handle = strings.TrimSpace(cr.Metadata.Name)
		if handle == "" {
			return "", "", fmt.Errorf("invalid %s: metadata.name is required", kind)
		}
		return kind, handle, nil
	}
}

// resourceFromArtifactZip reads the kind and handle (metadata.name) declared in
// the artifact zip's devportal.yaml.
func resourceFromArtifactZip(zipPath string) (kind, handle string, err error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to open artifact zip %s: %w", zipPath, err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if filepath.Base(file.Name) != archiveMetadataFileName {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return "", "", fmt.Errorf("failed to open %s in artifact: %w", archiveMetadataFileName, err)
		}
		data, readErr := io.ReadAll(rc)
		_ = rc.Close()
		if readErr != nil {
			return "", "", fmt.Errorf("failed to read %s in artifact: %w", archiveMetadataFileName, readErr)
		}

		var doc struct {
			Kind     string `yaml:"kind"`
			Metadata struct {
				Name string `yaml:"name"`
			} `yaml:"metadata"`
		}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return "", "", fmt.Errorf("failed to parse %s in artifact: %w", archiveMetadataFileName, err)
		}
		kind = strings.TrimSpace(doc.Kind)
		if kind == "" {
			return "", "", fmt.Errorf("%s in artifact is missing the 'kind' field", archiveMetadataFileName)
		}
		return kind, strings.TrimSpace(doc.Metadata.Name), nil
	}

	return "", "", fmt.Errorf("artifact zip %s does not contain %s", zipPath, archiveMetadataFileName)
}

// resolveApplyFilePath validates that filePath points to an existing regular file.
func resolveApplyFilePath(filePath string) (string, error) {
	trimmed := strings.TrimSpace(filePath)
	if trimmed == "" {
		return "", fmt.Errorf("file path is required")
	}

	resolvedPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path: %w", err)
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", resolvedPath)
		}
		return "", fmt.Errorf("failed to inspect file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("file path must point to a file, got directory: %s", resolvedPath)
	}

	return resolvedPath, nil
}
