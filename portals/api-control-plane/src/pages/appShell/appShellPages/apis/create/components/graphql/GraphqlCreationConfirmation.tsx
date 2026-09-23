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

import { Box, Button, Chip, IconButton, Paper, Stack, Tooltip, Typography } from '@wso2/oxygen-ui';
import { CheckCircle2, Copy } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Link as RouterLink } from 'react-router-dom';

import { GraphqlIcon } from '../../uiConfig';
import { useNotifications } from '@/components/Notifications';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import type { GraphQLApi } from '@/api/resources/graphqlApis';

const messages = defineMessages({
  contextLabel: {
    id: 'api.create.generalForm.context.label',
    defaultMessage: 'Context',
  },
  copied: {
    id: 'api.create.graphql.confirmation.copy.done',
    defaultMessage: 'Copied to clipboard.',
  },
  copy: {
    id: 'api.create.graphql.confirmation.copy.action',
    defaultMessage: 'Copy {label}',
    description: 'Accessible label for a field\'s copy-to-clipboard button.',
  },
  endpointLabel: {
    id: 'api.create.graphql.confirmation.endpoint.label',
    defaultMessage: 'Query and Mutation URL',
  },
  goToApi: {
    id: 'api.create.graphql.confirmation.action.goToApi',
    defaultMessage: 'Go to API',
  },
  subtitle: {
    id: 'api.create.graphql.confirmation.subtitle',
    defaultMessage: 'Deploy it to a gateway and try it out from its own overview page.',
  },
  title: {
    id: 'api.create.graphql.confirmation.title',
    defaultMessage: '{name} created',
    description: '{name} is the display name the user gave the API. Never translated.',
  },
  typeChip: {
    id: 'api.create.apiType.graphQl.title',
    defaultMessage: 'GraphQL API',
  },
});

/** One `label: value` row with a copy button. */
const CopyableField = ({ label, value }: { label: string; value: string }) => {
  const intl = useIntl();
  const { notify } = useNotifications();

  const copy = () => {
    void navigator.clipboard?.writeText(value);
    notify(intl.formatMessage(messages.copied), 'success');
  };

  return (
    <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', minWidth: 0 }}>
      <Typography color="text.secondary" sx={{ flexShrink: 0 }} variant="caption">
        {label}
      </Typography>
      <Typography noWrap sx={{ fontFamily: 'monospace', minWidth: 0 }} title={value} variant="body2">
        {value}
      </Typography>
      <Tooltip title={intl.formatMessage(messages.copy, { label })}>
        <IconButton
          aria-label={intl.formatMessage(messages.copy, { label })}
          onClick={copy}
          size="small"
        >
          <Copy size={15} />
        </IconButton>
      </Tooltip>
    </Stack>
  );
};

export type GraphqlCreationConfirmationProps = {
  api: GraphQLApi;
};

/**
 * The GraphQL wizard's terminal screen: what was created, then on to the
 * GraphQL API's own Overview page — not the shared (REST-typed) one, see
 * `graphqlApiPath` for why these routes live under a distinct segment.
 */
export const GraphqlCreationConfirmation = ({ api }: GraphqlCreationConfirmationProps) => {
  const intl = useIntl();
  const { params } = useConsoleScope();

  return (
    <Stack spacing={3} sx={{ alignItems: 'center', py: 6, textAlign: 'center', width: '100%' }}>
      <Box sx={{ color: 'success.main', display: 'flex' }}>
        <CheckCircle2 size={48} />
      </Box>

      <Stack spacing={0.5} sx={{ alignItems: 'center' }}>
        <Typography sx={{ fontWeight: 700 }} variant="h4">
          <FormattedMessage {...messages.title} values={{ name: api.displayName }} />
        </Typography>
        <Typography color="text.secondary" variant="body2">
          <FormattedMessage {...messages.subtitle} />
        </Typography>
      </Stack>

      <Paper sx={{ maxWidth: 560, p: 3, width: '100%' }} variant="outlined">
        <Stack spacing={2}>
          <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', justifyContent: 'center' }}>
            <GraphqlIcon />
            <Chip label={intl.formatMessage(messages.typeChip)} size="small" />
          </Stack>
          <CopyableField label={intl.formatMessage(messages.contextLabel)} value={api.context ?? ''} />
          <CopyableField
            label={intl.formatMessage(messages.endpointLabel)}
            value={api.upstream.main.url ?? ''}
          />
        </Stack>
      </Paper>

      <Button
        component={RouterLink}
        to={routes.graphqlApi(
          params.orgHandle ?? '',
          params.projectHandler ?? null,
          api.id ?? null,
        )}
        variant="contained"
      >
        <FormattedMessage {...messages.goToApi} />
      </Button>
    </Stack>
  );
};
