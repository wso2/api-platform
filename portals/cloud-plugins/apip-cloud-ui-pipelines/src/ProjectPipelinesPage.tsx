/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useEffect, useMemo, useState, type FC } from 'react';
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  ComplexSelect,
  PageContent,
  PageTitle,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { ArrowRight } from '@wso2/oxygen-ui-icons-react';
import PipelineStageCard from './components/PipelineStageCard';
import type { Environment, Pipeline } from './types';

export type ProjectPipelinesPageProps = {
  /** The pipeline(s) the project is currently bound to — zero or one. */
  pipelines: Pipeline[];
  /** All deployment pipelines in the organization, to choose from. */
  organizationPipelines: Pipeline[];
  environments: Environment[];
  /** Persist the project's pipeline binding. Given the selected pipeline id. */
  onAssociate: (pipelineIds: string[]) => void;
  /** A binding write is in flight — disables the selector and actions. */
  saving?: boolean;
};

/**
 * Project-scoped view for the project's single deployment-pipeline binding: pick
 * which organization pipeline this project promotes through (by id), preview its
 * promotion stages, and save. A project is bound to exactly one pipeline (the
 * platform-api `/projects/{id}/deployment-pipeline` binding), so this is a
 * selector, not an association list.
 */
const ProjectPipelinesPage: FC<ProjectPipelinesPageProps> = ({
  pipelines,
  organizationPipelines,
  environments,
  onAssociate,
  saving = false,
}) => {
  const currentPipelineId = pipelines[0]?.id ?? '';
  const [selectedPipelineId, setSelectedPipelineId] = useState(currentPipelineId);

  useEffect(() => setSelectedPipelineId(currentPipelineId), [currentPipelineId]);

  const selectedPipeline = useMemo(
    () => organizationPipelines.find((pipeline) => pipeline.id === selectedPipelineId),
    [organizationPipelines, selectedPipelineId]
  );
  const hasChanges = selectedPipelineId !== currentPipelineId;

  return (
    <PageContent fullWidth>
      <PageTitle sx={{ mb: 3 }}>
        <PageTitle.Header>Deployment Pipeline</PageTitle.Header>
        <PageTitle.SubHeader>
          Each project uses a single continuous deployment pipeline. All components in this project
          promote through its stages.
        </PageTitle.SubHeader>
      </PageTitle>

      <Box sx={{ width: '100%', maxWidth: 720, mb: 4 }}>
        <Typography variant="body2" sx={{ fontWeight: 600, mb: 1 }}>
          Pipeline
        </Typography>
        <ComplexSelect
          fullWidth
          displayEmpty
          sx={{
            minHeight: 64,
            '& .MuiSelect-select': {
              minHeight: 'unset',
              py: 2,
              display: 'flex',
              alignItems: 'center',
            },
          }}
          disabled={saving}
          value={selectedPipelineId}
          onChange={(event) => setSelectedPipelineId(event.target.value as string)}
          renderValue={(value) => {
            const pipeline = organizationPipelines.find((item) => item.id === value);
            if (!pipeline)
              return <Typography color="text.secondary">Select a pipeline</Typography>;
            return (
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <Typography>{pipeline.name}</Typography>
                {pipeline.isDefault ? (
                  <Chip label="Default" size="small" color="primary" variant="outlined" />
                ) : null}
              </Box>
            );
          }}
          MenuProps={{ slotProps: { paper: { sx: { maxHeight: 420 } } } }}
        >
          {organizationPipelines.map((pipeline) => (
            <ComplexSelect.MenuItem key={pipeline.id} value={pipeline.id}>
              <ComplexSelect.MenuItem.Text
                primary={
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Typography>{pipeline.name}</Typography>
                    {pipeline.isDefault ? (
                      <Chip label="Default" size="small" color="primary" variant="outlined" />
                    ) : null}
                  </Box>
                }
                secondary={pipeline.stages
                  .map(
                    (stage) =>
                      environments.find((environment) => environment.id === stage.environmentId)
                        ?.name ?? stage.environmentId
                  )
                  .join('  →  ')}
              />
            </ComplexSelect.MenuItem>
          ))}
        </ComplexSelect>
      </Box>

      {selectedPipeline ? (
        <Box
          sx={{
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 1.5,
            overflow: 'hidden',
            mb: 3,
          }}
        >
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              px: 2,
              py: 1.5,
              bgcolor: 'action.hover',
            }}
          >
            <Typography variant="subtitle1">{selectedPipeline.name}</Typography>
            {selectedPipeline.isDefault ? (
              <Chip
                label="Default"
                size="small"
                color="primary"
                variant="outlined"
                sx={{ ml: 1 }}
              />
            ) : null}
          </Box>
          <Box sx={{ px: 2, py: 2 }}>
            <Stack direction="row" alignItems="center" spacing={1.5} useFlexGap flexWrap="wrap">
              {selectedPipeline.stages.map((stage, index) => {
                const environment = environments.find((item) => item.id === stage.environmentId);
                const gateway = environment?.gateways.find(
                  (item) => item.id === stage.defaultGatewayId
                );
                return (
                  <Box
                    key={stage.id}
                    sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}
                  >
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
            </Stack>
          </Box>
        </Box>
      ) : (
        <Box
          sx={{
            border: '1px dashed',
            borderColor: 'divider',
            borderRadius: 1.5,
            py: 6,
            mb: 3,
            textAlign: 'center',
          }}
        >
          <Typography variant="body2" color="text.secondary">
            No organization pipelines are available.
          </Typography>
        </Box>
      )}

      <Stack direction="row" spacing={1.5}>
        <Button
          variant="outlined"
          disabled={!hasChanges || saving}
          onClick={() => setSelectedPipelineId(currentPipelineId)}
        >
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={!selectedPipelineId || !hasChanges || saving}
          startIcon={saving ? <CircularProgress size={16} color="inherit" /> : undefined}
          onClick={() => onAssociate([selectedPipelineId])}
        >
          Save
        </Button>
      </Stack>
    </PageContent>
  );
};

export default ProjectPipelinesPage;
