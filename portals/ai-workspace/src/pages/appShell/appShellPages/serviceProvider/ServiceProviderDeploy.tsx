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

import React, { useMemo } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  Alert,
  Box,
  Button,
  Grid,
  PageContent,
  PageTitle,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft } from '@wso2/oxygen-ui-icons-react';
import {
  LLMProviderProvider,
  useLLMProvider,
} from '../../../../contexts/llmProvider';
import {
  GatewayDeployProvider,
  useGatewayDeploy,
} from '../../../../contexts/GatewayDeployContext';
import { GatewayDeployMainSection } from '../../../../Components/GatewayDeploy';
import { FormattedMessage } from 'react-intl';
import LLLMStepBanner, {
  type LLLMStepBannerStepId,
} from '../quickStart/lllmStepBanner';
import {
  buildOrgPath,
  buildProjectPath,
} from '../../../../utils/projectRouting';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { AIEntityProvider } from '../../../../contexts/AIEntitiesContext';

type ServiceProviderDeployLayoutProps = {
  providerId: string;
};

function ServiceProviderDeployLayout({ providerId }: ServiceProviderDeployLayoutProps) {
  const navigate = useNavigate();
  const { provider } = useLLMProvider();
  const { deployments, isLoadingDeployments } = useGatewayDeploy();
  const { currentOrganization, currentProject } = useAppShell();
  const isProjectLevel = Boolean(currentProject?.id);
  const handleLLLMStepBannerClick = (stepId: LLLMStepBannerStepId) => {
    if (!providerId) return;

    if (stepId === 'deploy-to-gateway') {
      return;
    }

    const overviewPath = isProjectLevel
      ? buildProjectPath(
          currentOrganization,
          currentProject,
          `/service-provider/${providerId}`
        )
      : buildOrgPath(currentOrganization, `/service-provider/${providerId}`);

    navigate(overviewPath);
  };
  const stepBannerRefreshTrigger = useMemo(() => {
    if (isLoadingDeployments) return 'loading';
    const deploymentSignature = (deployments?.list ?? [])
      .map(
        (deployment) =>
          `${deployment.deploymentId}:${deployment.status}:${deployment.gatewayId}:${deployment.updatedAt ?? deployment.createdAt}`
      )
      .sort()
      .join('|');
    return `${deployments?.count ?? 0}:${deploymentSignature}`;
  }, [deployments?.count, deployments?.list, isLoadingDeployments]);

  return (
    <PageContent fullWidth>
      <Button
        size="small"
        startIcon={<ChevronLeft size={24} />}
        onClick={() => navigate(-1)}
      >
        Back to Service Provider
      </Button>
      <Box mt={1}>
        <LLLMStepBanner
          providerName={provider?.displayName}
          onStepClick={handleLLLMStepBannerClick}
          refreshTrigger={stepBannerRefreshTrigger}
        />
      </Box>

      <Grid size={{ xs: 12 }} sx={{ mt: 2 }}>
        <PageTitle>
          <PageTitle.Header>
            {' '}
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderDeploy.deploy.to.gateway"
              defaultMessage={'Deploy to Gateway'}
            />
          </PageTitle.Header>
          <PageTitle.SubHeader>
            <FormattedMessage
              id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderDeploy.deploy.service.provider.to.your.gateways"
              defaultMessage={'Deploy Service Provider to your Gateways'}
            />
          </PageTitle.SubHeader>
        </PageTitle>
      </Grid>

      <Stack spacing={2} sx={{ mt: 2 }}>
        {provider?.readOnly && (
          <Alert severity="info">
            This provider was created from a gateway. You can view its deployments,
            but deploy, redeploy, restore and undeploy actions are managed by the
            gateway and are unavailable in AI Workspace.
          </Alert>
        )}
        <GatewayDeployMainSection />
      </Stack>
    </PageContent>
  );
}

function ServiceProviderDeployContent() {
  const { providerId } = useParams<{ providerId: string }>();
  const { provider } = useLLMProvider();

  if (!providerId) {
    return (
      <PageContent fullWidth>
        <Typography variant="h6">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderDeploy.provider.id.is.required"
            defaultMessage={'Provider ID is required'}
          />
        </Typography>
      </PageContent>
    );
  }

  return (
    <AIEntityProvider type="llm-provider" id={providerId}>
      <GatewayDeployProvider apiId={providerId} readOnly={Boolean(provider?.readOnly)}>
        <ServiceProviderDeployLayout providerId={providerId} />
      </GatewayDeployProvider>
    </AIEntityProvider>
  );
}

export default function ServiceProviderDeploy() {
  const { providerId } = useParams<{ providerId: string }>();

  if (!providerId) {
    return (
      <PageContent fullWidth>
        <Typography variant="h6">
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.serviceProvider.ServiceProviderDeploy.provider.id.is.required"
            defaultMessage={'Provider ID is required'}
          />
        </Typography>
      </PageContent>
    );
  }

  return (
    <LLMProviderProvider providerId={providerId}>
      <ServiceProviderDeployContent />
    </LLMProviderProvider>
  );
}
