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

import { useState } from 'react';
import {
  Box,
  Chip,
  CircularProgress,
  IconButton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
  Button,
} from '@wso2/oxygen-ui';
import { Edit, Trash2 } from '@wso2/oxygen-ui-icons-react';

interface GatewayWithType {
  id: string;
  name: string;
  displayName?: string;
  functionalityType?: string;
  description?: string;
  isActive?: boolean;
  status?: string;
  gatewayType?: string;
  isCloudGateway: boolean;
}

interface GatewaysTableProps {
  data: GatewayWithType[];
  loading: boolean;
  onEdit?: (gatewayName: string) => void;
  onView?: (gatewayName: string, gatewayType: string) => void;
  onDelete?: (id: string) => void;
  isDeleting?: boolean;
}

export default function GatewaysTable({
  data,
  loading,
  onEdit,
  onView,
  onDelete,
  isDeleting,
}: GatewaysTableProps) {
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const handleDeleteConfirm = () => {
    if (deletingId && onDelete) {
      onDelete(deletingId);
      setDeletingId(null);
    }
  };

  const getStatusChip = (gateway: GatewayWithType) => {
    const isActive = gateway.isActive || gateway.status === 'connected';
    return (
      <Chip
        size="small"
        label={isActive ? 'Active' : 'Inactive'}
        color={isActive ? 'success' : 'default'}
      />
    );
  };

  if (loading) {
    return (
      <Box
        sx={{
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          minHeight: 200,
        }}
      >
        <CircularProgress />
      </Box>
    );
  }

  if (data.length === 0) {
    return (
      <Box sx={{ py: 8, textAlign: 'center' }}>
        <Typography variant="h6" color="text.secondary">
          No Gateways Found
        </Typography>
      </Box>
    );
  }

  return (
    <Box>
      <TableContainer>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Description</TableCell>
              <TableCell>Status</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {data.map((gateway) => (
              <TableRow
                key={gateway.id}
                onClick={() =>
                  onView?.(gateway.name, gateway.gatewayType || '')
                }
                sx={{
                  cursor: 'pointer',
                  '&:hover': { backgroundColor: 'action.hover' },
                }}
              >
                <TableCell>
                  <Typography variant="body2">
                    {gateway.displayName || gateway.name}
                  </Typography>
                </TableCell>
                <TableCell>
                  <Typography
                    variant="body2"
                    color="text.secondary"
                    sx={{
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                      maxWidth: 300,
                    }}
                  >
                    {gateway.description || '-'}
                  </Typography>
                </TableCell>
                <TableCell>{getStatusChip(gateway)}</TableCell>
                <TableCell align="right">
                  {!gateway.isCloudGateway && (
                    <Box
                      sx={{
                        display: 'flex',
                        justifyContent: 'flex-end',
                        gap: 1,
                      }}
                    >
                      <IconButton
                        size="small"
                        onClick={(e) => {
                          e.stopPropagation();
                          onEdit?.(gateway.name);
                        }}
                      >
                        <Edit size={18} />
                      </IconButton>
                      <IconButton
                        size="small"
                        color="error"
                        onClick={(e) => {
                          e.stopPropagation();
                          setDeletingId(gateway.id);
                        }}
                      >
                        <Trash2 size={18} />
                      </IconButton>
                    </Box>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>

      {/* Delete Confirmation Dialog */}
      <Dialog
        open={!!deletingId}
        onClose={() => setDeletingId(null)}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>Delete Gateway</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Are you sure you want to delete this gateway? This action cannot be
            undone.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => setDeletingId(null)}
            variant="outlined"
            color="secondary"
          >
            Cancel
          </Button>
          <Button
            onClick={handleDeleteConfirm}
            color="error"
            variant="contained"
          >
            Delete
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
