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

	"go.uber.org/zap"

	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/helm"
)

// DeployInput carries parameters shared by APIGateway and Kubernetes Gateway API reconciliation.
type DeployInput struct {
	Logger         *zap.Logger
	Config         *config.OperatorConfig
	GatewayName    string
	Namespace      string
	ValuesYAML     string
	ValuesFilePath string
	DockerUsername string
	DockerPassword string
}

// InstallOrUpgrade deploys or upgrades the platform-gateway Helm release.
func InstallOrUpgrade(ctx context.Context, in DeployInput) error {
	helmClient, err := helm.NewClientWithOptions(in.Config.Gateway.PlainHTTP)
	if err != nil {
		return fmt.Errorf("create Helm client: %w", err)
	}

	releaseName := helm.GetReleaseName(in.GatewayName)
	return helmClient.InstallOrUpgrade(ctx, helm.InstallOrUpgradeOptions{
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
	})
}

// Uninstall removes the platform-gateway Helm release.
func Uninstall(ctx context.Context, logger *zap.Logger, cfg *config.OperatorConfig, gatewayName, namespace string) error {
	releaseName := helm.GetReleaseName(gatewayName)
	logger.Info("Uninstalling Helm release", zap.String("release", releaseName), zap.String("namespace", namespace))

	helmClient, err := helm.NewClientWithOptions(cfg.Gateway.PlainHTTP)
	if err != nil {
		return fmt.Errorf("create Helm client: %w", err)
	}

	return helmClient.Uninstall(ctx, helm.UninstallOptions{
		ReleaseName: releaseName,
		Namespace:   namespace,
		Wait:        false,
		Timeout:     60,
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
