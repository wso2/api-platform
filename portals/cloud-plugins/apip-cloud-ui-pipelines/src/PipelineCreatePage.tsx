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
import type { CreatePipelineInput, Environment, Pipeline } from './types';
import { orderEnvironments } from './utils';

export type PipelineCreatePageProps = {
  environments: Environment[];
  /** Editing an existing pipeline pre-fills name/environments and changes the page's labels. Omit (or 'create') for a blank pipeline. */
  mode?: 'create' | 'edit';
  initialPipeline?: Pipeline;
  onBack: () => void;
  /** `pipelineId` is set only in edit mode, so the caller can route to create vs update without this page knowing the transport. Returning a promise lets this page disable the button until the save settles. */
  onSubmit: (input: CreatePipelineInput, pipelineId?: string) => void | Promise<void>;
};

/**
 * The linear chain the builder edits: environments in promotion order, each with
 * the gateway marked as its default. This is the builder's own working view over
 * the API shape — on submit it emits `promotionPaths` (consecutive pairs) and
 * `defaultGateways` directly; nothing else converts pipeline data.
 */
type ChainEntry = {
  /** Environment name — the identifier the API uses everywhere. */
  environment: string;
  defaultGatewayId: string;
};

const findEnvironment = (environments: Environment[], name: string) =>
  environments.find((environment) => environment.name === name);

const findGateway = (environment: Environment | undefined, gatewayId: string) =>
  environment?.gateways.find((gateway) => gateway.id === gatewayId);

/** Reconstructs the builder's chain from an existing pipeline's promotion graph. */
const toChain = (pipeline: Pipeline, environments: Environment[]): ChainEntry[] =>
  orderEnvironments(pipeline.promotionPaths).map((name) => {
    const environment = findEnvironment(environments, name);
    const marked = pipeline.defaultGateways.find((entry) => entry.environment === name)?.gatewayId;
    return {
      environment: name,
      defaultGatewayId: marked ?? (environment?.gateways.length === 1 ? environment.gateways[0].id : ''),
    };
  });

const PipelineCreatePage: FC<PipelineCreatePageProps> = ({
  environments,
  mode = 'create',
  initialPipeline,
  onBack,
  onSubmit,
}) => {
  const isEdit = mode === 'edit' && !!initialPipeline;
  const [name, setName] = useState(initialPipeline?.name ?? '');
  const [chain, setChain] = useState<ChainEntry[]>(
    initialPipeline ? toChain(initialPipeline, environments) : []
  );
  const [pickerOpen, setPickerOpen] = useState(false);
  const [pickerAnchor, setPickerAnchor] = useState<HTMLElement | null>(null);
  const [saving, setSaving] = useState(false);
  // Position a new environment is inserted at; null means append to the end. A
  // ref (not state) because it's only read once, synchronously, inside
  // handleAddEnvironment — no re-render should depend on it.
  const insertIndexRef = useRef<number | null>(null);

  const openPickerAt = (anchor: HTMLElement, insertIndex: number | null) => {
    insertIndexRef.current = insertIndex;
    setPickerAnchor(anchor);
    setPickerOpen(true);
  };

  const handleAddEnvironment = (environment: string, defaultGatewayId: string) => {
    const entry: ChainEntry = { environment, defaultGatewayId };
    setChain((prev) => {
      const index = insertIndexRef.current;
      if (index === null || index >= prev.length) return [...prev, entry];
      return [...prev.slice(0, index), entry, ...prev.slice(index)];
    });
  };

  const handleRemoveEnvironment = (environment: string) => {
    setChain((prev) => prev.filter((entry) => entry.environment !== environment));
  };

  // A pipeline needs at least two environments to form a promotion path
  // (source -> target), which is what the platform-api requires.
  const canSubmit = name.trim().length > 0 && chain.length >= 2;

  const handleSubmit = async () => {
    // Guard against a second click issuing a duplicate create/update while the
    // first request is still in flight.
    if (saving) return;
    // The linear chain is emitted as the API shape directly: consecutive pairs
    // become promotion paths, and only multi-gateway environments carry an
    // explicit default (single-gateway environments default implicitly).
    const promotionPaths = chain.slice(0, -1).map((entry, index) => ({
      sourceEnvironment: entry.environment,
      targetEnvironments: [chain[index + 1].environment],
    }));
    const defaultGateways = chain
      .filter((entry) => {
        const environment = findEnvironment(environments, entry.environment);
        return !!environment && environment.gateways.length > 1 && !!entry.defaultGatewayId;
      })
      .map((entry) => ({ environment: entry.environment, gatewayId: entry.defaultGatewayId }));
    setSaving(true);
    try {
      await onSubmit({ name: name.trim(), promotionPaths, defaultGateways }, initialPipeline?.id);
    } finally {
      setSaving(false);
    }
  };

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
            disabled={isEdit}
            helperText={isEdit ? 'A pipeline cannot be renamed after it is created.' : undefined}
            autoFocus={!isEdit}
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
            justifyContent: chain.length === 0 ? 'center' : 'flex-start',
            flexWrap: 'wrap',
            gap: 1.5,
          }}
        >
          {chain.length === 0 ? (
            <Button
              variant="contained"
              startIcon={<Plus size={16} />}
              onClick={(event) => openPickerAt(event.currentTarget, null)}
            >
              Add Environment
            </Button>
          ) : (
            <>
              {chain.map((entry, index) => {
                const environment = findEnvironment(environments, entry.environment);
                const gateway = findGateway(environment, entry.defaultGatewayId);
                return (
                  <Box key={entry.environment} sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
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
                      environmentName={environment?.name ?? entry.environment}
                      gatewayName={gateway?.name ?? entry.defaultGatewayId}
                      critical={environment?.critical}
                      onRemove={() => handleRemoveEnvironment(entry.environment)}
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
            usedEnvironments={chain.map((entry) => entry.environment)}
            onClose={() => setPickerOpen(false)}
            onAdd={handleAddEnvironment}
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
                : 'Add at least two environments to the pipeline.'
              : ''}
          >
            <span>
              <Button variant="contained" disabled={!canSubmit || saving} onClick={handleSubmit}>
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
