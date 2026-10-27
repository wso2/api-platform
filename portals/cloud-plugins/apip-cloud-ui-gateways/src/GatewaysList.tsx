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

import { useMemo, useState, type FC, type ReactNode } from 'react';
import {
  Avatar,
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Grid,
  IconButton,
  InputAdornment,
  ListingTable,
  PageContent,
  PageTitle,
  Stack,
  TablePagination,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Cloud, Edit, Plus, Search, Server, Settings, Trash2 } from '@wso2/oxygen-ui-icons-react';
import GatewaySettingsDrawer from './components/GatewaySettingsDrawer';
import { gatewayStatusColor, gatewayStatusLabel, gatewayTypeLabel } from './utils/gateway';
import NoGatewaysImage from './assets/images/NoGW.svg';
import type { AIWorkspaceHostPort } from './hostPort';
import type { Environment, Gateway, GatewayHosting } from './types';

export type GatewaysListProps = {
  gateways: Gateway[];
  environments: Environment[];
  /** Passed through to the configuration drawer, which calls platform-api itself. */
  port: AIWorkspaceHostPort;
  onAddClick: () => void;
  onEditClick: (gatewayId: string) => void;
  /** A self-hosted gateway's row opens its overview (Quick Start, connection setup) there. */
  onViewClick: (gatewayId: string) => void;
  /** Returning a promise lets the confirm dialog stay open, and busy, until the delete settles. */
  onDelete: (gatewayId: string, name: string) => void | Promise<void>;
};

/** Rows shown per section before pagination kicks in. */
const DEFAULT_PAGE_SIZE = 5;
const PAGE_SIZE_OPTIONS = [5, 10, 25];

function truncateText(text: string, maxLength: number): string {
  if (text.length <= maxLength) return text;
  return `${text.slice(0, maxLength).trim()}…`;
}

/**
 * One half of the hosting switcher above the table — its own icon, blurb and
 * live active/inactive split, acting as a tab: clicking it swaps which
 * hosting's gateways the table below shows. Only the selected tab draws a
 * border; the row of tabs overlaps its borders by a hairline (negative
 * margin) so adjacent tabs share one line instead of a double-thick seam,
 * exactly like the API wizard's `ApproachTab` (`DefineApproachLayout.tsx`),
 * which this mirrors.
 */
const HostingTab: FC<{
  icon: ReactNode;
  title: string;
  description: string;
  active: number;
  inactive: number;
  selected: boolean;
  onClick: () => void;
}> = ({ icon, title, description, active, inactive, selected, onClick }) => (
  <Box
    aria-pressed={selected}
    component="button"
    onClick={onClick}
    type="button"
    sx={{
      flex: 1,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 2,
      minHeight: 68,
      px: 2.5,
      py: 1.5,
      cursor: 'pointer',
      textAlign: 'left',
      color: 'text.primary',
      border: 1,
      borderColor: selected ? 'primary.main' : 'divider',
      borderRadius: '10px 10px 0 0',
      bgcolor: selected ? 'action.selected' : 'transparent',
    }}
  >
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, minWidth: 0 }}>
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          width: 40,
          height: 40,
          borderRadius: 1,
          bgcolor: selected ? 'primary.main' : 'action.hover',
          color: selected ? 'primary.contrastText' : 'text.secondary',
          flexShrink: 0,
        }}
      >
        {icon}
      </Box>
      <Box sx={{ minWidth: 0 }}>
        <Typography variant="body1" sx={{ fontWeight: 700 }}>
          {title}
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ opacity: 0.65 }}>
          {description}
        </Typography>
      </Box>
    </Box>
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, flexShrink: 0 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
        <Box sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: 'success.main' }} />
        <Typography variant="body2" color="text.secondary">
          {active} Active
        </Typography>
      </Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
        <Box sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: 'warning.main' }} />
        <Typography variant="body2" color="text.secondary">
          {inactive} Inactive
        </Typography>
      </Box>
    </Box>
  </Box>
);

