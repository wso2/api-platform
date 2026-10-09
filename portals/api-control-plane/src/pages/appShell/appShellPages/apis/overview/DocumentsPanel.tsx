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

import { Box, Card, Chip, Divider, Link, ListItemButton, Stack, Typography } from '@wso2/oxygen-ui';
import { ChevronRight, FileText, Plus } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Link as RouterLink } from 'react-router-dom';

import { useApiDocuments } from '@/api/resources/apiDocuments';
import { useFormatters } from '@/i18n/useFormatters';
import { REST_API_TYPE } from '@/api/resources/apiPublications';
import { routes } from '@/routes/paths';
import { Can } from '@/permissions';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { documentTypeName } from '../../develop/documents/documentTypes';
import { documentsSearch } from '../../develop/documents/documentsSearch';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.DocumentsPanel.title',
    defaultMessage: 'Documents',
  },
  viewMore: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.DocumentsPanel.viewMore',
    defaultMessage: 'View More',
    description: 'Opens the API’s Documents page to see every document.',
  },
  create: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.DocumentsPanel.create',
    defaultMessage: 'Create Document',
    description: 'Opens the form for a new API document. Shown when the API has none.',
  },
  empty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.DocumentsPanel.empty',
    defaultMessage: 'No Documents available for this API',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.DocumentsPanel.loading',
    defaultMessage: 'Loading documents…',
  },
  updated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.DocumentsPanel.updated',
    defaultMessage: 'Updated {when}',
    description: '{when} is a relative time, e.g. "2 days ago".',
  },
  loadError: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.DocumentsPanel.loadError',
    defaultMessage: 'Unable to load documents.',
  },
});

/** Icon and label of a header link sit on one line, centred on each other. */
const HEADER_LINK_SX = {
  alignItems: 'center',
  display: 'inline-flex',
  flexShrink: 0,
  gap: 0.5,
} as const;

/** Documents shown on the overview; the rest are a click away on the Documents page. */
const PREVIEW_COUNT = 5;

/**
 * The API's most recently updated documents. Each row opens that document on
 * Develop › Documents, and "View More" opens the full list there.
 */
export function DocumentsPanel() {
  const intl = useIntl();
  const { relativeTime } = useFormatters();
  const { params } = useConsoleScope();
  const { apiHandler, orgHandle, projectHandler } = params;
  const documentsQuery = useApiDocuments(REST_API_TYPE, apiHandler, { limit: PREVIEW_COUNT });

  const documentsPath =
    orgHandle && projectHandler && apiHandler
      ? routes.apiDevelopDocuments(orgHandle, projectHandler, apiHandler)
      : undefined;

  const documents = documentsQuery.data?.list ?? [];
  const total = documentsQuery.data?.pagination.total ?? 0;
  const loaded = !documentsQuery.isPending && !documentsQuery.error;

  return (
    <Card>
      <Stack
        alignItems="center"
        direction="row"
        justifyContent="space-between"
        spacing={1.5}
        sx={{ px: 2, py: 1.5 }}
      >
        <Typography component="h2" sx={{ fontWeight: 600 }} variant="h6">
          <FormattedMessage {...messages.title} />
        </Typography>
        {documentsPath && loaded && total > PREVIEW_COUNT && (
          <Link
            component={RouterLink}
            sx={HEADER_LINK_SX}
            to={documentsPath}
            underline="hover"
            variant="body2"
          >
            <FormattedMessage {...messages.viewMore} />
            <ChevronRight size={16} />
          </Link>
        )}
        {documentsPath && loaded && total === 0 && (
          <Can do="CreateAPIDocument" denied="hide">
            <Link
              component={RouterLink}
              sx={HEADER_LINK_SX}
              to={`${documentsPath}${documentsSearch({ mode: 'create' })}`}
              underline="hover"
              variant="body2"
            >
              <Plus size={16} />
              <FormattedMessage {...messages.create} />
            </Link>
          </Can>
        )}
      </Stack>
      <Divider />

      {documentsQuery.isPending ? (
        <Typography color="text.secondary" sx={{ px: 2, py: 2.5 }} variant="body2">
          <FormattedMessage {...messages.loading} />
        </Typography>
      ) : documentsQuery.error ? (
        <Typography color="error" sx={{ px: 2, py: 2.5 }} variant="body2">
          <FormattedMessage {...messages.loadError} />
        </Typography>
      ) : total === 0 ? (
        <Typography color="text.secondary" sx={{ px: 2, py: 2.5 }} variant="body2">
          <FormattedMessage {...messages.empty} />
        </Typography>
      ) : (
        <Stack
          component="ul"
          divider={<Divider component="li" />}
          sx={{ listStyle: 'none', m: 0, p: 0 }}
        >
          {documents.map((document) => (
            <Box component="li" key={document.id}>
              <ListItemButton
                component={RouterLink}
                sx={{
                  display: 'grid',
                  gridTemplateColumns: {
                    xs: 'auto minmax(0, 1fr) 8rem',
                    sm: 'auto minmax(10rem, 24rem) 8rem minmax(0, 1fr)',
                  },
                  alignItems: 'center',
                  columnGap: 1.5,
                  px: 2,
                  py: 1.25,
                }}
                to={`${documentsPath ?? ''}${documentsSearch({ docId: document.id, mode: 'browse' })}`}
              >
                <FileText color="currentColor" size={16} />
                {/*
                 * `noWrap` + the bounded `minmax(10rem, 24rem)` grid column
                 * truncates long titles with an ellipsis; `minWidth: 0` lets the
                 * cell shrink below its content width. The full name is kept
                 * in `title` so it's still readable on hover.
                 */}
                <Typography
                  noWrap
                  sx={{ minWidth: 0, overflow: 'hidden' }}
                  title={document.displayName}
                  variant="body2"
                >
                  {document.displayName}
                </Typography>
                <Chip
                  label={documentTypeName(intl, document.type)}
                  size="small"
                  sx={{ justifySelf: 'start', maxWidth: '100%', typography: 'caption' }}
                />
                <Typography
                  color="text.secondary"
                  noWrap
                  sx={{ display: { sm: 'block', xs: 'none' }, minWidth: 0 }}
                  variant="caption"
                >
                  <FormattedMessage
                    {...messages.updated}
                    values={{ when: relativeTime(document.updatedAt ?? document.createdAt) }}
                  />
                </Typography>
              </ListItemButton>
            </Box>
          ))}
        </Stack>
      )}
    </Card>
  );
}
