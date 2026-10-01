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

/**
 * Every provider a proxy routes to, one per row.
 *
 * A table rather than a stack of cards because the interesting question is a
 * comparison — which of these translates, which is primary, which is missing
 * something — and a column answers that at a glance where a card has to be read
 * one at a time.
 */

import React from 'react';
import { FormattedMessage, useIntl } from 'react-intl';
import {
  Alert,
  Box,
  Button,
  IconButton,
  ListingTable,
  Skeleton,
  Switch,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { PenSquare, Plus, Trash2 } from '@wso2/oxygen-ui-icons-react';
import GuardrailPill from '../GuardrailPill/GuardrailPill';
import type { TransformerResolution } from '../../utils/transformerResolution';
import type { ProxyProviderEntry } from '../../utils/types';

export type ProviderTableProps = {
  entries: ProxyProviderEntry[];
  /** The name a reader recognises. */
  displayNameFor: (entry: ProxyProviderEntry) => string;
  /** The value a client puts in a routing header to select this provider. */
  handleFor: (entry: ProxyProviderEntry) => string;
  resolutionFor: (entry: ProxyProviderEntry) => TransformerResolution;
  onMakePrimary: (providerId: string) => void;
  onEdit: (index: number) => void;
  /** Withheld on the last remaining row: a proxy always has a provider. */
  onRemove?: (providerId: string) => void;
  /**
   * Whether a particular row may be removed at all. Some surfaces hold the
   * primary in a slot of its own, where there is no such thing as removing it.
   */
  canRemove?: (entry: ProxyProviderEntry, index: number) => boolean;
  onConfigureTransformer: (index: number) => void;
  /** Detaches the translator from the row it belongs to. */
  onRemoveTransformer: (index: number) => void;
  disabled?: boolean;
};

/**
 * The version to show beside a translator's name.
 *
 * What the proxy stored wins over what the catalogue lists: the stored major is
 * what a gateway resolves, so it is the one that describes what will actually
 * run. Written bare, since the pill parenthesises it.
 */
const versionLabel = (
  entry: ProxyProviderEntry,
  resolution: TransformerResolution
): string => {
  const raw = entry.transformer?.version || resolution.policy?.version || '';
  return raw.trim().replace(/^v/i, '');
};

/**
 * The translator as a reader recognises it: the catalogue's display name, or
 * its own handle where the catalogue no longer carries it.
 */
const transformerName = (
  entry: ProxyProviderEntry,
  resolution: TransformerResolution
): string =>
  resolution.policy?.displayName || entry.transformer?.type || '';

export default function ProviderTable({
  entries,
  displayNameFor,
  handleFor,
  resolutionFor,
  onMakePrimary,
  onEdit,
  onRemove,
  canRemove,
  onConfigureTransformer,
  onRemoveTransformer,
  disabled = false,
}: ProviderTableProps) {
  const intl = useIntl();
  // Counted rather than merely coloured: a proxy that will turn traffic away is
  // worth saying once at the top, where it is read before the table is scanned.
  const unservable = entries.filter(
    (entry) => resolutionFor(entry).status === 'unresolved'
  ).length;

  /**
   * What the transformer column shows for one provider.
   *
   * One control in every state. What is waiting there differs — pick a
   * translator, adjust the one attached, or add one nothing requires — but the
   * thing a reader clicks does not, so a column of four providers reads as one
   * question asked four times rather than four different widgets.
   */
  const renderTransformerCell = (
    entry: ProxyProviderEntry,
    index: number,
    cyPrefix: string
  ) => {
    const resolution = resolutionFor(entry);

    // The catalogue has not answered yet, so what translates for this provider
    // is not yet knowable. Held as a shape the size of the answer rather than a
    // word: the row is waiting, not reporting, and a label reads as a verdict.
    if (resolution.status === 'unknown') {
      return (
        <Skeleton
          variant="rounded"
          width={180}
          height={34}
          data-cyid={`${cyPrefix}-transformer-loading`}
        />
      );
    }

    const attached = Boolean(entry.transformer?.type || resolution.policy);
    // The one state that is waiting on someone: the formats differ and nothing
    // translates between them, so the provider turns traffic away until it does.
    const needsAttention =
      resolution.status === 'unresolved' || resolution.status === 'invalid';
    const version = versionLabel(entry, resolution);

    // Named and versioned the way the guardrails tab writes a policy pill, so
    // the two lists of attached things read alike: "Display Name (0)".
    const label = attached
      ? `${transformerName(entry, resolution)}${version ? ` (${version})` : ''}`
      : intl.formatMessage(
          needsAttention
            ? {
                id: 'aiWorkspace.components.providerTable.required',
                defaultMessage: 'Required',
              }
            : {
                // Nothing needed, and still offered: matching formats settle
                // what is required, not what is allowed.
                id: 'aiWorkspace.components.providerTable.notRequired',
                defaultMessage: 'Not required',
              }
        );

    // Attached: the same card the guardrails tab uses for a policy, because it
    // is the same idea — a thing on the proxy, opened to adjust and crossed off
    // to detach. Changing one is removing it and adding another.
    if (attached) {
      return (
        <GuardrailPill
          label={label}
          onClick={() => onConfigureTransformer(index)}
          onRemove={disabled ? undefined : () => onRemoveTransformer(index)}
          removeAriaLabel={intl.formatMessage({
            id: 'aiWorkspace.components.providerTable.removeTransformer',
            defaultMessage: 'Remove transformer',
          })}
          data-cyid={`${cyPrefix}-transformer`}
        />
      );
    }

    // Nothing attached. What the row reports is a fact about the provider, not
    // a thing to act on, so it is stated rather than boxed — and the action
    // beside it is the one there is.
    return (
      <Box display="flex" alignItems="center" gap={1.5}>
        <Typography
          variant="body2"
          color={needsAttention ? 'error.main' : 'info.main'}
        >
          {label}
        </Typography>
        <Button
          variant="outlined"
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() => onConfigureTransformer(index)}
          disabled={disabled}
          sx={{ borderRadius: 999, textTransform: 'none' }}
          data-cyid={`${cyPrefix}-transformer-add`}
        >
          <FormattedMessage
            id="aiWorkspace.components.providerTable.add"
            defaultMessage="Add"
          />
        </Button>
      </Box>
    );
  };

  return (
    <>
      {unservable > 0 && (
        <Alert severity="warning" data-cyid="providers-transformer-warning">
          <FormattedMessage
            id="aiWorkspace.components.providerTable.unservable"
            defaultMessage="{count, plural, one {# provider requires} other {# providers require}} a transformer policy before it can receive traffic."
            values={{ count: unservable }}
          />
        </Alert>
      )}

      <ListingTable.Container>
        {/*
          Fixed layout, so a long provider id cannot widen its own column and
          squeeze the one beside it. The two columns that carry something to
          read share the width evenly; the switch and the two icons take only
          what they need.
        */}
        <ListingTable
          sx={{ tableLayout: 'fixed' }}
          data-cyid="proxy-provider-list"
        >
          {/*
            The default light-mode head tint is grey[50], which against a white
            paper is all but invisible — the header row reads as another row of
            data. Deepened one step here, in light only: the dark rule already
            separates the two and is left as it is.
          */}
          <ListingTable.Head
            sx={(theme) => ({
              '& .MuiTableCell-head': theme.applyStyles('light', {
                backgroundColor: theme.palette.grey[200],
              }),
            })}
          >
            <ListingTable.Row>
              <ListingTable.Cell sx={{ width: '34%' }}>
                <FormattedMessage
                  id="aiWorkspace.components.providerTable.provider"
                  defaultMessage="Provider"
                />
              </ListingTable.Cell>
              <ListingTable.Cell sx={{ width: '34%' }}>
                <FormattedMessage
                  id="aiWorkspace.components.providerTable.transformerPolicy"
                  defaultMessage="Transformer policy"
                />
              </ListingTable.Cell>
              <ListingTable.Cell align="center" sx={{ width: '16%' }}>
                <FormattedMessage
                  id="aiWorkspace.components.providerTable.primary"
                  defaultMessage="Primary"
                />
              </ListingTable.Cell>
              <ListingTable.Cell align="center" sx={{ width: '16%' }}>
                <FormattedMessage
                  id="aiWorkspace.components.providerTable.actions"
                  defaultMessage="Actions"
                />
              </ListingTable.Cell>
            </ListingTable.Row>
          </ListingTable.Head>
          <ListingTable.Body>
            {entries.map((entry, index) => {
              const cyPrefix = `provider-row-${index}`;
              return (
                <ListingTable.Row
                  key={`${entry.id}-${index}`}
                  data-cyid={cyPrefix}
                >
                  <ListingTable.Cell>
                    <Typography variant="body2" sx={{ fontWeight: 600 }}>
                      {displayNameFor(entry)}
                    </Typography>
                    {/*
                      The handle is the alias where there is one and the id
                      otherwise — either way the value a client puts in a
                      routing header. A row showing something that does not
                      route would be worse than showing nothing.
                    */}
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      sx={{
                        fontFamily:
                          'ui-monospace, SFMono-Regular, Menlo, monospace',
                        // Wrapped, never clipped: this is the value a client
                        // puts in a routing header, so half of it is worse
                        // than a taller row.
                        display: 'block',
                        overflowWrap: 'anywhere',
                      }}
                      data-cyid={`${cyPrefix}-handle`}
                    >
                      {handleFor(entry)}
                    </Typography>
                  </ListingTable.Cell>

                  <ListingTable.Cell>
                    {renderTransformerCell(entry, index, cyPrefix)}
                  </ListingTable.Cell>

                  <ListingTable.Cell align="center">
                    {/*
                      A proxy always has a primary, so the one that is cannot be
                      switched off — only another can be switched on. That is
                      enforced by ignoring the change rather than by disabling
                      the switch: greying out the primary reads as "unavailable",
                      which is the opposite of what being the primary means.
                    */}
                    <Switch
                      size="small"
                      checked={Boolean(entry.isPrimary)}
                      onChange={() => {
                        if (!entry.isPrimary) {
                          onMakePrimary(entry.id);
                        }
                      }}
                      disabled={disabled}
                      inputProps={{ 'aria-label': 'Primary provider' }}
                      data-cyid={`${cyPrefix}-primary`}
                    />
                  </ListingTable.Cell>

                  <ListingTable.Cell align="center">
                    <Box
                      display="flex"
                      alignItems="center"
                      justifyContent="center"
                      gap={0.5}
                    >
                      <Tooltip
                        title={
                          <FormattedMessage
                            id="aiWorkspace.components.providerTable.editProvider"
                            defaultMessage="Edit provider"
                          />
                        }
                      >
                        <span>
                          <IconButton
                            size="small"
                            onClick={() => onEdit(index)}
                            disabled={disabled}
                            data-cyid={`${cyPrefix}-edit`}
                          >
                            <PenSquare size={16} />
                          </IconButton>
                        </span>
                      </Tooltip>
                      <Tooltip
                        title={
                          <FormattedMessage
                            id="aiWorkspace.components.providerTable.removeProvider"
                            defaultMessage="Remove provider"
                          />
                        }
                      >
                        <span>
                          <IconButton
                            size="small"
                            color="error"
                            onClick={() => onRemove?.(entry.id)}
                            disabled={
                              disabled ||
                              !onRemove ||
                              canRemove?.(entry, index) === false
                            }
                            data-cyid={`${cyPrefix}-remove`}
                          >
                            <Trash2 size={16} />
                          </IconButton>
                        </span>
                      </Tooltip>
                    </Box>
                  </ListingTable.Cell>
                </ListingTable.Row>
              );
            })}
          </ListingTable.Body>
        </ListingTable>
      </ListingTable.Container>
    </>
  );
}
