import { useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Grid,
  PageContent,
  PageTitle,
  Stack,
  Tab,
  Tabs,
  Typography,
} from '@wso2/oxygen-ui';
import { KeyRound } from '@wso2/oxygen-ui-icons-react';
import { Link, useParams } from 'react-router-dom';

import {
  useCreateGatewayToken,
  useGateway,
} from '../../api/hooks/useMvpQueries';
import { useNotifications } from '../../components/Notifications';
import { ErrorState, LoadingState } from '../../components/StateViews';
import { runtimeConfig } from '../../config/runtime';
import { routes } from '../../routes/paths';
import type { Gateway } from '../../types/domain';
import { relativeTime } from '../../utils/relativeTime';
import { CopyableCommand } from './CopyableCommand';

const TOKEN_PLACEHOLDER = '<your-gateway-token>';
const ZIP = 'wso2apip-api-gateway';

const downloadCmd = () =>
  `curl -sLO https://github.com/wso2/api-platform/releases/latest/download/${ZIP}.zip && \\\n  unzip ${ZIP}.zip`;

const configureCmd = (host: string, token: string) =>
  `cat > ${ZIP}/configs/keys.env << 'ENVFILE'\n` +
  `GATEWAY_CONTROLPLANE_HOST=${host}\n` +
  `GATEWAY_REGISTRATION_TOKEN=${token}\n` +
  `ENVFILE`;

const startDockerCmd = () =>
  `cd ${ZIP} && docker compose --env-file configs/keys.env up`;

const helmCmd = (name: string, host: string, token: string) =>
  `helm install ${name} oci://ghcr.io/wso2/api-platform/helm-charts/gateway \\\n` +
  `  --set gateway.controller.controlPlane.host="${host}" \\\n` +
  `  --set gateway.controller.controlPlane.port=443 \\\n` +
  `  --set gateway.controller.controlPlane.token.value="${token}"`;

function Step({
  n,
  title,
  children,
}: {
  n: number;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <Box sx={{ mt: 2 }}>
      <Typography sx={{ fontWeight: 600, mb: 1 }} variant="body2">
        {n}. {title}
      </Typography>
      {children}
    </Box>
  );
}

