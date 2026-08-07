import { useMemo, useState } from 'react';
import {
  alpha,
  Box,
  Button,
  IconButton,
  InputAdornment,
  PageContent,
  PageTitle,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Check,
  Clock,
  Cloud,
  Copy,
  Layers,
  Network,
  Plus,
  Search,
  Server,
} from '@wso2/oxygen-ui-icons-react';
import { useNavigate, useParams } from 'react-router-dom';

import { useGateways } from '../../api/hooks/useMvpQueries';
import {
  EmptyState,
  ErrorState,
  LoadingState,
} from '../../components/StateViews';
import { routes } from '../../routes/paths';
import type { Gateway } from '../../types/domain';
import { relativeTime } from '../../utils/relativeTime';
import { groupGatewaysByEnvironment } from './gatewayEnvironments';
import './gatewaysUi.css';

const MODE_LABEL: Record<Gateway['mode'], string> = {
  'self-hosted': 'Self-hosted',
  managed: 'WSO2-managed',
};

type GatewayFilter = 'all' | 'managed' | 'self';

/** A small KPI summary tile. */
function StatCard({
  label,
  value,
  dotColor,
}: {
  label: string;
  value: number;
  dotColor: string;
}) {
  return (
    <Box
      sx={{
        bgcolor: 'background.paper',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 1.5,
        px: 2.5,
        py: 2,
      }}
    >
      <Stack alignItems="center" direction="row" spacing={1.25}>
        <Box
          sx={{ bgcolor: dotColor, borderRadius: '50%', height: 8, width: 8 }}
        />
        <Typography
          color="text.secondary"
          sx={{ fontWeight: 500 }}
          variant="body2"
        >
          {label}
        </Typography>
      </Stack>
      <Typography
        sx={{ fontWeight: 300, letterSpacing: '-.5px', mt: 1 }}
        variant="h4"
      >
        {value}
      </Typography>
    </Box>
  );
}

function GatewayCard({
  gateway,
  onOpen,
}: {
  gateway: Gateway;
  onOpen: (gateway: Gateway) => void;
}) {
  const [copied, setCopied] = useState(false);
  const isSelfHosted = gateway.mode === 'self-hosted';
  const kindColor = isSelfHosted ? 'primary.main' : 'info.main';

  const copyEndpoint = (event: React.MouseEvent) => {
    event.stopPropagation();
    navigator.clipboard?.writeText(gateway.vhost).catch(() => undefined);
    setCopied(true);
    setTimeout(() => setCopied(false), 1400);
  };

  const chipSx = {
    alignItems: 'center',
    bgcolor: 'action.hover',
    border: '1px solid',
    borderColor: 'divider',
    borderRadius: 1,
    color: 'text.secondary',
    display: 'inline-flex',
    fontSize: 12,
    fontWeight: 500,
    gap: 0.75,
    px: 1.25,
    py: 0.5,
  };

  return (
    <Box
      onClick={() => onOpen(gateway)}
      sx={{
        bgcolor: 'background.paper',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 2,
        cursor: 'pointer',
        p: 2.5,
        transition: 'border-color .2s, box-shadow .2s, transform .2s',
        '&:hover': {
          borderColor: 'primary.main',
          boxShadow: 3,
          transform: 'translateY(-2px)',
        },
      }}
    >
      <Stack direction="row" spacing={1.75}>
        <Box
          sx={{
            alignItems: 'center',
            bgcolor: 'action.hover',
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 1.5,
            color: 'text.secondary',
            display: 'flex',
            flex: 'none',
            height: 46,
            justifyContent: 'center',
            width: 46,
          }}
        >
          <Network size={22} />
        </Box>
        <Box sx={{ minWidth: 0 }}>
          <Typography noWrap sx={{ fontWeight: 600 }} variant="subtitle1">
            {gateway.displayName}
          </Typography>
          <Stack
            alignItems="center"
            direction="row"
            spacing={0.5}
            sx={{ mt: 0.25 }}
          >
            <Typography
              noWrap
              sx={{
                color: 'text.secondary',
                fontFamily: 'monospace',
                fontSize: 12.5,
              }}
            >
              {gateway.vhost}
            </Typography>
            <Tooltip title={copied ? 'Copied' : 'Copy endpoint'}>
              <IconButton
                onClick={copyEndpoint}
                size="small"
                sx={{ flex: 'none' }}
              >
                {copied ? <Check size={14} /> : <Copy size={14} />}
              </IconButton>
            </Tooltip>
          </Stack>
        </Box>
      </Stack>

      <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1, mt: 2 }}>
        <Box
          sx={{
            ...chipSx,
            bgcolor: (t) =>
              alpha(
                isSelfHosted ? t.palette.primary.main : t.palette.info.main,
                0.14
              ),
            borderColor: (t) =>
              alpha(
                isSelfHosted ? t.palette.primary.main : t.palette.info.main,
                0.3
              ),
            color: kindColor,
            fontWeight: 600,
          }}
        >
          {isSelfHosted ? <Server size={13} /> : <Cloud size={13} />}
          {MODE_LABEL[gateway.mode]}
        </Box>
        <Box sx={chipSx}>{gateway.functionalityType}</Box>
        {gateway.version && <Box sx={chipSx}>v{gateway.version}</Box>}
      </Stack>

      <Stack
        alignItems="center"
        direction="row"
        justifyContent="space-between"
        sx={{ borderColor: 'divider', borderTop: '1px solid', mt: 2, pt: 1.75 }}
      >
        <Stack
          alignItems="center"
          direction="row"
          spacing={0.875}
          sx={{ color: gateway.isActive ? 'success.main' : 'text.disabled' }}
        >
          <Box
            className={gateway.isActive ? 'gw-conn-dot-live' : undefined}
            sx={{
              bgcolor: gateway.isActive ? 'success.main' : 'text.disabled',
              borderRadius: '50%',
              height: 8,
              width: 8,
            }}
          />
          <Typography sx={{ fontSize: 12.5, fontWeight: 500 }}>
            {gateway.isActive ? 'Connected' : 'Not connected'}
          </Typography>
        </Stack>
        {gateway.updatedAt && (
          <Stack
            alignItems="center"
            direction="row"
            spacing={0.625}
            sx={{ color: 'text.disabled' }}
          >
            <Clock size={13} />
            <Typography sx={{ fontSize: 12 }}>
              {relativeTime(gateway.updatedAt)}
            </Typography>
          </Stack>
        )}
      </Stack>
    </Box>
  );
}

