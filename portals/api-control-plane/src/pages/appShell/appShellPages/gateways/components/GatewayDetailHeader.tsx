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

import { useState } from 'react';
import {
  Avatar,
  Button,
  Card,
  IconButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Clock, Network, Pencil } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useUpdateGateway, type Gateway } from '@/api/resources/gateways';
import { useNotifications } from '@/components/Notifications';
import { useFormatters } from '@/i18n/useFormatters';
import { gatewayInitials } from '../utils/gatewayDisplay';
import {
  GatewayFunctionalityChip,
  GatewayModeChip,
  GatewayStatusLabel,
  GatewayVersion,
} from './gatewayChips';

const messages = defineMessages({
  cancel: {
    id: 'gateways.detail.Header.edit.cancel',
    defaultMessage: 'Cancel',
    description: 'Discards unsaved edits to the gateway name and description.',
  },
  created: {
    id: 'gateways.detail.Header.created',
    defaultMessage: 'Created',
    description:
      'Label before the relative time a gateway was registered, e.g. "Created 1 month ago".',
  },
  descriptionLabel: {
    id: 'gateways.detail.Header.description.label',
    defaultMessage: 'Description',
    description: 'Label of the box used to edit the gateway description.',
  },
  descriptionPlaceholder: {
    id: 'gateways.detail.Header.description.placeholder',
    defaultMessage: 'No description',
    description:
      'Stands in for the gateway description when it has none. Rendered in italics as an absence, not a value.',
  },
  edit: {
    id: 'gateways.detail.Header.edit',
    defaultMessage: 'Edit name and description',
    description:
      'Accessible label and tooltip for the pencil that opens the gateway name and description for editing.',
  },
  nameLabel: {
    id: 'gateways.detail.Header.name.label',
    defaultMessage: 'Name',
    description: 'Label of the box used to edit the gateway name.',
  },
  nameRequired: {
    id: 'gateways.detail.Header.name.required',
    defaultMessage: 'Enter a gateway name.',
  },
  save: {
    id: 'gateways.detail.Header.edit.save',
    defaultMessage: 'Save',
    description: 'Commits edits to the gateway name and description.',
  },
  saved: {
    id: 'gateways.detail.Header.edit.saved',
    defaultMessage: 'Gateway details updated.',
    description: 'Confirmation shown after the gateway name or description is saved.',
  },
});

/** Edge of the identity tile and the monogram inside it. */
const AVATAR_SIZE = 72;
const AVATAR_FONT_SIZE = 28;
const AVATAR_ICON_SIZE = 32;

/** Keeps the editor from stretching across a wide screen. */
const EDIT_FIELD_MAX_WIDTH = 560;

/** What the pencil opens for editing. */
type Draft = {
  description: string;
  displayName: string;
};

export type GatewayDetailHeaderProps = {
  gateway: Gateway;
  gatewayId: string;
};

/**
 * Displays gateway identity and status with in-place editing of name and description.
 *
 * Name and description are edited together via a single control. Version,
 * functionality type, and status are read-only.
 */
