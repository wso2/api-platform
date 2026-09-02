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
  Alert,
  Avatar,
  Box,
  Button,
  Card,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  IconButton,
  InputAdornment,
  PageContent,
  PageTitle,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Clock3, Plus, Search, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { deleteEnvironment, listEnvironments } from './mocks/environmentsStore';
import type { NotifySeverity } from './hostPort';
import type { Environment } from './types';

export type EnvironmentsListProps = {
  readOnly: boolean;
  onCreateClick: () => void;
  notify?: (message: string, severity?: NotifySeverity) => void;
};

function relativeTime(value: string): string {
  const elapsed = Math.max(0, Date.now() - new Date(value).getTime());
  const minutes = Math.floor(elapsed / 60_000);
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes} minute${minutes === 1 ? '' : 's'} ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} hour${hours === 1 ? '' : 's'} ago`;
  const days = Math.floor(hours / 24);
  if (days < 365) return `${days} day${days === 1 ? '' : 's'} ago`;
  const years = Math.floor(days / 365);
  return `${years} year${years === 1 ? '' : 's'} ago`;
}

const EnvironmentsList: FC<EnvironmentsListProps> = ({ readOnly, onCreateClick, notify }) => {
  const [environments, setEnvironments] = useState(() => listEnvironments());
  const [searchQuery, setSearchQuery] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<Environment | null>(null);
  const rows = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    return query ? environments.filter((environment) => environment.name.toLowerCase().includes(query)) : environments;
  }, [environments, searchQuery]);

  const handleDelete = () => {
    if (!deleteTarget) return;
    deleteEnvironment(deleteTarget.id);
    setEnvironments(listEnvironments());
    notify?.(`Environment "${deleteTarget.name}" deleted.`, 'success');
    setDeleteTarget(null);
  };

  return (
    <PageContent fullWidth>
      <Box sx={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', mb: 3 }}>
        <PageTitle>
          <PageTitle.Header>Environments</PageTitle.Header>
          <PageTitle.SubHeader>Manage the deployment environments available across your organization.</PageTitle.SubHeader>
        </PageTitle>
        {!readOnly ? <Button variant="contained" startIcon={<Plus size={20} />} onClick={onCreateClick}>Create</Button> : null}
      </Box>

      {readOnly ? <Alert severity="info" sx={{ mb: 3 }}>Environments can be managed only at Organization level.</Alert> : null}

      <TextField
        fullWidth
        placeholder="Search Environments..."
        value={searchQuery}
        onChange={(event) => setSearchQuery(event.target.value)}
        sx={{ mb: 2 }}
        slotProps={{
          input: {
            startAdornment: <InputAdornment position="start"><Search size={20} /></InputAdornment>,
          },
        }}
      />

      <Card>
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Name</TableCell>
                {readOnly ? <TableCell>Data Plane</TableCell> : null}
                <TableCell>Type</TableCell>
                <TableCell>Created</TableCell>
                {!readOnly ? <TableCell align="right">Actions</TableCell> : null}
              </TableRow>
            </TableHead>
            <TableBody>
              {rows.length === 0 ? (
                <TableRow><TableCell colSpan={4}><Typography variant="body2" color="text.secondary">No environments found.</Typography></TableCell></TableRow>
              ) : rows.map((environment) => (
                <TableRow key={environment.id}>
                  <TableCell>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
                      <Avatar sx={{ width: 36, height: 36, bgcolor: 'primary.light', color: 'primary.contrastText', fontSize: 16 }}>
                        {environment.name.trim().slice(0, 2).toUpperCase()}
                      </Avatar>
                      <Typography variant="h6" sx={{ fontWeight: 600 }}>{environment.name}</Typography>
                    </Box>
                  </TableCell>
                  {readOnly ? <TableCell>Choreo Cloud US Dataplane</TableCell> : null}
                  <TableCell>{environment.critical ? 'Critical Environment' : 'Non-Critical Environment'}</TableCell>
                  <TableCell>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, color: 'text.secondary' }}>
                      <Clock3 size={16} /><Typography variant="body2">{relativeTime(environment.createdAt)}</Typography>
                    </Box>
                  </TableCell>
                  {!readOnly ? (
                    <TableCell align="right">
                      <IconButton size="small" color="error" aria-label={`Delete ${environment.name}`} onClick={() => setDeleteTarget(environment)}>
                        <Trash2 size={16} />
                      </IconButton>
                    </TableCell>
                  ) : null}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </Card>

      <Dialog open={Boolean(deleteTarget)} onClose={() => setDeleteTarget(null)}>
        <DialogTitle>Delete Environment</DialogTitle>
        <DialogContent><DialogContentText>Are you sure you want to delete {deleteTarget?.name}?</DialogContentText></DialogContent>
        <DialogActions>
          <Button variant="outlined" color="secondary" onClick={() => setDeleteTarget(null)}>Cancel</Button>
          <Button color="error" onClick={handleDelete}>Delete</Button>
        </DialogActions>
      </Dialog>
    </PageContent>
  );
};

export default EnvironmentsList;