export function GatewaysPage() {
  const { orgHandle = '' } = useParams();
  const navigate = useNavigate();
  const gatewaysQuery = useGateways();
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState<GatewayFilter>('all');

  const openGateway = (gateway: Gateway) =>
    navigate(routes.gateway(orgHandle, gateway.id));
  const provision = () => navigate(routes.newGateway(orgHandle));

  const gateways = useMemo(
    () => gatewaysQuery.data || [],
    [gatewaysQuery.data]
  );

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase();
    return gateways.filter((gateway) => {
      const matchesTerm =
        !term ||
        [gateway.displayName, gateway.name, gateway.vhost]
          .filter(Boolean)
          .some((field) => field.toLowerCase().includes(term));
      const matchesFilter =
        filter === 'all' ||
        (filter === 'managed' && gateway.mode === 'managed') ||
        (filter === 'self' && gateway.mode === 'self-hosted');
      return matchesTerm && matchesFilter;
    });
  }, [gateways, search, filter]);

  const connectedCount = gateways.filter((gateway) => gateway.isActive).length;
  const environmentCount = groupGatewaysByEnvironment(gateways).length;
  const groups = groupGatewaysByEnvironment(filtered);

  return (
    <PageContent fullWidth>
      <PageTitle>
        <PageTitle.Header>API Gateways</PageTitle.Header>
        <PageTitle.SubHeader>
          Provision and manage the gateways that expose your APIs.
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button
            onClick={provision}
            startIcon={<Plus />}
            sx={{ borderRadius: 5 }}
            variant="contained"
          >
            Provision gateway
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      {gatewaysQuery.isLoading ? (
        <LoadingState label="Loading gateways" />
      ) : gatewaysQuery.error ? (
        <ErrorState message="Unable to load gateways" />
      ) : gateways.length === 0 ? (
        <EmptyState
          actionLabel="Provision gateway"
          description="Provision a self-hosted gateway to start exposing APIs to clients."
          onAction={provision}
          title="No gateways yet"
        />
      ) : (
        <Stack spacing={4}>
          {/* KPI summary */}
          <Box
            sx={{
              display: 'grid',
              gap: 2,
              gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, 1fr)' },
            }}
          >
            <StatCard
              dotColor="primary.main"
              label="Total gateways"
              value={gateways.length}
            />
            <StatCard
              dotColor="success.main"
              label="Connected"
              value={connectedCount}
            />
            <StatCard
              dotColor="text.disabled"
              label="Not connected"
              value={gateways.length - connectedCount}
            />
            <StatCard
              dotColor="info.main"
              label="Environments"
              value={environmentCount}
            />
          </Box>

          {/* Toolbar */}
          <Stack
            alignItems="center"
            direction="row"
            spacing={2}
            sx={{ flexWrap: 'wrap' }}
          >
            <TextField
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search gateways"
              size="small"
              slotProps={{
                input: {
                  startAdornment: (
                    <InputAdornment position="start">
                      <Search size={18} />
                    </InputAdornment>
                  ),
                },
              }}
              sx={{ flex: 1, maxWidth: 420, minWidth: 240 }}
              value={search}
            />
            <ToggleButtonGroup
              exclusive
              onChange={(_event, next: GatewayFilter | null) =>
                next && setFilter(next)
              }
              size="small"
              value={filter}
            >
              <ToggleButton value="all">All</ToggleButton>
              <ToggleButton value="managed">Managed</ToggleButton>
              <ToggleButton value="self">Self-hosted</ToggleButton>
            </ToggleButtonGroup>
          </Stack>

          {/* Groups */}
          {groups.length === 0 ? (
            <EmptyState
              description="Try a different search term or filter."
              title="No matching gateways"
            />
          ) : (
            groups.map((group) => (
              <Box key={group.environment.id}>
                <Stack
                  alignItems="center"
                  direction="row"
                  spacing={1.25}
                  sx={{ mb: 2 }}
                >
                  <Layers size={20} />
                  <Typography sx={{ fontWeight: 600 }} variant="h6">
                    {group.environment.name}
                  </Typography>
                  <Box
                    sx={{
                      alignItems: 'center',
                      bgcolor: 'action.hover',
                      borderRadius: 3,
                      color: 'text.secondary',
                      display: 'inline-flex',
                      fontSize: 12.5,
                      fontWeight: 600,
                      height: 24,
                      justifyContent: 'center',
                      minWidth: 24,
                      px: 1,
                    }}
                  >
                    {group.gateways.length}
                  </Box>
                </Stack>
                <Box
                  sx={{
                    display: 'grid',
                    gap: 2.5,
                    gridTemplateColumns:
                      'repeat(auto-fill, minmax(360px, 1fr))',
                  }}
                >
                  {group.gateways.map((gateway) => (
                    <GatewayCard
                      gateway={gateway}
                      key={gateway.id}
                      onOpen={openGateway}
                    />
                  ))}
                </Box>
              </Box>
            ))
          )}
        </Stack>
      )}
    </PageContent>
  );
}
