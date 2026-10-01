/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package helmgateway

import (
	"context"
	"fmt"

	"log/slog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/helm"
)

// DeployInput carries parameters shared by APIGateway and Kubernetes Gateway API reconciliation.
type DeployInput struct {
	Logger         *slog.Logger
	Config         *config.OperatorConfig
	GatewayName    string
	Namespace      string
	ValuesYAML     string
	ValuesFilePath string
	DockerUsername string
	DockerPassword string

	// Client reads the cluster to decide whether a legacy release is safe to
	// remove. FromGatewayAPI says which kind is reconciling, mirroring the field
	// the gateway registry keeps for the same purpose.
	Client         client.Reader
	FromGatewayAPI bool
}

// legacyCleanupInput is what deciding the fate of a legacy release needs, shared
// by the deploy and the delete path so both reach the same verdict.
type legacyCleanupInput struct {
	HelmClient     *helm.Client
	Client         client.Reader
	Logger         *slog.Logger
	Config         *config.OperatorConfig
	GatewayName    string
	Namespace      string
	FromGatewayAPI bool
}

// InstallOrUpgrade deploys or upgrades the platform-gateway Helm release.
func InstallOrUpgrade(ctx context.Context, in DeployInput) error {
	helmClient, err := helm.NewClientWithOptions(in.Config.Gateway.PlainHTTP)
	if err != nil {
		return fmt.Errorf("create Helm client: %w", err)
	}

	releaseName := helm.GetReleaseName(in.GatewayName)
	if err := helmClient.InstallOrUpgrade(ctx, helm.InstallOrUpgradeOptions{
		ReleaseName:     releaseName,
		Namespace:       in.Namespace,
		ChartPath:       in.Config.Gateway.HelmChartPath,
		ChartName:       in.Config.Gateway.HelmChartName,
		ValuesYAML:      in.ValuesYAML,
		ValuesFilePath:  in.ValuesFilePath,
		Version:         in.Config.Gateway.HelmChartVersion,
		CreateNamespace: false,
		Wait:            true,
		Timeout:         300,
		Username:        in.DockerUsername,
		Password:        in.DockerPassword,
		Insecure:        in.Config.Gateway.InsecureRegistry,
		PlainHTTP:       in.Config.Gateway.PlainHTTP,
	}); err != nil {
		return err
	}

	// Only once the replacement is installed, so a failed install leaves the
	// gateway on the release that is still serving it.
	return removeLegacyRelease(ctx, legacyCleanupInput{
		HelmClient:     helmClient,
		Client:         in.Client,
		Logger:         in.Logger,
		Config:         in.Config,
		GatewayName:    in.GatewayName,
		Namespace:      in.Namespace,
		FromGatewayAPI: in.FromGatewayAPI,
	})
}

// removeLegacyRelease uninstalls the release a gateway was deployed under before
// its computed name changed, so the old chart's resources — a gateway-runtime
// Deployment and Service among them — do not keep running unowned.
//
// The release name alone is not proof of ownership. The operator identifies a
// gateway by namespace and name and nothing else — the registry is keyed that way
// — so an APIGateway and a Kubernetes Gateway sharing a name in one namespace are
// one identity that two controllers claim. Uninstalling on name alone could pull
// the release out from under the other claimant before it has migrated.
//
// Ownership is therefore established from two facts, neither of which needs
// anything recorded inside the release (nothing is: these releases were installed
// by an operator that predates this code):
//
//	the release was installed from the gateway chart, and
//	no gateway of the other kind claims this namespace and name.
//
// Together they mean the release can only be ours. If either fails the release is
// left alone and the reason is logged; an orphan is recoverable, deleting a
// release that is still serving traffic is not.
//
// Returning the error is what makes this retryable: the caller reconciles again,
// and the Helm client already reports a release that is not there as success, so
// a gateway that never had a legacy release costs one lookup and nothing else.
func removeLegacyRelease(ctx context.Context, in legacyCleanupInput) error {
	legacy := helm.LegacyReleaseName(in.GatewayName)
	if legacy == "" {
		return nil
	}

	chartName, err := in.HelmClient.ReleaseChartName(in.Namespace, legacy)
	if err != nil {
		return fmt.Errorf("read legacy release %q: %w", legacy, err)
	}
	if chartName == "" {
		return nil
	}
	if chartName != helm.ChartName() {
		in.Logger.Info("Leaving legacy release alone: not installed from the gateway chart",
			slog.String("release", legacy), slog.String("namespace", in.Namespace),
			slog.String("chart", chartName))
		return nil
	}

	contested, err := otherKindClaimsGateway(ctx, in)
	if err != nil {
		return fmt.Errorf("check for a competing gateway named %q: %w", in.GatewayName, err)
	}
	if contested {
		in.Logger.Warn("Leaving legacy release alone: another gateway claims this namespace and name",
			slog.String("release", legacy), slog.String("namespace", in.Namespace),
			slog.String("gateway", in.GatewayName))
		return nil
	}

	in.Logger.Info("Removing legacy Helm release",
		slog.String("release", legacy), slog.String("namespace", in.Namespace))

	if err := in.HelmClient.Uninstall(ctx, helm.UninstallOptions{
		ReleaseName: legacy,
		Namespace:   in.Namespace,
		Wait:        false,
		Timeout:     60,
	}); err != nil {
		return fmt.Errorf("uninstall legacy release %q: %w", legacy, err)
	}
	return nil
}

