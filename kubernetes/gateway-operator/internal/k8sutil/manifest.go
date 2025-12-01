/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8sutil

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"

	yamlv2 "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ManifestApplier handles applying Kubernetes manifests from files
type ManifestApplier struct {
	client client.Client
	scheme *runtime.Scheme
}

// NewManifestApplier creates a new ManifestApplier
func NewManifestApplier(client client.Client, scheme *runtime.Scheme) *ManifestApplier {
	return &ManifestApplier{
		client: client,
		scheme: scheme,
	}
}

// ApplyManifestFile reads a YAML manifest file and applies all resources to the cluster
func (m *ManifestApplier) ApplyManifestFile(ctx context.Context, manifestPath string, namespace string, owner client.Object) error {
	log := log.FromContext(ctx)

	log.Info("Reading manifest file", "path", manifestPath)

	// Read the manifest file
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest file: %w", err)
	}

	// Parse and apply resources
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	appliedCount := 0
	errorCount := 0

	for {
		var rawObj runtime.RawExtension
		if err := decoder.Decode(&rawObj); err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Error(err, "Failed to decode resource, skipping")
			errorCount++
			continue
		}

		// Skip empty documents
		if len(rawObj.Raw) == 0 {
			continue
		}

		// Convert to unstructured object
		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal(rawObj.Raw, obj); err != nil {
			log.Error(err, "Failed to unmarshal resource, skipping")
			errorCount++
			continue
		}

		// Skip empty objects
		if obj.GetKind() == "" {
			continue
		}

		// Set namespace if not specified
		if obj.GetNamespace() == "" && namespace != "" {
			obj.SetNamespace(namespace)
		}

		// Apply the resource
		if err := m.applyResource(ctx, obj, owner); err != nil {
			log.Error(err, "Failed to apply resource",
				"kind", obj.GetKind(),
				"name", obj.GetName(),
				"namespace", obj.GetNamespace())
			errorCount++
			continue
		}

		log.Info("Applied resource",
			"kind", obj.GetKind(),
			"name", obj.GetName(),
			"namespace", obj.GetNamespace())
		appliedCount++
	}

	if errorCount > 0 {
		return fmt.Errorf("applied %d resources with %d errors", appliedCount, errorCount)
	}

	log.Info("Successfully applied all resources from manifest",
		"path", manifestPath,
		"count", appliedCount)

	return nil
}

// applyResource applies a single Kubernetes resource using server-side apply or create/update
func (m *ManifestApplier) applyResource(ctx context.Context, obj *unstructured.Unstructured, owner client.Object) error {
	log := log.FromContext(ctx)

	// Set owner reference if provided
	if owner != nil {
		if err := controllerutil.SetControllerReference(owner, obj, m.scheme); err != nil {
			log.Info("Could not set owner reference (resource may be cluster-scoped)", "error", err)
		}
	}

	// Try to get the existing resource
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())

	err := m.client.Get(ctx, client.ObjectKey{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}, existing)

	if err != nil {
		if errors.IsNotFound(err) {
			// Resource doesn't exist, create it
			return m.client.Create(ctx, obj)
		}
		return fmt.Errorf("failed to get existing resource: %w", err)
	}

	// Resource exists
	// Skip update for PersistentVolumeClaims as they have immutable fields once bound
	if obj.GetKind() == "PersistentVolumeClaim" {
		log.Info("Skipping update for existing PVC (immutable when bound)",
			"name", obj.GetName(),
			"namespace", obj.GetNamespace())
		return nil
	}

	// Update other resources
	obj.SetResourceVersion(existing.GetResourceVersion())
	return m.client.Update(ctx, obj)
}

