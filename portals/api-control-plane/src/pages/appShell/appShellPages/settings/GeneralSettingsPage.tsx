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
import { Avatar, Button, Card, CardContent, Divider, Stack, Typography } from '@wso2/oxygen-ui';
import { BoxIcon, Building, Clock, Copy, Globe } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useNotifications } from '@/components/Notifications';
import { LoadingState } from '@/components/StateViews';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { relativeTime } from '@/utils/relativeTime';

const messages = defineMessages({
  copyId: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.GeneralSettingsPage.copyId',
    defaultMessage: 'Copy ID',
  },
  copySucceeded: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.GeneralSettingsPage.copySucceeded',
    defaultMessage: 'ID copied to clipboard.',
  },
  copyFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.GeneralSettingsPage.copyFailed',
    defaultMessage: 'Unable to copy the ID.',
  },
  created: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.GeneralSettingsPage.created',
    defaultMessage: 'Created',
  },
  region: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.GeneralSettingsPage.region',
    defaultMessage: 'Region',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.GeneralSettingsPage.loading',
    defaultMessage: 'Loading settings',
  },
  projectDetailsTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.GeneralSettingsPage.projectDetailsTitle',
    defaultMessage: 'Project Details',
  },
  organizationDetailsTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.GeneralSettingsPage.organizationDetailsTitle',
    defaultMessage: 'Organization Details',
  },
});

type Detail = { icon: ReactNode; id: string; label: string; value: string };

/** An icon + `Label value` pair, the value spaced explicitly from the label
 * rather than relying on whitespace surviving between two inline elements. */
function DetailItem({ icon, label, value }: Detail) {
  return (
    <Stack alignItems="center" direction="row" spacing={0.75}>
      <Stack alignItems="center" sx={{ color: 'text.secondary' }}>
        {icon}
      </Stack>
      <Typography color="text.secondary" sx={{ opacity: 0.7 }} variant="body2">
        {label}
      </Typography>
      <Typography variant="body2">{value}</Typography>
    </Stack>
  );
}

type EntityGeneralCardProps = {
  icon: ReactNode;
  displayName: string;
  id: string;
  /** Rows shown below the identity row, e.g. created date, region. */
  details?: Detail[];
};

/**
 * Shared "General" tab content for both the organization- and project-level
 * Settings pages: identity (icon, display name, a copy-id action) plus
 * whatever read-only details that scope has — an organization also has a
 * region, a project does not.
 */
function EntityGeneralCard({ icon, displayName, id, details = [] }: EntityGeneralCardProps) {
  const intl = useIntl();
  const { notify } = useNotifications();

  const copyId = () => {
    navigator.clipboard
      ?.writeText(id)
      .then(() => notify(intl.formatMessage(messages.copySucceeded), 'success'))
      .catch(() => notify(intl.formatMessage(messages.copyFailed), 'error'));
  };

  return (
    <Card variant="outlined">
      <CardContent>
        <Stack alignItems="center" direction="row" justifyContent="space-between" spacing={2}>
          <Stack alignItems="center" direction="row" spacing={2.5} sx={{ minWidth: 0 }}>
            <Avatar
              sx={{ bgcolor: 'primary.light', color: 'primary.contrastText', height: 48, width: 48 }}
              variant="rounded"
            >
              {icon}
            </Avatar>
            <Typography noWrap sx={{ fontWeight: 700 }} variant="h6">
              {displayName}
            </Typography>
          </Stack>

          <Button onClick={copyId} startIcon={<Copy size={16} />} variant="outlined">
            <FormattedMessage {...messages.copyId} />
          </Button>
        </Stack>

        {details.length > 0 && (
          <>
            <Divider sx={{ my: 2 }} />

            <Stack
              alignItems="center"
              direction="row"
              divider={<Divider flexItem orientation="vertical" />}
              flexWrap="wrap"
              spacing={2}
            >
              {details.map((detail) => (
                <DetailItem key={detail.id} {...detail} />
              ))}
            </Stack>
          </>
        )}
      </CardContent>
    </Card>
  );
}

export function GeneralSettingsPage() {
  const intl = useIntl();
  const { isLoading, organization, params, project } = useConsoleScope();

  if (params.projectHandler) {
    if (!project) return <LoadingState label={intl.formatMessage(messages.loading)} />;

    const details: Detail[] = [];
    if (project.createdAt) {
      details.push({
        icon: <Clock size={14} />,
        id: 'created',
        label: intl.formatMessage(messages.created),
        value: relativeTime(project.createdAt),
      });
    }

    return (
      <Stack spacing={1.5}>
        <Typography sx={{ fontWeight: 600 }} variant="subtitle2">
          <FormattedMessage {...messages.projectDetailsTitle} />
        </Typography>
        <EntityGeneralCard
          details={details}
          displayName={project.displayName}
          icon={<BoxIcon size={22} />}
          id={project.id}
        />
      </Stack>
    );
  }

  if (isLoading || !organization) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }

  const details: Detail[] = [];
  if (organization.createdAt) {
    details.push({
      icon: <Clock size={14} />,
      id: 'created',
      label: intl.formatMessage(messages.created),
      value: relativeTime(organization.createdAt),
    });
  }
  if (organization.region) {
    details.push({
      icon: <Globe size={14} />,
      id: 'region',
      label: intl.formatMessage(messages.region),
      value: organization.region,
    });
  }

  return (
    <Stack spacing={1.5}>
      <Typography sx={{ fontWeight: 600 }} variant="subtitle2">
        <FormattedMessage {...messages.organizationDetailsTitle} />
      </Typography>
      <EntityGeneralCard
        details={details}
        displayName={organization.displayName}
        icon={<Building size={22} />}
        id={organization.id}
      />
    </Stack>
  );
}
