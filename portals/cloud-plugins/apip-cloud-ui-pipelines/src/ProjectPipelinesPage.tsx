/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useMemo, useState, type FC } from 'react';
import {
  Box,
  Button,
  Checkbox,
  Chip,
  Collapse,
  ComplexSelect,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  ListSubheader,
  PageContent,
  PageTitle,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ArrowRight, ChevronDown, ChevronUp, Plus, X } from '@wso2/oxygen-ui-icons-react';
import PipelineStageCard from './components/PipelineStageCard';
import type { Environment, Pipeline } from './types';

export type ProjectPipelinesPageProps = {
  pipelines: Pipeline[];
  organizationPipelines: Pipeline[];
  environments: Environment[];
  onAssociate: (pipelineIds: string[]) => void;
  onRemove: (pipelineId: string) => void;
};

const ProjectPipelinesPage: FC<ProjectPipelinesPageProps> = ({
  pipelines,
  organizationPipelines,
  environments,
  onAssociate,
  onRemove,
}) => {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [selectorOpen, setSelectorOpen] = useState(false);
  const [expandedIds, setExpandedIds] = useState<string[]>(() => pipelines.map((pipeline) => pipeline.id));

  const associatedIds = useMemo(() => new Set(pipelines.map((pipeline) => pipeline.id)), [pipelines]);
  const availablePipelines = organizationPipelines.filter(
    (pipeline) => !pipeline.isDefault && !associatedIds.has(pipeline.id)
  );

  const closeDialog = () => {
    setDialogOpen(false);
    setSelectorOpen(false);
    setSelectedIds([]);
  };

  const handleAdd = () => {
    onAssociate(selectedIds);
    setExpandedIds((current) => [...new Set([...current, ...selectedIds])]);
    closeDialog();
  };

  return (
    <PageContent fullWidth>
      <PageTitle sx={{ mb: 2 }}>
        <PageTitle.Header>Continuous Deployment Pipelines</PageTitle.Header>
        <PageTitle.Actions>
          <Tooltip
            title={availablePipelines.length === 0 ? 'All organization pipelines are already associated with this project.' : ''}
          >
            <span>
              <Button
                variant="contained"
                size="small"
                startIcon={<Plus size={16} />}
                onClick={() => setDialogOpen(true)}
                disabled={availablePipelines.length === 0}
              >
                Add
              </Button>
            </span>
          </Tooltip>
        </PageTitle.Actions>
      </PageTitle>

      {pipelines.length === 0 ? (
        <Box sx={{ border: '1px dashed', borderColor: 'divider', borderRadius: 1.5, py: 6, textAlign: 'center' }}>
          <Typography variant="body2" color="text.secondary">
            No organization pipelines are available for this project.
          </Typography>
        </Box>
      ) : (
        <Stack spacing={2}>
          {pipelines.map((pipeline) => {
            const expanded = expandedIds.includes(pipeline.id);
            return (
              <Box key={pipeline.id} sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1.5, overflow: 'hidden' }}>
                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', px: 2, py: 1.5, bgcolor: 'action.hover' }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Typography variant="subtitle1">{pipeline.name}</Typography>
                    {pipeline.isDefault ? <Chip label="Default" size="small" color="primary" variant="outlined" /> : null}
                  </Box>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    {!pipeline.isDefault ? (
                      <Button size="small" variant="outlined" color="error" onClick={() => onRemove(pipeline.id)}>
                        Remove
                      </Button>
                    ) : null}
                    <IconButton
                      size="small"
                      aria-label={expanded ? `Collapse ${pipeline.name}` : `Expand ${pipeline.name}`}
                      onClick={() => setExpandedIds((current) =>
                        current.includes(pipeline.id)
                          ? current.filter((id) => id !== pipeline.id)
                          : [...current, pipeline.id]
                      )}
                    >
                      {expanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
                    </IconButton>
                  </Box>
                </Box>
                <Collapse in={expanded}>
                  <Box sx={{ px: 2, py: 2, display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
                    {pipeline.stages.map((stage, index) => {
                      const environment = environments.find((item) => item.id === stage.environmentId);
                      const gateway = environment?.gateways.find((item) => item.id === stage.defaultGatewayId);
                      return (
                        <Box key={stage.id} sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
                          {index > 0 ? <Box sx={{ display: 'flex', color: 'text.secondary' }}><ArrowRight size={18} /></Box> : null}
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

      <Dialog open={dialogOpen} onClose={closeDialog} fullWidth maxWidth="sm">
        <DialogTitle sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          Add Deployment Pipelines
          <IconButton aria-label="Close" onClick={closeDialog}><X size={20} /></IconButton>
        </DialogTitle>
        <DialogContent>
          <Typography color="text.secondary" sx={{ mb: 2 }}>
            Select deployment pipelines to add to your project.
          </Typography>
          <ComplexSelect
            fullWidth
            multiple
            displayEmpty
            open={selectorOpen}
            onOpen={() => setSelectorOpen(true)}
            onClose={() => setSelectorOpen(false)}
            value={selectedIds}
            onChange={(event) => {
              const value = event.target.value;
              setSelectedIds(typeof value === 'string' ? value.split(',') : value as string[]);
            }}
            renderValue={(selected) => {
              const ids = selected as string[];
              if (ids.length === 0) {
                return <Typography color="text.secondary">Select pipelines</Typography>;
              }
              return ids
                .map((id) => availablePipelines.find((pipeline) => pipeline.id === id)?.name)
                .filter(Boolean)
                .join(', ');
            }}
            MenuProps={{
              autoFocus: false,
              disableAutoFocusItem: true,
              slotProps: { paper: { sx: { maxHeight: 320 } } },
            }}
          >
            <ListSubheader
              component="div"
              sx={{ display: 'flex', justifyContent: 'space-between', borderBottom: '1px solid', borderColor: 'divider' }}
            >
              <Button
                size="small"
                onClick={(event) => {
                  event.stopPropagation();
                  setSelectedIds(availablePipelines.map((pipeline) => pipeline.id));
                }}
              >
                Select All
              </Button>
              <Button
                size="small"
                onClick={(event) => {
                  event.stopPropagation();
                  setSelectedIds([]);
                }}
              >
                Clear
              </Button>
            </ListSubheader>
            {availablePipelines.map((pipeline) => (
              <ComplexSelect.MenuItem key={pipeline.id} value={pipeline.id}>
                <Checkbox checked={selectedIds.includes(pipeline.id)} />
                <ComplexSelect.MenuItem.Text primary={pipeline.name} />
              </ComplexSelect.MenuItem>
            ))}
          </ComplexSelect>
        </DialogContent>
        <DialogActions>
          <Button onClick={closeDialog}>Cancel</Button>
          <Tooltip title={selectedIds.length === 0 ? 'Select at least one pipeline to add.' : ''}>
            <span>
              <Button variant="contained" disabled={selectedIds.length === 0} onClick={handleAdd}>
                Add
              </Button>
            </span>
          </Tooltip>
        </DialogActions>
      </Dialog>
    </PageContent>
  );
};

export default ProjectPipelinesPage;