export function GatewayDetailPage() {
  const { orgHandle = '', gatewayId = '' } = useParams();
  const { notify } = useNotifications();
  const gatewayQuery = useGateway(orgHandle, gatewayId, { poll: true });
  const createToken = useCreateGatewayToken(orgHandle, gatewayId);
  const [token, setToken] = useState<string>();
  const [tab, setTab] = useState(0);

  const host = runtimeConfig.gatewayControlPlaneHost;
  const tokenValue = token || TOKEN_PLACEHOLDER;

  if (gatewayQuery.isLoading) return <LoadingState label="Loading gateway" />;
  if (gatewayQuery.error)
    return <ErrorState message="Unable to load gateway" />;
  if (!gatewayQuery.data) return <ErrorState title="Gateway not found" />;

  const gateway: Gateway = gatewayQuery.data;
  const connected = Boolean(gateway.isActive);

  const generateToken = () => {
    createToken.mutate(undefined, {
      onSuccess: (result) => {
        setToken(result.token);
        notify(
          'Registration token generated. Copy it now — it is shown once.',
          'success'
        );
      },
      onError: (error) =>
        notify(
          error instanceof Error ? error.message : 'Failed to generate token',
          'error'
        ),
    });
  };

  return (
    <PageContent fullWidth>
      <PageTitle>
        <Link to={routes.gateways(orgHandle)}>
          <PageTitle.BackButton>Back to gateways</PageTitle.BackButton>
        </Link>
        <PageTitle.Header>{gateway.displayName}</PageTitle.Header>
        <PageTitle.SubHeader>{gateway.vhost}</PageTitle.SubHeader>
      </PageTitle>

      {/* Connection status banner */}
      {connected ? (
        <Alert severity="success" sx={{ mb: 3 }}>
          Gateway is connected and active.
        </Alert>
      ) : (
        <Alert
          icon={<CircularProgress size={18} />}
          severity="info"
          sx={{ mb: 3 }}
        >
          Waiting for the gateway to connect. Complete the setup steps below,
          then this page updates automatically once the gateway comes online.
        </Alert>
      )}

      <Grid container spacing={3}>
        {/* Setup hub */}
        <Grid size={{ xs: 12, md: 8 }}>
          <Card>
            <CardContent>
              <Typography sx={{ fontWeight: 700 }} variant="h6">
                Self-hosted gateway setup
              </Typography>
              <Typography
                color="text.secondary"
                sx={{ mt: 0.5 }}
                variant="body2"
              >
                Run the gateway on your own infrastructure and connect it to the
                platform using a registration token.
              </Typography>

              {/* Token */}
              <Box sx={{ mt: 2 }}>
                <Button
                  disabled={createToken.isPending}
                  onClick={generateToken}
                  startIcon={<KeyRound size={18} />}
                  variant="contained"
                >
                  {createToken.isPending
                    ? 'Generating…'
                    : token
                      ? 'Regenerate token'
                      : 'Generate registration token'}
                </Button>
                {token && (
                  <Alert severity="warning" sx={{ mt: 2 }}>
                    Copy this token now — it is shown only once. Regenerating
                    creates a new token.
                    <Box
                      sx={{
                        fontFamily: 'monospace',
                        mt: 1,
                        overflowWrap: 'anywhere',
                      }}
                    >
                      {token}
                    </Box>
                  </Alert>
                )}
              </Box>

              {/* Install instructions */}
              <Tabs onChange={(_e, v) => setTab(v)} sx={{ mt: 3 }} value={tab}>
                <Tab label="Docker" />
                <Tab label="Kubernetes" />
              </Tabs>

              {tab === 0 && (
                <Box>
                  <Step n={1} title="Download the gateway">
                    <CopyableCommand code={downloadCmd()} />
                  </Step>
                  <Step n={2} title="Configure the registration token">
                    <CopyableCommand code={configureCmd(host, tokenValue)} />
                  </Step>
                  <Step n={3} title="Start the gateway">
                    <CopyableCommand code={startDockerCmd()} />
                  </Step>
                </Box>
              )}

              {tab === 1 && (
                <Box>
                  <Step n={1} title="Install the gateway with Helm">
                    <CopyableCommand
                      code={helmCmd(gateway.name, host, tokenValue)}
                    />
                  </Step>
                </Box>
              )}
            </CardContent>
          </Card>
        </Grid>

        {/* Details */}
        <Grid size={{ xs: 12, md: 4 }}>
          <Card sx={{ height: '100%' }}>
            <CardContent>
              <Typography sx={{ fontWeight: 700, mb: 1 }} variant="h6">
                Details
              </Typography>
              <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1, mb: 2 }}>
                <Chip
                  color={
                    gateway.mode === 'self-hosted' ? 'primary' : 'secondary'
                  }
                  label={
                    gateway.mode === 'self-hosted'
                      ? 'Self-hosted'
                      : 'WSO2-managed'
                  }
                  size="small"
                />
                <Chip
                  label={gateway.functionalityType}
                  size="small"
                  variant="outlined"
                />
                {gateway.version && (
                  <Chip
                    label={`v${gateway.version}`}
                    size="small"
                    variant="outlined"
                  />
                )}
                <Chip
                  color={connected ? 'success' : 'default'}
                  label={connected ? 'Connected' : 'Not connected'}
                  size="small"
                />
                {gateway.isCritical && (
                  <Chip color="warning" label="Critical" size="small" />
                )}
              </Stack>
              <Typography color="text.secondary" variant="body2">
                Name: {gateway.name}
              </Typography>
              <Typography color="text.secondary" variant="body2">
                Control plane: {host}
              </Typography>
              {gateway.description && (
                <Typography
                  color="text.secondary"
                  sx={{ mt: 1 }}
                  variant="body2"
                >
                  {gateway.description}
                </Typography>
              )}
              {gateway.updatedAt && (
                <Typography
                  color="text.secondary"
                  sx={{ display: 'block', mt: 2 }}
                  variant="caption"
                >
                  Updated {relativeTime(gateway.updatedAt)}
                </Typography>
              )}
            </CardContent>
          </Card>
        </Grid>
      </Grid>
    </PageContent>
  );
}
