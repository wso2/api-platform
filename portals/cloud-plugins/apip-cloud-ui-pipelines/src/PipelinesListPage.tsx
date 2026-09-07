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
import { isLinearPipeline, orderEnvironments, resolveGatewayName } from './utils';

export type PipelinesListPageProps = {
  pipelines: Pipeline[];
  environments: Environment[];
  onCreateClick: () => void;
  onEditClick: (id: string) => void;
  onDelete: (id: string) => void;
};

const findEnvironment = (environments: Environment[], name: string) =>
  environments.find((environment) => environment.name === name);

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
    if (!pendingDelete || pendingDelete.isDefault) return;
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
            const orderedNames = orderEnvironments(pipeline.promotionPaths);
            // The builder can only edit a linear chain; a branching or cyclic
            // pipeline would lose edges if saved through it, so editing is
            // disabled for those until a graph-preserving editor exists.
            const editable = isLinearPipeline(pipeline.promotionPaths);
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
                    <Typography variant="subtitle1">
                      {pipeline.name}
                    </Typography>
                  </Box>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                    <Tooltip
                      title={
                        editable
                          ? 'Edit pipeline'
                          : 'This pipeline has a branching promotion graph and cannot be edited here yet.'
                      }
                    >
                      <span>
                        <IconButton
                          size="small"
                          aria-label={`Edit ${pipeline.name}`}
                          onClick={() => onEditClick(pipeline.id)}
                          disabled={!editable}
                        >
                          <Pencil size={16} />
                        </IconButton>
                      </span>
                    </Tooltip>
                    <Tooltip
                      title={pipeline.isDefault ? "The default pipeline can't be deleted." : ''}
                    >
                      <span>
                        <IconButton
                          size="small"
                          color="error"
                          aria-label={`Delete ${pipeline.name}`}
                          onClick={() => setPendingDelete(pipeline)}
                          disabled={pipeline.isDefault}
                        >
                          <Trash2 size={16} />
                        </IconButton>
                      </span>
                    </Tooltip>
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
                    {orderedNames.map((environmentName, index) => {
                      const environment = findEnvironment(environments, environmentName);
                      return (
                        <Box key={environmentName} sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
                          {index > 0 ? (
                            <Box sx={{ display: 'flex', color: 'text.secondary' }}>
                              <ArrowRight size={18} />
                            </Box>
                          ) : null}
                          <PipelineStageCard
                            environmentName={environment?.name ?? environmentName}
                            gatewayName={resolveGatewayName(pipeline, environments, environmentName)}
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