export function GatewayDetailHeader({ gateway, gatewayId }: GatewayDetailHeaderProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const { dateTime, relativeTime } = useFormatters();
  const updateGateway = useUpdateGateway();

  // `null` is the read state. A draft object is the edit state, so opening the
  // editor and its contents are one piece of state rather than two that can
  // disagree.
  const [draft, setDraft] = useState<Draft | null>(null);
  const [nameTouched, setNameTouched] = useState(false);

  const displayName = gateway.displayName || gatewayId;
  const description = gateway.description?.trim() ?? '';
  // `updatedAt` is absent until the first edit, so a freshly registered gateway
  // still has a timestamp to show.
  const created = gateway.createdAt || gateway.updatedAt;

  const openEditor = () => {
    setNameTouched(false);
    setDraft({ description, displayName });
  };

  const closeEditor = () => {
    setNameTouched(false);
    setDraft(null);
  };

  const save = () => {
    if (!draft) return;

    const nextName = draft.displayName.trim();
    const nextDescription = draft.description.trim();

    if (nextName === '') {
      setNameTouched(true);
      return;
    }

    // Nothing changed: close rather than spend a request re-sending what the
    // server already holds.
    if (nextName === displayName && nextDescription === description) {
      closeEditor();
      return;
    }

    // The spec's update body is the whole gateway, so the fetched object is
    // spread back with only these two fields replaced.
    updateGateway.mutate(
      {
        body: { ...gateway, description: nextDescription, displayName: nextName },
        gatewayId,
      },
      {
        onSuccess: () => {
          closeEditor();
          notify(intl.formatMessage(messages.saved), 'success');
        },
      },
    );
  };

  const nameError = nameTouched && draft?.displayName.trim() === '';

  return (
    <Card sx={{ p: 2 }}>
      <Stack direction="row" spacing={2} sx={{ alignItems: 'flex-start' }}>
        <Avatar
          sx={{
            bgcolor: 'primary.light',
            color: 'primary.contrastText',
            flexShrink: 0,
            fontSize: AVATAR_FONT_SIZE,
            height: AVATAR_SIZE,
            width: AVATAR_SIZE,
          }}
          variant="rounded"
        >
          {gatewayInitials(displayName) || <Network size={AVATAR_ICON_SIZE} />}
        </Avatar>

        <Stack spacing={1} sx={{ flexGrow: 1, minWidth: 0 }}>
          {draft ? (
            <Stack spacing={1.5} sx={{ maxWidth: EDIT_FIELD_MAX_WIDTH }}>
              <TextField
                autoFocus
                disabled={updateGateway.isPending}
                error={nameError}
                fullWidth
                helperText={nameError ? intl.formatMessage(messages.nameRequired) : undefined}
                label={intl.formatMessage(messages.nameLabel)}
                onBlur={() => setNameTouched(true)}
                onChange={(event) => setDraft({ ...draft, displayName: event.target.value })}
                required
                size="small"
                value={draft.displayName}
              />
              <TextField
                disabled={updateGateway.isPending}
                fullWidth
                label={intl.formatMessage(messages.descriptionLabel)}
                maxRows={6}
                multiline
                onChange={(event) => setDraft({ ...draft, description: event.target.value })}
                size="small"
                value={draft.description}
              />
              <Stack direction="row" spacing={1}>
                <Button
                  disabled={updateGateway.isPending}
                  onClick={save}
                  size="small"
                  variant="contained"
                >
                  <FormattedMessage {...messages.save} />
                </Button>
                <Button disabled={updateGateway.isPending} onClick={closeEditor} size="small">
                  <FormattedMessage {...messages.cancel} />
                </Button>
              </Stack>
            </Stack>
          ) : (
            <>
              <Stack
                direction="row"
                spacing={1}
                sx={{ alignItems: 'center', flexWrap: 'wrap', minWidth: 0 }}
                useFlexGap
              >
                <Tooltip title={displayName}>
                  <Typography noWrap variant="h3">
                    {displayName}
                  </Typography>
                </Tooltip>
                <GatewayStatusLabel gateway={gateway} />
                <Tooltip title={intl.formatMessage(messages.edit)}>
                  <IconButton
                    aria-label={intl.formatMessage(messages.edit)}
                    onClick={openEditor}
                    size="small"
                    sx={{ flexShrink: 0 }}
                  >
                    <Pencil size={16} />
                  </IconButton>
                </Tooltip>
              </Stack>

              {description ? (
                <Typography color="text.secondary" variant="body2">
                  {description}
                </Typography>
              ) : (
                <Typography color="text.disabled" sx={{ fontStyle: 'italic' }} variant="body2">
                  <FormattedMessage {...messages.descriptionPlaceholder} />
                </Typography>
              )}
            </>
          )}

          <Stack
            direction="row"
            spacing={1}
            sx={{ alignItems: 'center', flexWrap: 'wrap' }}
            useFlexGap
          >
            <GatewayModeChip gateway={gateway} />
            <GatewayFunctionalityChip gateway={gateway} />
            <GatewayVersion version={gateway.version} />
          </Stack>

          {created && (
            <Stack
              direction="row"
              spacing={0.75}
              sx={{ alignItems: 'center', color: 'text.secondary' }}
            >
              <Typography color="text.secondary" variant="body2">
                <FormattedMessage {...messages.created} />
              </Typography>
              <Clock size={14} />
              <Tooltip title={dateTime(created)}>
                <Typography color="text.secondary" variant="body2">
                  {relativeTime(created)}
                </Typography>
              </Tooltip>
            </Stack>
          )}
        </Stack>
      </Stack>
    </Card>
  );
}
