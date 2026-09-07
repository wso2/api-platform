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

import { useMemo, useState, type FC } from 'react';
import {
  Avatar,
  Box,
  Button,
  Card,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Grid,
  IconButton,
  InputAdornment,
  PageContent,
  PageTitle,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Edit, Plus, Search, Settings, Trash2 } from '@wso2/oxygen-ui-icons-react';
import GatewaySettingsDrawer from './components/GatewaySettingsDrawer';
import { gatewayTypeLabel } from './utils/gateway';
import NoGatewaysImage from './assets/images/NoGW.svg';
import type { Environment, Gateway } from './types';

export type GatewaysListProps = {
  gateways: Gateway[];
  environments: Environment[];
  onAddClick: () => void;
  onEditClick: (gatewayId: string) => void;
  onDelete: (gatewayId: string, name: string) => void;
};

function truncateText(text: string, maxLength: number): string {
  if (text.length <= maxLength) return text;
  return `${text.slice(0, maxLength).trim()}…`;
}

const GatewaysList: FC<GatewaysListProps> = ({
  gateways,
  environments,
  onAddClick,
  onEditClick,
  onDelete,
}) => {
  const [searchQuery, setSearchQuery] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; name: string } | null>(null);
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

  const handleDeleteConfirm = () => {
    if (!deleteTarget) return;
    onDelete(deleteTarget.id, deleteTarget.name);
    setDeleteTarget(null);
  };

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
            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', py: 6 }}>
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
          <>
            <Grid size={{ xs: 12 }}>
              <TextField
                fullWidth
                placeholder="Search Gateways..."
                value={searchQuery}
                onChange={(event) => setSearchQuery(event.target.value)}
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
            </Grid>

            <Grid size={{ xs: 12 }}>
              <Card>
                <TableContainer>
                  <Table size="small">
                    <TableHead>
                      <TableRow>
                        <TableCell>Name</TableCell>
                        <TableCell>Description</TableCell>
                        <TableCell>Type</TableCell>
                        <TableCell>Status</TableCell>
                        <TableCell>Environment</TableCell>
                        <TableCell align="right">Actions</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {filteredGateways.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={6}>
                            <Typography variant="body2" color="text.secondary">
                              No gateways found.
                            </Typography>
                          </TableCell>
                        </TableRow>
                      ) : (
                        filteredGateways.map((gateway) => (
                          <TableRow key={gateway.id}>
                            <TableCell sx={{ minWidth: 220 }}>
                              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
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
                                <Typography variant="h6" sx={{ fontWeight: 600 }}>
                                  {truncateText(gateway.name, 25)}
                                </Typography>
                              </Box>
                            </TableCell>
                            <TableCell>
                              <Typography
                                variant="body2"
                                color="text.secondary"
                                sx={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 300 }}
                              >
                                {gateway.description || '—'}
                              </Typography>
                            </TableCell>
                            <TableCell>
                              <Chip label={gatewayTypeLabel(gateway.type)} size="small" variant="outlined" />
                            </TableCell>
                            <TableCell>
                              {/* Inactive is a warning, not an error: a gateway
                                  reads inactive while it is still being
                                  provisioned, before its controller connects. */}
                              <Chip
                                size="small"
                                variant="outlined"
                                label={gateway.status === 'active' ? 'Active' : 'Inactive'}
                                color={gateway.status === 'active' ? 'success' : 'warning'}
                              />
                            </TableCell>
                            <TableCell>
                              <Typography variant="body2" color="text.secondary">
                                {environmentNames.get(gateway.environmentId) ||
                                  gateway.environmentId ||
                                  '—'}
                              </Typography>
                            </TableCell>
                            <TableCell align="right">
                              <IconButton size="small" onClick={() => onEditClick(gateway.id)} aria-label={`Edit ${gateway.name}`}>
                                <Edit size={16} />
                              </IconButton>
                              <IconButton
                                size="small"
                                onClick={() => setSettingsGateway(gateway)}
                                aria-label={`Configure ${gateway.name}`}
                              >
                                <Settings size={16} />
                              </IconButton>
                              <IconButton
                                size="small"
                                color="error"
                                onClick={() => setDeleteTarget({ id: gateway.id, name: gateway.name })}
                                aria-label={`Delete ${gateway.name}`}
                              >
                                <Trash2 size={16} />
                              </IconButton>
                            </TableCell>
                          </TableRow>
                        ))
                      )}
                    </TableBody>
                  </Table>
                </TableContainer>
              </Card>
            </Grid>
          </>
        )}
      </Grid>

      <Dialog open={Boolean(deleteTarget)} onClose={() => setDeleteTarget(null)}>
        <DialogTitle>Delete Gateway</DialogTitle>
        <DialogContent>
          <DialogContentText>Are you sure you want to delete {deleteTarget?.name}?</DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteTarget(null)} variant="outlined" color="secondary">
            Cancel
          </Button>
          <Button color="error" onClick={handleDeleteConfirm}>
            Delete
          </Button>
        </DialogActions>
      </Dialog>

      <GatewaySettingsDrawer
        open={settingsGateway !== null}
        onClose={() => setSettingsGateway(null)}
        gateway={settingsGateway}
        environments={environments}
      />
    </PageContent>
  );
};

export default GatewaysList;
