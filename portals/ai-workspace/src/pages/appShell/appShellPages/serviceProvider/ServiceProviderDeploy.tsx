import React, { useMemo } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
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
          providerName={provider?.name}
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
        <GatewayDeployMainSection />
      </Stack>
    </PageContent>
  );
}

function ServiceProviderDeployContent() {
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
    <AIEntityProvider type="llm-provider" id={providerId}>
      <GatewayDeployProvider apiId={providerId}>
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