/** The table + pagination for whichever hosting tab is currently selected. */
const GatewayTable: FC<{
  gateways: Gateway[];
  environmentNames: Map<string, string>;
  renderActions: (gateway: Gateway) => ReactNode;
  /** A self-hosted gateway's row opens its overview; a managed one's row does nothing. */
  onRowClick: (gateway: Gateway) => void;
}> = ({ gateways, environmentNames, renderActions, onRowClick }) => {
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_PAGE_SIZE);

  // Clamped rather than reset via an effect: a search narrowing the results
  // (or a delete) can leave `page` pointing past the last page, which would
  // otherwise render an empty table instead of falling back to what's there.
  const lastPage = Math.max(0, Math.ceil(gateways.length / rowsPerPage) - 1);
  const safePage = Math.min(page, lastPage);

  const pageRows = useMemo(
    () => gateways.slice(safePage * rowsPerPage, safePage * rowsPerPage + rowsPerPage),
    [gateways, safePage, rowsPerPage]
  );

  return (
    <ListingTable.Container disablePaper>
      <ListingTable>
        <ListingTable.Head>
          <ListingTable.Row>
            <ListingTable.Cell>Name</ListingTable.Cell>
            <ListingTable.Cell>Description</ListingTable.Cell>
            <ListingTable.Cell>Type</ListingTable.Cell>
            <ListingTable.Cell>Status</ListingTable.Cell>
            <ListingTable.Cell>Environment</ListingTable.Cell>
            <ListingTable.Cell align="right">Actions</ListingTable.Cell>
          </ListingTable.Row>
        </ListingTable.Head>
        <ListingTable.Body>
          {pageRows.length === 0 ? (
            <ListingTable.Row>
              <ListingTable.Cell colSpan={6}>
                <Typography variant="body2" color="text.secondary">
                  No gateways found.
                </Typography>
              </ListingTable.Cell>
            </ListingTable.Row>
          ) : (
            pageRows.map((gateway) => (
              <ListingTable.Row
                key={gateway.id}
                hover
                onClick={gateway.hosting === 'self-hosted' ? () => onRowClick(gateway) : undefined}
                sx={{ cursor: gateway.hosting === 'self-hosted' ? 'pointer' : 'default' }}
              >
                <ListingTable.Cell sx={{ minWidth: 220 }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <ListingTable.CellIcon
                      icon={
                        <Avatar
                          sx={{
                            width: 36,
                            height: 36,
                            backgroundColor: 'primary.light',
                            color: 'primary.contrastText',
                            fontSize: 16,
                          }}
                        >
                          {gateway.name.trim().slice(0, 2).toUpperCase()}
                        </Avatar>
                      }
                      primary={truncateText(gateway.name, 25)}
                    />
                    {/* The default is per gateway TYPE, so several
                        gateways in one environment can each be
                        marked — one per type. */}
                    {gateway.isDefault ? (
                      <Chip
                        label="Default"
                        size="small"
                        color="primary"
                        variant="outlined"
                        sx={{ height: 20, fontSize: '0.7rem' }}
                      />
                    ) : null}
                  </Box>
                </ListingTable.Cell>
                <ListingTable.Cell truncate maxWidth={300}>
                  <Typography variant="body2" color="text.secondary">
                    {gateway.description || '—'}
                  </Typography>
                </ListingTable.Cell>
                <ListingTable.Cell>
                  <Chip label={gatewayTypeLabel(gateway.type)} size="small" variant="outlined" />
                </ListingTable.Cell>
                <ListingTable.Cell>
                  {/* A failed gateway says why, on the chip itself:
                      it is the only status nothing will resolve on
                      its own, so the reason has to be where the
                      status is. */}
                  <Chip
                    size="small"
                    variant="outlined"
                    icon={<Box sx={{ width: 6, height: 6, borderRadius: '50%', bgcolor: 'currentColor' }} />}
                    label={gatewayStatusLabel(gateway.status)}
                    color={gatewayStatusColor(gateway.status)}
                    title={gateway.status === 'failed' ? gateway.statusReason : undefined}
                  />
                </ListingTable.Cell>
                <ListingTable.Cell>
                  <Typography variant="body2" color="text.secondary">
                    {environmentNames.get(gateway.environmentId) || gateway.environmentId || '—'}
                  </Typography>
                </ListingTable.Cell>
                <ListingTable.Cell align="right" onClick={(event) => event.stopPropagation()}>
                  {renderActions(gateway)}
                </ListingTable.Cell>
              </ListingTable.Row>
            ))
          )}
        </ListingTable.Body>
      </ListingTable>
      {gateways.length > 0 ? (
        <TablePagination
          component="div"
          count={gateways.length}
          page={safePage}
          rowsPerPage={rowsPerPage}
          rowsPerPageOptions={PAGE_SIZE_OPTIONS}
          onPageChange={(_event, nextPage) => setPage(nextPage)}
          onRowsPerPageChange={(event) => {
            setRowsPerPage(parseInt(event.target.value, 10));
            setPage(0);
          }}
        />
      ) : null}
    </ListingTable.Container>
  );
};

