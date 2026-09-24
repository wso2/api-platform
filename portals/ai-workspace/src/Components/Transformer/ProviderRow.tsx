/**
 * One attached provider, collapsed to a row.
 *
 * Used once a proxy has more than one provider, or while one is being added:
 * the providers already settled shrink to rows so the form being filled in is
 * the only thing expanded.
 *
 * The row carries what distinguishes one provider from another — its name,
 * whether it is the primary, and what translates for it — and nothing else. The
 * rest is behind the configure action.
 */

import React from 'react';
import { FormattedMessage } from 'react-intl';
import { Box, Button, Card, Stack, TextField, Typography } from '@wso2/oxygen-ui';
import { Circle, CircleDot, PenLine, X } from '@wso2/oxygen-ui-icons-react';
import type { TransformerResolution } from '../../utils/transformerResolution';

export type ProviderRowProps = {
  displayName: string;
  /**
   * What a client puts in a routing header to select this provider. Shown
   * alongside the translator, because a row that names a provider without it
   * does not tell a reader how to reach that provider.
   */
  requestHandle?: string;
  /**
   * `field` while a provider is being chosen, where the row sits among inputs
   * and reads as one. `plain` on a settled list, where it is a record rather
   * than a control.
   */
  variant?: 'field' | 'plain';
  isPrimary: boolean;
  resolution: TransformerResolution;
  /** Shown only once the proxy has more than one provider to choose between. */
  showPrimaryToggle: boolean;
  onMakePrimary?: () => void;
  onEdit?: () => void;
  onRemove?: () => void;
  /** Offered where translation is needed but nothing provides it. */
  onConfigureTransformer?: () => void;
  disabled?: boolean;
  'data-cyid'?: string;
};

export default function ProviderRow({
  displayName,
  requestHandle,
  variant = 'field',
  isPrimary,
  resolution,
  showPrimaryToggle,
  onMakePrimary,
  onEdit,
  onRemove,
  onConfigureTransformer,
  disabled = false,
  'data-cyid': dataCyId = 'provider-row',
}: ProviderRowProps) {
  const needsConfiguring = resolution.status === 'unresolved' || resolution.status === 'invalid';
  const isPlain = variant === 'plain';

  return (
    <Card
      variant="outlined"
      sx={{ p: 1.5, bgcolor: isPlain ? 'background.paper' : 'action.hover' }}
      data-cyid={dataCyId}
    >
      <Stack spacing={0.75}>
        <Box display="flex" alignItems="center" gap={1}>
          {/* Read-only: which provider this is was settled when it was added. */}
          {isPlain ? (
            <Typography
              variant="body2"
              sx={{ fontWeight: 600, flex: 1 }}
              data-cyid={`${dataCyId}-name`}
            >
              {displayName}
            </Typography>
          ) : (
            <TextField
              fullWidth
              size="small"
              value={displayName}
              slotProps={{ input: { readOnly: true } }}
              sx={{ bgcolor: 'background.paper' }}
              data-cyid={`${dataCyId}-name`}
            />
          )}
          {showPrimaryToggle && (
            <Button
              size="small"
              variant="outlined"
              color={isPrimary ? 'primary' : 'secondary'}
              startIcon={isPrimary ? <CircleDot size={16} /> : <Circle size={16} />}
              onClick={onMakePrimary}
              disabled={disabled || isPrimary}
              sx={{
                borderRadius: 999,
                flexShrink: 0,
                // On the provider that already is the primary this reads as a
                // state rather than an action, so it keeps its colour: greyed
                // out it says "unavailable", which is the opposite of what
                // marking the primary means. A row disabled because a form is
                // open elsewhere still greys with everything else.
                ...(isPrimary && !disabled
                  ? {
                      '&.Mui-disabled': {
                        color: 'primary.main',
                        borderColor: 'primary.main',
                      },
                    }
                  : {}),
              }}
              data-cyid={`${dataCyId}-primary`}
            >
              <FormattedMessage
                id="aiWorkspace.components.providerRow.primary"
                defaultMessage="PRIMARY"
              />
            </Button>
          )}
          <Button
            size="small"
            variant="outlined"
            onClick={onEdit}
            disabled={disabled}
            sx={{ minWidth: 0, px: 1, flexShrink: 0 }}
            data-cyid={`${dataCyId}-edit`}
          >
            <PenLine size={16} />
          </Button>
          <Button
            size="small"
            variant="outlined"
            onClick={onRemove}
            disabled={disabled || !onRemove}
            sx={{ minWidth: 0, px: 1, flexShrink: 0 }}
            data-cyid={`${dataCyId}-remove`}
          >
            <X size={16} />
          </Button>
        </Box>

        {/*
          What translates for this provider, in one line. Nothing is shown for a
          provider still being chosen; the matching case says so plainly rather
          than leaving a gap that could equally mean "still loading".
        */}
        {(resolution.title || requestHandle) && (
          <Box display="flex" alignItems="center" gap={1} sx={{ pl: 0.5 }}>
            {resolution.status === 'none' ? (
              // Only where the formats were actually compared. A provider
              // whose template has not loaded resolves to nothing at all, and
              // announcing "no transformer needed" for it states a conclusion
              // nothing has reached.
              resolution.title ? (
                <Typography variant="caption" color="info.main">
                  <FormattedMessage
                    id="aiWorkspace.components.providerRow.noTransformerNeeded"
                    defaultMessage="no transformer needed"
                  />
                </Typography>
              ) : null
            ) : resolution.policy ? (
              <>
                {/*
                  Named, and nothing more. How a translator came to be attached
                  is not recorded anywhere, so any account of it would be a
                  guess — but anything true about it, such as its being
                  attached where nothing needs translating, is said here.
                */}
                <Typography
                  variant="caption"
                  color="success.main"
                  sx={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}
                >
                  {resolution.policy.name}
                </Typography>
                {resolution.note && (
                  <Typography variant="caption" color="warning.main">
                    {resolution.note}
                  </Typography>
                )}
              </>
            ) : resolution.title ? (
              <>
                <Typography variant="caption" color="warning.main">
                  {resolution.title}
                </Typography>
                {needsConfiguring && onConfigureTransformer && (
                  <Button
                    size="small"
                    onClick={onConfigureTransformer}
                    disabled={disabled}
                    sx={{ px: 0.5, minHeight: 0 }}
                    data-cyid={`${dataCyId}-configure-transformer`}
                  >
                    <FormattedMessage
                      id="aiWorkspace.components.providerRow.configureTransformer"
                      defaultMessage="Configure transformer"
                    />
                  </Button>
                )}
              </>
            ) : null}
            {/*
              The handle trails the translator rather than leading it: what a
              provider needs translating is the thing that differs between
              rows, and the handle is what a reader copies once they have found
              the row they want.
            */}
            {requestHandle && (
              <>
                {resolution.title && (
                  <Typography variant="caption" color="text.disabled">
                    ·
                  </Typography>
                )}
                <Typography
                  variant="caption"
                  sx={{
                    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
                  }}
                  data-cyid={`${dataCyId}-handle`}
                >
                  {requestHandle}
                </Typography>
              </>
            )}
          </Box>
        )}
      </Stack>
    </Card>
  );
}
