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

import type { ReactNode } from 'react';
import { Avatar, Box, Button, Chip, IconButton, Stack, Typography } from '@wso2/oxygen-ui';
import { Check, Copy, X } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { ServiceAccount } from '@/api/resources/serviceAccounts';
import { useFormatters } from '@/i18n/useFormatters';
import { initials } from '@/utils/initials';
import { useCopy } from './useCopy';

const messages = defineMessages({
  overline: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountEditParts.overline',
    defaultMessage: 'Edit service account',
  },
  close: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountEditParts.close',
    defaultMessage: 'Close',
  },
  statusActive: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountEditParts.statusActive',
    defaultMessage: 'Active',
  },
  statusDisabled: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountEditParts.statusDisabled',
    defaultMessage: 'Disabled',
  },
  createdBy: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountEditParts.createdBy',
    defaultMessage: 'Created by {who}',
  },
  updated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountEditParts.updated',
    defaultMessage: 'Updated {when}',
    description: '{when} is a relative time, e.g. "3 minutes ago".',
  },
  secretReplaced: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountEditParts.secretReplaced',
    defaultMessage: 'Secret replaced {when}',
    description: '{when} is a relative time, e.g. "3 minutes ago".',
  },
  clientId: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountEditParts.clientId',
    defaultMessage: 'Client ID',
  },
  secret: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountEditParts.secret',
    defaultMessage: 'Secret',
  },
  copy: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountEditParts.copy',
    defaultMessage: 'Copy',
  },
  copied: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountEditParts.copied',
    defaultMessage: 'Copied',
  },
  copyClientId: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountEditParts.copyClientId',
    defaultMessage: 'Copy client ID',
  },
  regenerate: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountEditParts.regenerate',
    defaultMessage: 'Regenerate',
  },
  separator: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.serviceAccounts.components.ServiceAccountEditParts.separator',
    defaultMessage: '·',
    description: 'Between the facts under the account name.',
  },
});

/** `***05a89` as shown on screen: dots, then the last characters. */
const maskForDisplay = (masked: string) => '••••••••' + masked.replace(/^\*+/, '');

/** Small uppercase heading for a group of fields. */
export function SectionLabel({ children }: { children: ReactNode }) {
  return (
    <Typography
      color="text.secondary"
      sx={{ fontWeight: 700, letterSpacing: '0.08em', textTransform: 'uppercase' }}
      variant="caption"
    >
      {children}
    </Typography>
  );
}

/** The edit dialog's title: who the account is, at a glance. */
export function ServiceAccountHeader({
  account,
  disabled,
  onClose,
}: {
  account: ServiceAccount;
  disabled: boolean;
  onClose: () => void;
}) {
  const intl = useIntl();
  const { relativeTime } = useFormatters();
  const active = account.status === 'active';
  // Who the account is on the first line; when it last changed on the second.
  const lines = [
    [
      <Box component="span" key="id" sx={{ fontFamily: 'monospace' }}>
        {account.id}
      </Box>,
      account.createdBy && intl.formatMessage(messages.createdBy, { who: account.createdBy }),
    ],
    [
      account.updatedAt &&
        intl.formatMessage(messages.updated, { when: relativeTime(account.updatedAt) }),
      account.secretRegeneratedAt &&
        intl.formatMessage(messages.secretReplaced, {
          when: relativeTime(account.secretRegeneratedAt),
        }),
    ],
  ]
    .map((facts) => facts.filter(Boolean))
    .filter((facts) => facts.length > 0);

  return (
    <Stack alignItems="flex-start" direction="row" spacing={2} sx={{ px: 3, py: 2.5 }}>
      <Avatar
        sx={{
          background: (theme) =>
            `linear-gradient(135deg, ${theme.palette.primary.light}, ${theme.palette.primary.main})`,
          color: 'primary.contrastText',
          fontSize: 20,
          fontWeight: 700,
          height: 52,
          width: 52,
        }}
      >
        {initials(account.displayName)}
      </Avatar>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography color="text.secondary" variant="body2">
          <FormattedMessage {...messages.overline} />
        </Typography>
        <Stack alignItems="center" direction="row" flexWrap="wrap" gap={1.5}>
          <Typography component="h2" sx={{ fontWeight: 600, wordBreak: 'break-word' }} variant="h5">
            {account.displayName}
          </Typography>
          <Chip
            color={active ? 'success' : 'default'}
            label={intl.formatMessage(active ? messages.statusActive : messages.statusDisabled)}
            size="small"
            variant="outlined"
          />
        </Stack>
        {lines.map((facts, line) => (
          <Typography
            color="text.secondary"
            key={line}
            sx={{ mt: line === 0 ? 0.5 : 0 }}
            variant="body2"
          >
            {facts.map((fact, index) => (
              <Box component="span" key={index}>
                {index > 0 && (
                  <Box component="span" sx={{ mx: 0.75 }}>
                    <FormattedMessage {...messages.separator} />
                  </Box>
                )}
                {fact}
              </Box>
            ))}
          </Typography>
        ))}
      </Box>
      <IconButton
        aria-label={intl.formatMessage(messages.close)}
        disabled={disabled}
        onClick={onClose}
        size="small"
      >
        <X size={18} />
      </IconButton>
    </Stack>
  );
}

/** Client ID and masked secret, with Copy and Regenerate. */
export function CredentialsPanel({
  account,
  disabled,
  onRegenerate,
}: {
  account: ServiceAccount;
  disabled: boolean;
  onRegenerate: () => void;
}) {
  const intl = useIntl();
  const { copied, copy } = useCopy();
  const row = {
    alignItems: 'center',
    display: 'grid',
    gap: 2,
    gridTemplateColumns: '96px 1fr auto',
    px: 2,
    py: 1.25,
  } as const;
  const value = { fontFamily: 'monospace', minWidth: 0, wordBreak: 'break-all' } as const;

  return (
    <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 1 }}>
      <Box sx={{ ...row, borderBottom: 1, borderColor: 'divider' }}>
        <Typography color="text.secondary" variant="body2">
          <FormattedMessage {...messages.clientId} />
        </Typography>
        <Typography sx={value} variant="body2">
          {account.clientId}
        </Typography>
        <Button
          aria-label={intl.formatMessage(messages.copyClientId)}
          color="inherit"
          onClick={() => copy(account.clientId)}
          size="small"
          startIcon={copied ? <Check size={16} /> : <Copy size={16} />}
          variant="text"
        >
          <FormattedMessage {...(copied ? messages.copied : messages.copy)} />
        </Button>
      </Box>
      <Box sx={row}>
        <Typography color="text.secondary" variant="body2">
          <FormattedMessage {...messages.secret} />
        </Typography>
        <Typography sx={value} variant="body2">
          {maskForDisplay(account.maskedSecret)}
        </Typography>
        <Button disabled={disabled} onClick={onRegenerate} size="small" variant="text">
          <FormattedMessage {...messages.regenerate} />
        </Button>
      </Box>
    </Box>
  );
}