const GatewaysList: FC<GatewaysListProps> = ({
  gateways,
  environments,
  port,
  onAddClick,
  onEditClick,
  onViewClick,
  onDelete,
}) => {
  const [searchQuery, setSearchQuery] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; name: string } | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [settingsGateway, setSettingsGateway] = useState<Gateway | null>(null);

  // Environments are keyed by name, so a gateway that points at an environment
  // missing from the list (deleted, or not yet loaded) still shows its raw
  // `environmentId` rather than nothing.
  const environmentNames = useMemo(
    () => new Map(environments.map((environment) => [environment.id, environment.name])),
    [environments]
  );

  const filteredGateways = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    if (!query) return gateways;
    return gateways.filter((gateway) =>
      [gateway.name, gateway.description].filter(Boolean).join(' ').toLowerCase().includes(query)
    );
  }, [gateways, searchQuery]);

  const managedGateways = useMemo(
    () => filteredGateways.filter((gateway) => gateway.hosting === 'managed'),
    [filteredGateways]
  );
  const selfHostedGateways = useMemo(
    () => filteredGateways.filter((gateway) => gateway.hosting === 'self-hosted'),
    [filteredGateways]
  );

  // A gateway counts as Active here once it is up and not broken — 'active'
  // (connected right now) or 'provisioning' (still being built, nothing wrong
  // yet) both read as Active. Only 'failed' and 'inactive' (disconnected) pull
  // it into the Inactive bucket, since those are the two states actually worth
  // flagging on this summary.
  const summarize = (list: Gateway[]) => ({
    active: list.filter((gateway) => gateway.status === 'active' || gateway.status === 'provisioning')
      .length,
    inactive: list.filter((gateway) => gateway.status === 'failed' || gateway.status === 'inactive')
      .length,
  });
  const managedSummary = summarize(gateways.filter((gateway) => gateway.hosting === 'managed'));
  const selfHostedSummary = summarize(gateways.filter((gateway) => gateway.hosting === 'self-hosted'));

  // Which hosting's gateways the table shows — a tab, independent of what the
  // search box narrows within it.
  const [activeHosting, setActiveHosting] = useState<GatewayHosting>('managed');
  const activeGateways = activeHosting === 'managed' ? managedGateways : selfHostedGateways;

  const handleDeleteConfirm = async () => {
    // The dialog stays open, with its button busy, until the delete settles:
    // closing first left the row on screen with nothing to say a delete was even
    // running, and let a second gateway be deleted while the first was in flight.
    if (!deleteTarget || deleting) return;
    setDeleting(true);
    try {
      await onDelete(deleteTarget.id, deleteTarget.name);
      setDeleteTarget(null);
    } finally {
      setDeleting(false);
    }
  };

  const renderActions = (gateway: Gateway) => (
    <>
      <IconButton
        size="small"
        onClick={() => onEditClick(gateway.id)}
        disabled={gateway.status === 'deleting'}
        aria-label={`Edit ${gateway.name}`}
      >
        <Edit size={16} />
      </IconButton>
      {gateway.hosting === 'managed' ? (
        <IconButton
          size="small"
          onClick={() => setSettingsGateway(gateway)}
          disabled={gateway.status === 'deleting'}
          aria-label={`Configure ${gateway.name}`}
        >
          <Settings size={16} />
        </IconButton>
      ) : null}
      <IconButton
        size="small"
        color="error"
        onClick={() => setDeleteTarget({ id: gateway.id, name: gateway.name })}
        disabled={gateway.status === 'deleting'}
        aria-label={`Delete ${gateway.name}`}
      >
        <Trash2 size={16} />
      </IconButton>
    </>
  );

  return (
    <PageContent fullWidth>
      <Grid container spacing={2} sx={{ width: '100%', m: 0 }}>
        <Grid size={{ xs: 12 }}>
          <Box sx={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', flexWrap: 'nowrap', gap: 2 }}>
            <PageTitle sx={{ minWidth: 0, flex: 1 }}>
              <PageTitle.Header>Gateways</PageTitle.Header>
              <PageTitle.SubHeader>Manage and monitor your gateway deployments.</PageTitle.SubHeader>
            </PageTitle>

            <Stack direction="row" spacing={1.5} sx={{ ml: 'auto', flexShrink: 0 }}>
              {gateways.length > 0 ? (
                <Button variant="contained" onClick={onAddClick} startIcon={<Plus size={20} />}>
                  Add Gateway
                </Button>
              ) : null}
            </Stack>
          </Box>
        </Grid>

        {gateways.length === 0 ? (
          <Grid size={{ xs: 12 }}>
            <Box sx={{ alignItems: 'center', display: 'flex', justifyContent: 'center', py: 6, width: '100%' }}>
              <Stack spacing={1.5} alignItems="center" justifyContent="center" sx={{ textAlign: 'center' }}>
                <Box component="img" src={NoGatewaysImage} alt="No gateways" sx={{ width: 200, maxWidth: '80%' }} />
                <Typography variant="body1" color="text.secondary">
                  No available gateways
                </Typography>
                <Button variant="contained" onClick={onAddClick} startIcon={<Plus size={20} />}>
                  Add Gateway
                </Button>
              </Stack>
            </Box>
          </Grid>
        ) : (
          <Grid size={{ xs: 12 }}>
            <Box sx={{ display: 'flex', '& > button + button': { ml: '-1px' } }}>
              <HostingTab
                icon={<Cloud size={20} />}
                title="WSO2 managed gateways"
                description="Hosted and operated by WSO2 in the cloud."
                active={managedSummary.active}
                inactive={managedSummary.inactive}
                selected={activeHosting === 'managed'}
                onClick={() => setActiveHosting('managed')}
              />
              <HostingTab
                icon={<Server size={20} />}
                title="Self-hosted gateways"
                description="Running on your infrastructure, connected to this control plane."
                active={selfHostedSummary.active}
                inactive={selfHostedSummary.inactive}
                selected={activeHosting === 'self-hosted'}
                onClick={() => setActiveHosting('self-hosted')}
              />
            </Box>

            <Box
              sx={{
                p: 2,
                border: 1,
                borderColor: 'primary.main',
                borderRadius: '0 0 10px 10px',
                mt: '-1px',
              }}
            >
              <TextField
                fullWidth
                placeholder="Search gateways..."
                value={searchQuery}
                onChange={(event) => setSearchQuery(event.target.value)}
                sx={{ mb: 2 }}
                slotProps={{
                  input: {
                    startAdornment: (
                      <InputAdornment position="start">
                        <Search size={20} />
                      </InputAdornment>
                    ),
                  },
                }}
              />

              <GatewayTable
                gateways={activeGateways}
                environmentNames={environmentNames}
                renderActions={renderActions}
                onRowClick={(gateway) => onViewClick(gateway.id)}
              />
            </Box>
          </Grid>
        )}
      </Grid>

      <Dialog open={Boolean(deleteTarget)} onClose={deleting ? undefined : () => setDeleteTarget(null)}>
        <DialogTitle>Delete Gateway</DialogTitle>
        <DialogContent>
          <DialogContentText>Are you sure you want to delete {deleteTarget?.name}?</DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteTarget(null)} variant="outlined" color="secondary" disabled={deleting}>
            Cancel
          </Button>
          <Button
            color="error"
            onClick={handleDeleteConfirm}
            disabled={deleting}
            startIcon={deleting ? <CircularProgress size={16} color="inherit" /> : undefined}
          >
            {deleting ? 'Deleting…' : 'Delete'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Keyed by gateway so the form's draft state belongs to one gateway and cannot outlive it. */}
      <GatewaySettingsDrawer
        key={settingsGateway?.id ?? 'none'}
        open={settingsGateway !== null}
        onClose={() => setSettingsGateway(null)}
        gateway={settingsGateway}
        port={port}
      />
    </PageContent>
  );
};

export default GatewaysList;
