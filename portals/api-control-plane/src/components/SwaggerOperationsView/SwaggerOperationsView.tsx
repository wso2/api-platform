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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { Box, IconButton, Stack, Tooltip, Typography } from '@wso2/oxygen-ui';
import { Trash2 } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { Operation } from '@/api/resources/restApis';
import { SwaggerResourceRow } from './SwaggerResourceRow';

const messages = defineMessages({
  delete: {
    id: 'apiControlPlane.components.SwaggerOperationsView.delete',
    defaultMessage: 'Delete resource',
    description: 'Tooltip on the button that removes one API resource from the list. Verb phrase.',
  },
  deleteLabel: {
    id: 'apiControlPlane.components.SwaggerOperationsView.deleteLabel',
    defaultMessage: 'Delete {method} {path}',
    description:
      'Accessible label for the delete button on one resource. {method} is an HTTP verb such as GET; {path} is a URL path such as /books/{id}. Neither is translated.',
  },
  empty: {
    id: 'apiControlPlane.components.SwaggerOperationsView.empty',
    defaultMessage: 'No operations available.',
  },
});

export type SwaggerOperationsViewProps = {
  isOperationDisabled?: (operation: Operation, index: number) => boolean;
  onDelete?: (index: number) => void;
  operations: Operation[];
  showDelete?: boolean;
};

/** Compact Swagger-style operation summary shared by pages that do not need Swagger's detail UI. */
export function SwaggerOperationsView({
  isOperationDisabled,
  onDelete,
  operations,
  showDelete = false,
}: SwaggerOperationsViewProps) {
  const intl = useIntl();

  if (operations.length === 0) {
    return (
      <Typography color="text.secondary" variant="body2">
        <FormattedMessage {...messages.empty} />
      </Typography>
    );
  }

  return (
    <Stack spacing={1.5}>
      {operations.map((operation, index) => {
        const { method, path } = operation.request;
        const disabled = isOperationDisabled?.(operation, index) ?? false;
        return (
          <SwaggerResourceRow
            actions={
              showDelete && onDelete ? (
                <Tooltip title={intl.formatMessage(messages.delete)}>
                  {/* A disabled button fires no events, so the Tooltip listens
                      on this wrapper instead of on the button itself. */}
                  <Box component="span" sx={{ display: 'inline-flex' }}>
                    <IconButton
                      aria-label={intl.formatMessage(messages.deleteLabel, { method, path })}
                      color="error"
                      disabled={disabled}
                      onClick={() => onDelete(index)}
                      size="small"
                    >
                      <Trash2 size={17} />
                    </IconButton>
                  </Box>
                </Tooltip>
              ) : undefined
            }
            description={operation.description}
            disabled={disabled}
            key={`${method}-${path}-${index}`}
            method={method}
            path={path}
          />
        );
      })}
    </Stack>
  );
}
