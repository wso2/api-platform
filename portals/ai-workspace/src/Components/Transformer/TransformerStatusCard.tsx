/**
 * How a provider's translator is reported, everywhere it is reported.
 *
 * The create form, the provider drawer and the compact rows all render this, so
 * the same provider reads identically wherever it appears. A screen composing
 * its own wording is a screen that will eventually word it differently.
 *
 * A provider that needs no translation says so, rather than showing an empty
 * space that could equally mean "still loading".
 */

import React from 'react';
import { FormattedMessage } from 'react-intl';
import { Box, Button, Card, Chip, Stack, Typography } from '@wso2/oxygen-ui';
import {
  CircleCheck,
  Info,
  SlidersHorizontal,
  TriangleAlert,
  X,
} from '@wso2/oxygen-ui-icons-react';
import type { TransformerResolution } from '../../utils/transformerResolution';
import type { ProxyProviderTransformer } from '../../utils/types';

export type TransformerStatusCardProps = {
  resolution: TransformerResolution;
  /** What is actually stored, so unset parameters can be reported. */
  transformer?: ProxyProviderTransformer | null;
  /** Opens the transformer picker. Omitted where the surface cannot configure. */
  onConfigure?: () => void;
  /**
   * Detaches what is attached. Offered beside the configure action, and only
   * where something is actually attached — a match that is merely implied by
   * the two formats is not a thing a reader can take off.
   */
  onRemove?: () => void;
  disabled?: boolean;
  'data-cyid'?: string;
};

/**
 * Parameters the policy declares but the configuration does not set.
 *
 * Reported, never enforced. An unset parameter is a working configuration: the
 * value the request already carries is used, so setting one overrides the
 * request rather than completing the setup. Treating it as incomplete would gate
 * on a state that runs perfectly well.
 */
const unsetParameterNames = (
  resolution: TransformerResolution,
  transformer?: ProxyProviderTransformer | null
): string[] => {
  const declared = resolution.policy?.parameters ?? [];
  const configured = transformer?.params ?? {};
  return declared
    .filter((parameter) => {
      const value = configured[parameter.name];
      return value === undefined || value === null || value === '';
    })
    .map((parameter) => parameter.name);
};

export default function TransformerStatusCard({
  resolution,
  transformer,
  onConfigure,
  onRemove,
  disabled = false,
  'data-cyid': dataCyId = 'transformer-status',
}: TransformerStatusCardProps) {
  if (!resolution.title) {
    return null;
  }

  const isWarning = resolution.tone === 'warning';
  const isInfo = resolution.tone === 'info';
  const unsetParameters = unsetParameterNames(resolution, transformer);

  return (
    <Stack spacing={0.5} data-cyid={dataCyId}>
      <Typography variant="caption" color="text.secondary">
        <FormattedMessage
          id="aiWorkspace.components.transformer.label"
          defaultMessage="Transformer"
        />
      </Typography>
      <Card
        variant="outlined"
        sx={{
          p: 1.5,
          borderColor: isWarning
            ? 'warning.main'
            : isInfo
              ? 'info.main'
              : 'success.main',
          bgcolor: isWarning
            ? 'warning.lighter'
            : isInfo
              ? 'info.lighter'
              : 'success.lighter',
        }}
      >
        <Box display="flex" alignItems="flex-start" justifyContent="space-between" gap={1}>
          <Box display="flex" alignItems="flex-start" gap={1}>
            <Box
              component="span"
              sx={{
                display: 'flex',
                mt: 0.25,
                color: isWarning
                  ? 'warning.main'
                  : isInfo
                    ? 'info.main'
                    : 'success.main',
              }}
            >
              {isWarning ? (
                <TriangleAlert size={16} />
              ) : isInfo ? (
                <Info size={16} />
              ) : (
                <CircleCheck size={16} />
              )}
            </Box>
            <Stack spacing={0.25}>
              <Typography
                variant="body2"
                sx={{ fontWeight: 600 }}
                data-cyid={`${dataCyId}-title`}
              >
                {resolution.title}
              </Typography>
              {resolution.detail && (
                <Typography
                  variant="caption"
                  color="text.secondary"
                  data-cyid={`${dataCyId}-detail`}
                >
                  {resolution.detail}
                </Typography>
              )}
              {resolution.note && (
                <Typography
                  variant="caption"
                  color="warning.main"
                  data-cyid={`${dataCyId}-note`}
                >
                  {resolution.note}
                </Typography>
              )}
            </Stack>
          </Box>
          {/*
            A resolved policy is adjusted from a quiet icon; an unresolved one
            needs an action a reader can find, so it gets a labelled button
            below the explanation instead.
          */}
          <Box display="flex" alignItems="center" gap={0.5} sx={{ flexShrink: 0 }}>
            {/*
              Offered even where nothing needs translating. That a provider
              speaks the proxy's format settles what is required, not what is
              allowed — someone may still want a translator in the path, and a
              card that only reports leaves them nowhere to say so.
            */}
            {onConfigure && resolution.status !== 'unresolved' && (
              <Button
                size="small"
                variant="outlined"
                onClick={onConfigure}
                disabled={disabled}
                sx={{ minWidth: 0, px: 1 }}
                data-cyid={`${dataCyId}-configure`}
              >
                <SlidersHorizontal size={16} />
              </Button>
            )}
            {onRemove && transformer?.type && (
              <Button
                size="small"
                variant="outlined"
                color="error"
                onClick={onRemove}
                disabled={disabled}
                sx={{ minWidth: 0, px: 1 }}
                data-cyid={`${dataCyId}-remove`}
              >
                <X size={16} />
              </Button>
            )}
          </Box>
        </Box>
        {onConfigure && resolution.status === 'unresolved' && (
          <Box sx={{ display: 'flex', justifyContent: 'flex-end', mt: 1 }}>
            <Button
              size="small"
              variant="outlined"
              startIcon={<SlidersHorizontal size={16} />}
              onClick={onConfigure}
              disabled={disabled}
              data-cyid={`${dataCyId}-configure`}
            >
              <FormattedMessage
                id="aiWorkspace.components.transformer.configureTransformer"
                defaultMessage="Configure transformer"
              />
            </Button>
          </Box>
        )}
        {unsetParameters.length > 0 && (
          <Box display="flex" flexWrap="wrap" gap={0.5} sx={{ mt: 1 }}>
            {unsetParameters.map((name) => (
              <Chip
                key={name}
                size="small"
                variant="outlined"
                data-cyid={`${dataCyId}-unset-${name}`}
                label={
                  <FormattedMessage
                    id="aiWorkspace.components.transformer.notConfigured"
                    defaultMessage="{name} not configured"
                    values={{ name }}
                  />
                }
              />
            ))}
          </Box>
        )}
      </Card>
    </Stack>
  );
}
