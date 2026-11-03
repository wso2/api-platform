/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useEffect, useState, type FC } from 'react';
import {
  Box,
  Button,
  CircularProgress,
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
import { NO_PERMISSION_TOOLTIP } from './permissions';
import type { Environment, Pipeline } from './types';
import { isLinearPipeline, orderEnvironments } from './utils';

export type PipelinesListPageProps = {
  pipelines: Pipeline[];
  environments: Environment[];
  /** Whether the user holds a scope that creates pipelines; the Create button is disabled otherwise. */
  canCreate: boolean;
  /** Whether the user holds a scope that updates pipelines; every Edit is disabled otherwise. */
  canUpdate: boolean;
  /** Whether the user holds a scope that deletes pipelines; every Delete is disabled otherwise. */
  canDelete: boolean;
  onCreateClick: () => void;
  onEditClick: (id: string) => void;
  /** Returning a promise lets the confirm dialog stay open, and busy, until the delete settles. */
  onDelete: (id: string) => void | Promise<void>;
};

const findEnvironment = (environments: Environment[], name: string) =>
  environments.find((environment) => environment.name === name);

const PipelinesListPage: FC<PipelinesListPageProps> = ({
  pipelines,
  environments,
  canCreate,
  canUpdate,
  canDelete,
  onCreateClick,
  onEditClick,
  onDelete,
}) => {
  const [expandedIds, setExpandedIds] = useState<string[]>(() => pipelines.map((p) => p.id));
  const [pendingDelete, setPendingDelete] = useState<Pipeline | null>(null);
  const [deleting, setDeleting] = useState(false);
  // A confirmation opened before the scope was lost closes rather than staying usable.
  useEffect(() => {
    if (!canDelete) setPendingDelete(null);
  }, [canDelete]);
  const [searchQuery, setSearchQuery] = useState('');

  const toggleExpanded = (id: string) => {
    setExpandedIds((prev) =>
      prev.includes(id) ? prev.filter((existing) => existing !== id) : [...prev, id]
    );
  };

  const handleConfirmDelete = async () => {
    // The dialog stays open, with its button busy, until the delete settles:
    // closing first left the pipeline on screen with nothing to say a delete was
    // running, and a second click could issue a second delete.
    if (!pendingDelete || pendingDelete.isDefault || deleting) return;
    // Rechecked here, not only on the row's button: the dialog can outlive the
    // scope that opened it.
    if (!canDelete) {
      setPendingDelete(null);
      return;
    }
    setDeleting(true);
    try {
      await onDelete(pendingDelete.id);
      setPendingDelete(null);
    } finally {
      setDeleting(false);
    }
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
          <Tooltip title={canCreate ? '' : NO_PERMISSION_TOOLTIP}>
            {/* Wrapped: a disabled button fires no events, so the tooltip explaining why would never open. */}
            <span>
              <Button
                variant="contained"
                size="small"
                startIcon={<Plus size={16} />}
                disabled={!canCreate}
                onClick={onCreateClick}
              >
                Create Pipeline
              </Button>
            </span>
          </Tooltip>
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
                        !canUpdate
                          ? NO_PERMISSION_TOOLTIP
                          : editable
                            ? 'Edit pipeline'
                            : 'This pipeline has a branching promotion graph and cannot be edited here yet.'
                      }
                    >
                      <span>
                        <IconButton
                          size="small"
                          aria-label={`Edit ${pipeline.name}`}
                          onClick={() => onEditClick(pipeline.id)}
                          disabled={!editable || !canUpdate}
                        >
                          <Pencil size={16} />
                        </IconButton>
                      </span>
                    </Tooltip>
                    <Tooltip
                      title={
                        !canDelete
                          ? NO_PERMISSION_TOOLTIP
                          : pipeline.isDefault
                            ? "The default pipeline can't be deleted."
                            : ''
                      }
                    >
                      <span>
                        <IconButton
                          size="small"
                          color="error"
                          aria-label={`Delete ${pipeline.name}`}
                          onClick={() => setPendingDelete(pipeline)}
                          disabled={pipeline.isDefault || !canDelete}
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

      <Dialog open={!!pendingDelete} onClose={deleting ? undefined : () => setPendingDelete(null)}>
        <DialogTitle>Delete pipeline?</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            Are you sure you want to delete <strong>{pendingDelete?.name}</strong>? This action cannot be
            undone.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPendingDelete(null)} disabled={deleting}>
            Cancel
          </Button>
          <Button
            color="error"
            variant="contained"
            onClick={handleConfirmDelete}
            disabled={deleting}
            startIcon={deleting ? <CircularProgress size={16} color="inherit" /> : undefined}
          >
            {deleting ? 'Deleting…' : 'Delete'}
          </Button>
        </DialogActions>
      </Dialog>
    </PageContent>
  );
};

export default PipelinesListPage;
