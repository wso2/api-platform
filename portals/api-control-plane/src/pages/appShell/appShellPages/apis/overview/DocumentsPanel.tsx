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

import {
  Box,
  Button,
  Card,
  Chip,
  Divider,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronRight, FileText } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Link as RouterLink } from 'react-router-dom';

import { useApiDocuments } from '@/api/resources/apiDocuments';
import { REST_API_TYPE } from '@/api/resources/apiPublications';
import { useFormatters } from '@/i18n/useFormatters';
import { routes } from '@/routes/paths';
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
  empty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.DocumentsPanel.empty',
    defaultMessage: 'No Documents available for this API',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.DocumentsPanel.loading',
    defaultMessage: 'Loading documents…',
  },
  loadError: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.DocumentsPanel.loadError',
    defaultMessage: 'Unable to load documents.',
  },
  updated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.DocumentsPanel.updated',
    defaultMessage: 'Updated {when}',
    description: '{when} is a relative time, e.g. "2 days ago".',
  },
});

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

  return (
    <Card>
      <Stack
        direction="row"
        sx={{ alignItems: 'center', justifyContent: 'space-between', px: 2, py: 1.5 }}
      >
        <Typography component="h2" sx={{ fontWeight: 600 }} variant="h6">
          <FormattedMessage {...messages.title} />
        </Typography>
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
        <>
          <List disablePadding>
            {documents.map((document, index) => {
              const when = relativeTime(document.updatedAt ?? document.createdAt);
              return (
                <Box component="li" key={document.id} sx={{ listStyle: 'none' }}>
                  {index > 0 && <Divider component="div" />}
                  <ListItemButton
                    component={RouterLink}
                    sx={{ gap: 1.5, px: 2, py: 1.25 }}
                    to={`${documentsPath ?? ''}${documentsSearch({ docId: document.id, mode: 'browse' })}`}
                  >
                    <ListItemIcon sx={{ minWidth: 0 }}>
                      <FileText size={18} />
                    </ListItemIcon>
                    <ListItemText
                      primary={document.displayName}
                      secondary={
                        when ? intl.formatMessage(messages.updated, { when }) : undefined
                      }
                      slotProps={{
                        primary: { noWrap: true, variant: 'body2' },
                        secondary: { noWrap: true, variant: 'caption' },
                      }}
                      sx={{ minWidth: 0 }}
                    />
                    <Chip
                      label={documentTypeName(intl, document.type)}
                      size="small"
                      sx={{ flexShrink: 0, typography: 'caption' }}
                    />
                  </ListItemButton>
                </Box>
              );
            })}
          </List>
          {documentsPath && total > PREVIEW_COUNT && (
            <>
              <Divider />
              <Box sx={{ display: 'flex', justifyContent: 'center', p: 1.5 }}>
                <Button
                  component={RouterLink}
                  endIcon={<ChevronRight size={16} />}
                  size="small"
                  to={documentsPath}
                  variant="outlined"
                >
                  <FormattedMessage {...messages.viewMore} />
                </Button>
              </Box>
            </>
          )}
        </>
      )}
    </Card>
  );
}
