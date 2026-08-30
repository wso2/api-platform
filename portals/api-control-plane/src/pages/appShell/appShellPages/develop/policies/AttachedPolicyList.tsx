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

import { Box, Button, Chip, IconButton, Stack, Tooltip, Typography } from '@wso2/oxygen-ui';
import { GripVertical, Pencil, Plus, Shield, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { useState } from 'react';

import type { ApiPolicy } from '@/types/domain';

/**
 * Renders a flat, ordered list of attached policies (the Hybrid-style policy
 * list) with edit / delete and drag-to-reorder, plus an Add button + empty
 * state. Reordering uses native HTML5 drag within this list only.
 */
export function AttachedPolicyList({
  policies,
  canAdd,
  onAdd,
  onEdit,
  onRemove,
  onReorder,
  emptyText = 'No policies attached.',
}: {
  policies: ApiPolicy[];
  canAdd: boolean;
  onAdd: () => void;
  onEdit: (index: number) => void;
  onRemove: (index: number) => void;
  onReorder: (from: number, to: number) => void;
  emptyText?: string;
}) {
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [overIndex, setOverIndex] = useState<number | null>(null);

  return (
    <Box>
      <Box
        sx={{
          alignItems: 'center',
          display: 'flex',
          justifyContent: 'space-between',
          mb: 1,
        }}
      >
        <Typography sx={{ fontWeight: 600 }} variant="body2">
          Policies
        </Typography>
        {canAdd && (
          <Button onClick={onAdd} size="small" startIcon={<Plus size={14} />} variant="outlined">
            Add Policy
          </Button>
        )}
      </Box>

      {policies.length === 0 ? (
        <Box
          sx={{
            bgcolor: 'action.hover',
            borderRadius: 1,
            color: 'text.secondary',
            px: 2,
            py: 1.5,
          }}
        >
          <Typography variant="body2">{emptyText}</Typography>
        </Box>
      ) : (
        <Stack spacing={0.75}>
          {policies.map((policy, index) => {
            const isOver = overIndex === index && dragIndex !== null && dragIndex !== index;
            return (
              <Box
                draggable={canAdd}
                key={`${policy.name}-${index}`}
                onDragEnd={() => {
                  setDragIndex(null);
                  setOverIndex(null);
                }}
                onDragOver={(event) => {
                  if (dragIndex === null) return;
                  event.preventDefault();
                  setOverIndex(index);
                }}
                onDragStart={(event) => {
                  if (!canAdd) return;
                  setDragIndex(index);
                  event.dataTransfer.effectAllowed = 'move';
                  // Mark as an internal reorder so external policy drops ignore it.
                  event.dataTransfer.setData('application/x-policy-reorder', String(index));
                }}
                onDrop={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                  if (dragIndex !== null && dragIndex !== index) onReorder(dragIndex, index);
                  setDragIndex(null);
                  setOverIndex(null);
                }}
                sx={{
                  alignItems: 'center',
                  bgcolor: 'background.paper',
                  border: '1px solid',
                  borderColor: isOver ? 'primary.main' : 'divider',
                  borderRadius: 1.5,
                  borderTopWidth: isOver ? 3 : 1,
                  display: 'flex',
                  gap: 1,
                  opacity: dragIndex === index ? 0.5 : 1,
                  px: 1.25,
                  py: 0.75,
                }}
              >
                {canAdd && (
                  <Box sx={{ color: 'text.disabled', cursor: 'grab', display: 'flex' }}>
                    <GripVertical size={16} />
                  </Box>
                )}
                <Shield size={16} />
                <Typography noWrap sx={{ flex: 1, fontWeight: 500 }} variant="body2">
                  {policy.name}
                </Typography>
                <Chip label={`v${policy.version}`} size="small" variant="outlined" />
                {canAdd && (
                  <Stack direction="row">
                    <Tooltip title="Edit">
                      <IconButton
                        aria-label="Edit policy"
                        onClick={() => onEdit(index)}
                        size="small"
                      >
                        <Pencil size={14} />
                      </IconButton>
                    </Tooltip>
                    <Tooltip title="Remove">
                      <IconButton
                        aria-label="Remove policy"
                        color="error"
                        onClick={() => onRemove(index)}
                        size="small"
                      >
                        <Trash2 size={14} />
                      </IconButton>
                    </Tooltip>
                  </Stack>
                )}
              </Box>
            );
          })}
        </Stack>
      )}

      {canAdd && policies.length > 0 && (
        <Box
          sx={{
            border: '1px dashed',
            borderColor: 'divider',
            borderRadius: 1,
            color: 'text.secondary',
            mt: 0.75,
            px: 2,
            py: 1,
            textAlign: 'center',
          }}
        >
          <Typography variant="caption">{emptyText}</Typography>
        </Box>
      )}
    </Box>
  );
}