// DeleteManifestResources deletes all resources defined in a manifest file
func (m *ManifestApplier) DeleteManifestResources(ctx context.Context, manifestPath string, namespace string) error {
	log := log.FromContext(ctx)

	log.Info("Reading manifest file for deletion", "path", manifestPath)

	// Read the manifest file
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest file: %w", err)
	}

	// Parse and delete resources
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	deletedCount := 0
	errorCount := 0

	for {
		var rawObj runtime.RawExtension
		if err := decoder.Decode(&rawObj); err != nil {
			if err.Error() == "EOF" {
				break
			}
			errorCount++
			continue
		}

		if len(rawObj.Raw) == 0 {
			continue
		}

		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal(rawObj.Raw, obj); err != nil {
			errorCount++
			continue
		}

		if obj.GetKind() == "" {
			continue
		}

		if obj.GetNamespace() == "" && namespace != "" {
			obj.SetNamespace(namespace)
		}

		// Delete the resource
		err := m.client.Delete(ctx, obj)
		if err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete resource",
				"kind", obj.GetKind(),
				"name", obj.GetName(),
				"namespace", obj.GetNamespace())
			errorCount++
			continue
		}

		if err == nil {
			log.Info("Deleted resource",
				"kind", obj.GetKind(),
				"name", obj.GetName(),
				"namespace", obj.GetNamespace())
			deletedCount++
		}
	}

	if errorCount > 0 {
		return fmt.Errorf("deleted %d resources with %d errors", deletedCount, errorCount)
	}

	log.Info("Successfully deleted all resources from manifest",
		"path", manifestPath,
		"count", deletedCount)

	return nil
}

// ApplyManifestTemplate reads a YAML template file, renders it with the provided data, and applies all resources
func (m *ManifestApplier) ApplyManifestTemplate(ctx context.Context, templatePath string, namespace string, owner client.Object, data interface{}) error {
	log := log.FromContext(ctx)

	log.Info("Reading manifest template file", "path", templatePath)

	// Read the template file
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read manifest template file: %w", err)
	}

	// Create template with custom functions
	funcMap := template.FuncMap{
		"toYaml": func(v interface{}) string {
			data, err := yamlv2.Marshal(v)
			if err != nil {
				return ""
			}
			return strings.TrimSuffix(string(data), "\n")
		},
		"nindent": func(spaces int, v string) string {
			indent := strings.Repeat(" ", spaces)
			lines := strings.Split(v, "\n")
			result := make([]string, len(lines))
			for i, line := range lines {
				if line != "" {
					result[i] = indent + line
				} else {
					result[i] = ""
				}
			}
			return "\n" + strings.Join(result, "\n")
		},
	}

	// Parse and execute the template
	tmpl, err := template.New("manifest").Funcs(funcMap).Parse(string(templateData))
	if err != nil {
		return fmt.Errorf("failed to parse manifest template: %w", err)
	}

	var renderedManifest bytes.Buffer
	if err := tmpl.Execute(&renderedManifest, data); err != nil {
		return fmt.Errorf("failed to execute manifest template: %w", err)
	}

	log.V(1).Info("Rendered manifest template", "content", renderedManifest.String())

	// Parse and apply resources
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(renderedManifest.Bytes()), 4096)
	appliedCount := 0
	errorCount := 0

	for {
		var rawObj runtime.RawExtension
		if err := decoder.Decode(&rawObj); err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Error(err, "Failed to decode resource, skipping")
			errorCount++
			continue
		}

		// Skip empty documents
		if len(rawObj.Raw) == 0 {
			continue
		}

		// Convert to unstructured object
		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal(rawObj.Raw, obj); err != nil {
			log.Error(err, "Failed to unmarshal resource, skipping")
			errorCount++
			continue
		}

		// Skip empty objects
		if obj.GetKind() == "" {
			continue
		}

		// Set namespace if not specified
		if obj.GetNamespace() == "" && namespace != "" {
			obj.SetNamespace(namespace)
		}

		// Apply the resource
		if err := m.applyResource(ctx, obj, owner); err != nil {
			log.Error(err, "Failed to apply resource",
				"kind", obj.GetKind(),
				"name", obj.GetName(),
				"namespace", obj.GetNamespace())
			errorCount++
			continue
		}

		log.Info("Applied resource from template",
			"kind", obj.GetKind(),
			"name", obj.GetName(),
			"namespace", obj.GetNamespace())
		appliedCount++
	}

	if errorCount > 0 {
		return fmt.Errorf("applied %d resources with %d errors", appliedCount, errorCount)
	}

	log.Info("Successfully applied all resources from manifest template",
		"path", templatePath,
		"count", appliedCount)

	return nil
}
