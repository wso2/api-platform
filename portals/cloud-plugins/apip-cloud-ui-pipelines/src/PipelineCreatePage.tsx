/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useRef, useState, type FC } from 'react';
import {
  Box,
  Button,
  IconButton,
  PageContent,
  PageTitle,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ArrowRight, ChevronLeft, Plus } from '@wso2/oxygen-ui-icons-react';
import EnvironmentGatewayPicker from './components/EnvironmentGatewayPicker';
import PipelineStageCard from './components/PipelineStageCard';
import type { CreatePipelineInput, Environment, Pipeline, PipelineStage } from './types';

export type PipelineCreatePageProps = {
  environments: Environment[];
  /** Editing an existing pipeline pre-fills name/stages and changes the page's labels. Omit (or 'create') for a blank pipeline. */
  mode?: 'create' | 'edit';
  initialPipeline?: Pipeline;
  onBack: () => void;
  /** `pipelineId` is set only in edit mode, so the caller can route to createPipeline vs updatePipeline without this page knowing which store function to call. */
  onSubmit: (input: CreatePipelineInput, pipelineId?: string) => void;
};

const findEnvironment = (environments: Environment[], environmentId: string) =>
  environments.find((environment) => environment.id === environmentId);

const findGateway = (environment: Environment | undefined, gatewayId: string) =>
  environment?.gateways.find((gateway) => gateway.id === gatewayId);

const PipelineCreatePage: FC<PipelineCreatePageProps> = ({
  environments,
  mode = 'create',
  initialPipeline,
  onBack,
  onSubmit,
}) => {
  const isEdit = mode === 'edit' && !!initialPipeline;
  const [name, setName] = useState(initialPipeline?.name ?? '');
  const isDefault = initialPipeline?.isDefault ?? false;
  const [stages, setStages] = useState<PipelineStage[]>(initialPipeline?.stages ?? []);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [pickerAnchor, setPickerAnchor] = useState<HTMLElement | null>(null);
  // Position a new stage is inserted at; null means append to the end. A ref
  // (not state) because it's only ever read once, synchronously, inside
  // handleAddStage — no re-render should depend on it.
  const insertIndexRef = useRef<number | null>(null);

  const openPickerAt = (anchor: HTMLElement, insertIndex: number | null) => {
    insertIndexRef.current = insertIndex;
    setPickerAnchor(anchor);
    setPickerOpen(true);
  };

  const handleAddStage = (environmentId: string, defaultGatewayId: string) => {
    const newStage: PipelineStage = { id: crypto.randomUUID(), environmentId, defaultGatewayId };
    setStages((prev) => {
      const index = insertIndexRef.current;
      if (index === null || index >= prev.length) return [...prev, newStage];
      return [...prev.slice(0, index), newStage, ...prev.slice(index)];
    });
  };

  const handleRemoveStage = (stageId: string) => {
    setStages((prev) => prev.filter((stage) => stage.id !== stageId));
  };

  const canSubmit = name.trim().length > 0 && stages.length > 0;

  return (
    <PageContent fullWidth>
      <Button size="small" startIcon={<ChevronLeft size={18} />} onClick={onBack}>
        Back to Deployment Pipelines
      </Button>

      <Stack spacing={2} mt={2}>
        <PageTitle>
          <PageTitle.Header>{isEdit ? 'Edit Pipeline' : 'Create a New Pipeline'}</PageTitle.Header>
        </PageTitle>
      </Stack>

      <Box sx={{ mt: 2, maxWidth: 880 }}>
        <Box sx={{ maxWidth: 420, mb: 2 }}>
          <Typography variant="body2" sx={{ mb: 0.5 }}>
            Name
          </Typography>
          <TextField
            fullWidth
            placeholder="Enter pipeline name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            autoFocus
          />
        </Box>

        <Box
          sx={{
            bgcolor: 'action.hover',
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 1.5,
            p: 3,
            minHeight: 160,
            display: 'flex',
            alignItems: 'center',
            justifyContent: stages.length === 0 ? 'center' : 'flex-start',
            flexWrap: 'wrap',
            gap: 1.5,
          }}
        >
          {stages.length === 0 ? (
            <Button
              variant="contained"
              startIcon={<Plus size={16} />}
              onClick={(event) => openPickerAt(event.currentTarget, null)}
            >
              Add Environment
            </Button>
          ) : (
            <>
              {stages.map((stage, index) => {
                const environment = findEnvironment(environments, stage.environmentId);
                const gateway = findGateway(environment, stage.defaultGatewayId);
                return (
                  <Box key={stage.id} sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
                    {index > 0 ? (
                      <Tooltip title="Insert environment here">
                        <IconButton
                          size="small"
                          aria-label="Insert environment here"
                          onClick={(event) => openPickerAt(event.currentTarget, index)}
                          sx={{ color: 'text.secondary' }}
                        >
                          <ArrowRight size={18} />
                        </IconButton>
                      </Tooltip>
                    ) : null}
                    <PipelineStageCard
                      environmentName={environment?.name ?? stage.environmentId}
                      gatewayName={gateway?.name ?? stage.defaultGatewayId}
                      critical={environment?.critical}
                      onRemove={() => handleRemoveStage(stage.id)}
                    />
                  </Box>
                );
              })}
              <Tooltip title="Add next environment">
                <IconButton
                  color="primary"
                  onClick={(event) => openPickerAt(event.currentTarget, null)}
                  sx={{
                    bgcolor: 'primary.main',
                    color: 'primary.contrastText',
                    '&:hover': { bgcolor: 'primary.dark' },
                  }}
                >
                  <Plus size={20} />
                </IconButton>
              </Tooltip>
            </>
          )}

          <EnvironmentGatewayPicker
            open={pickerOpen}
            anchorEl={pickerAnchor}
            environments={environments}
            usedStages={stages}
            onClose={() => setPickerOpen(false)}
            onAdd={handleAddStage}
          />
        </Box>

        <Box sx={{ mt: 3, display: 'flex', gap: 1 }}>
          <Button variant="outlined" color="secondary" onClick={onBack}>
            Cancel
          </Button>
          <Tooltip
            title={!canSubmit
              ? name.trim().length === 0
                ? 'Enter a pipeline name to continue.'
                : 'Add at least one environment to the pipeline.'
              : ''}
          >
            <span>
              <Button
                variant="contained"
                disabled={!canSubmit}
                onClick={() => onSubmit({ name: name.trim(), isDefault, stages }, initialPipeline?.id)}
              >
                {isEdit ? 'Save Changes' : 'Create'}
              </Button>
            </span>
          </Tooltip>
        </Box>
      </Box>
    </PageContent>
  );
};

export default PipelineCreatePage;
