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
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ApplyManifest is a convenience function to apply a manifest file in one call
// This is useful for quick operations where you don't need to reuse the applier
func ApplyManifest(ctx context.Context, client client.Client, scheme *runtime.Scheme, manifestPath, namespace string, owner client.Object) error {
	applier := NewManifestApplier(client, scheme)
	return applier.ApplyManifestFile(ctx, manifestPath, namespace, owner)
}

// DeleteManifest is a convenience function to delete resources from a manifest file
func DeleteManifest(ctx context.Context, client client.Client, scheme *runtime.Scheme, manifestPath, namespace string) error {
	applier := NewManifestApplier(client, scheme)
	return applier.DeleteManifestResources(ctx, manifestPath, namespace)
}

// ApplyManifestTemplate is a convenience function to apply a manifest template with data in one call
// This is useful for quick operations where you don't need to reuse the applier
func ApplyManifestTemplate(ctx context.Context, client client.Client, scheme *runtime.Scheme, templatePath, namespace string, owner client.Object, data interface{}) error {
	applier := NewManifestApplier(client, scheme)
	return applier.ApplyManifestTemplate(ctx, templatePath, namespace, owner, data)
}