// otherKindClaimsGateway reports whether a gateway of the kind this reconcile is
// not already claims the same namespace and name.
//
// Only a Kubernetes Gateway on a GatewayClass this operator manages counts: one on
// somebody else's class is not a claim on this identity, and treating it as one
// would leave the legacy release orphaned forever.
func otherKindClaimsGateway(ctx context.Context, in legacyCleanupInput) (bool, error) {
	key := client.ObjectKey{Namespace: in.Namespace, Name: in.GatewayName}

	if in.FromGatewayAPI {
		var other apiv1.APIGateway
		if err := in.Client.Get(ctx, key, &other); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}

	var other gatewayv1.Gateway
	if err := in.Client.Get(ctx, key, &other); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return in.Config.ManagedGatewayClass(string(other.Spec.GatewayClassName)), nil
}

// UninstallInput carries what removing a gateway's releases needs. Client and
// FromGatewayAPI are what let the legacy release be attributed before it is
// removed; see removeLegacyRelease.
type UninstallInput struct {
	Logger         *slog.Logger
	Config         *config.OperatorConfig
	Client         client.Reader
	GatewayName    string
	Namespace      string
	FromGatewayAPI bool
}

// Uninstall removes the platform-gateway Helm release.
func Uninstall(ctx context.Context, in UninstallInput) error {
	logger, cfg := in.Logger, in.Config
	gatewayName, namespace := in.GatewayName, in.Namespace
	releaseName := helm.GetReleaseName(gatewayName)
	logger.Info("Uninstalling Helm release", slog.String("release", releaseName), slog.String("namespace", namespace))

	helmClient, err := helm.NewClientWithOptions(cfg.Gateway.PlainHTTP)
	if err != nil {
		return fmt.Errorf("create Helm client: %w", err)
	}

	if err := helmClient.Uninstall(ctx, helm.UninstallOptions{
		ReleaseName: releaseName,
		Namespace:   namespace,
		Wait:        false,
		Timeout:     60,
	}); err != nil {
		return err
	}

	// A gateway deleted before it was ever reconciled onto its new name would
	// otherwise leave the legacy release behind with nothing left to own it.
	return removeLegacyRelease(ctx, legacyCleanupInput{
		HelmClient:     helmClient,
		Client:         in.Client,
		Logger:         logger,
		Config:         cfg,
		GatewayName:    gatewayName,
		Namespace:      namespace,
		FromGatewayAPI: in.FromGatewayAPI,
	})
}

// ReleaseDeployed reports whether the gateway Helm release exists and its latest revision is deployed.
func ReleaseDeployed(ctx context.Context, cfg *config.OperatorConfig, gatewayName, namespace string) (bool, error) {
	_ = ctx
	helmClient, err := helm.NewClientWithOptions(cfg.Gateway.PlainHTTP)
	if err != nil {
		return false, fmt.Errorf("create Helm client: %w", err)
	}
	releaseName := helm.GetReleaseName(gatewayName)
	return helmClient.IsReleaseDeployed(namespace, releaseName)
}
