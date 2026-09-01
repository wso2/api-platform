/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useState, type FC } from 'react';
import {
  Box,
  Button,
  Chip,
  Collapse,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  PageContent,
  PageTitle,
  SearchBar,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ArrowRight,
  ChevronDown,
  ChevronUp,
  Pencil,
  Plus,
  Trash2,
} from '@wso2/oxygen-ui-icons-react';
import PipelineStageCard from './components/PipelineStageCard';
import type { Environment, Pipeline } from './types';

export type PipelinesListPageProps = {
  pipelines: Pipeline[];
  environments: Environment[];
  onCreateClick: () => void;
  onEditClick: (id: string) => void;
  onDelete: (id: string) => void;
};

const findEnvironment = (environments: Environment[], environmentId: string) =>
  environments.find((environment) => environment.id === environmentId);

const findGateway = (environment: Environment | undefined, gatewayId: string) =>
  environment?.gateways.find((gateway) => gateway.id === gatewayId);

const PipelinesListPage: FC<PipelinesListPageProps> = ({
  pipelines,
  environments,
  onCreateClick,
  onEditClick,
  onDelete,
}) => {
  const [expandedIds, setExpandedIds] = useState<string[]>(() => pipelines.map((p) => p.id));
  const [pendingDelete, setPendingDelete] = useState<Pipeline | null>(null);
  const [searchQuery, setSearchQuery] = useState('');

  const toggleExpanded = (id: string) => {
    setExpandedIds((prev) =>
      prev.includes(id) ? prev.filter((existing) => existing !== id) : [...prev, id]
    );
  };

  const handleConfirmDelete = () => {
    if (!pendingDelete) return;
    onDelete(pendingDelete.id);
    setPendingDelete(null);
  };

  const normalizedQuery = searchQuery.trim().toLowerCase();
  const filteredPipelines = normalizedQuery
    ? pipelines.filter((pipeline) => pipeline.name.toLowerCase().includes(normalizedQuery))
    : pipelines;

  return (
    <PageContent fullWidth>
      <PageTitle sx={{ mb: 2 }}>
        <PageTitle.Header>Continuous Deployment Pipelines</PageTitle.Header>
        <PageTitle.Actions>
          <Button variant="contained" size="small" startIcon={<Plus size={16} />} onClick={onCreateClick}>
            Create Pipeline
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      {pipelines.length > 0 ? (
        <Box sx={{ mb: 2, maxWidth: 360 }}>
          <SearchBar
            fullWidth
            placeholder="Search pipelines"
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
          />
        </Box>
      ) : null}

      {pipelines.length === 0 ? (
        <Box
          sx={{
            border: '1px dashed',
            borderColor: 'divider',
            borderRadius: 1.5,
            py: 6,
            textAlign: 'center',
          }}
        >
          <Typography variant="body2" color="text.secondary">
            No deployment pipelines yet.
          </Typography>
        </Box>
      ) : filteredPipelines.length === 0 ? (
        <Box
          sx={{
            border: '1px dashed',
            borderColor: 'divider',
            borderRadius: 1.5,
            py: 6,
            textAlign: 'center',
          }}
        >
          <Typography variant="body2" color="text.secondary">
            No pipelines match &quot;{searchQuery.trim()}&quot;.
          </Typography>
        </Box>
      ) : (
        <Stack spacing={2}>
          {filteredPipelines.map((pipeline) => {
            const expanded = expandedIds.includes(pipeline.id);
            return (
              <Box
                key={pipeline.id}
                sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1.5, overflow: 'hidden' }}
              >
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    px: 2,
                    py: 1.5,
                    bgcolor: 'action.hover',
                  }}
                >
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Typography variant="subtitle1" >
                      {pipeline.name}
                    </Typography>
                    {pipeline.isDefault ? (
                      <Chip label="Default" size="small" color="primary" variant="outlined" />
                    ) : null}
                  </Box>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                    <Tooltip title="Edit pipeline">
                      <IconButton
                        size="small"
                        aria-label={`Edit ${pipeline.name}`}
                        onClick={() => onEditClick(pipeline.id)}
                      >
                        <Pencil size={16} />
                      </IconButton>
                    </Tooltip>
                    <IconButton
                      size="small"
                      color="error"
                      aria-label={`Delete ${pipeline.name}`}
                      onClick={() => setPendingDelete(pipeline)}
                    >
                      <Trash2 size={16} />
                    </IconButton>
                    <IconButton
                      size="small"
                      aria-label={expanded ? 'Collapse' : 'Expand'}
                      onClick={() => toggleExpanded(pipeline.id)}
                    >
                      {expanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
                    </IconButton>
                  </Box>
                </Box>
                <Collapse in={expanded}>
                  <Box sx={{ px: 2, py: 2, display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
                    {pipeline.stages.map((stage, index) => {
                      const environment = findEnvironment(environments, stage.environmentId);
                      const gateway = findGateway(environment, stage.defaultGatewayId);
                      return (
                        <Box key={stage.id} sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
                          {index > 0 ? (
                            <Box sx={{ display: 'flex', color: 'text.secondary' }}>
                              <ArrowRight size={18} />
                            </Box>
                          ) : null}
                          <PipelineStageCard
                            environmentName={environment?.name ?? stage.environmentId}
                            gatewayName={gateway?.name ?? stage.defaultGatewayId}
                            critical={environment?.critical}
                          />
                        </Box>
                      );
                    })}
                  </Box>
                </Collapse>
              </Box>
            );
          })}
        </Stack>
      )}

      <Dialog open={!!pendingDelete} onClose={() => setPendingDelete(null)}>
        <DialogTitle>Delete pipeline?</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            Are you sure you want to delete <strong>{pendingDelete?.name}</strong>? This action cannot be
            undone.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPendingDelete(null)}>Cancel</Button>
          <Button color="error" variant="contained" onClick={handleConfirmDelete}>
            Delete
          </Button>
        </DialogActions>
      </Dialog>
    </PageContent>
  );
};

export default PipelinesListPage;
