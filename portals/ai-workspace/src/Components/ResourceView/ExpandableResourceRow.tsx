/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import React, { useMemo } from 'react';
import { Box, Collapse, IconButton, Stack, Typography } from '@wso2/oxygen-ui';
import { ChevronDown, ChevronUp } from '@wso2/oxygen-ui-icons-react';
import SwaggerSpecViewer from '../SwaggerSpecViewer';
import ResourceRow, { ResourceViewItem } from './ResourceRow';

type ExpandableResourceRowProps = {
  resource: ResourceViewItem;
  isOpen: boolean;
  selected?: boolean;
  operationSpec?: Record<string, unknown> | null;
  showSummary?: boolean;
  noSummaryText?: string;
  onRowClick?: () => void;
  onToggleOpen: () => void;
};

export default function ExpandableResourceRow({
  resource,
  isOpen,
  selected,
  operationSpec,
  showSummary = true,
  noSummaryText = 'No summary available.',
  onRowClick,
  onToggleOpen,
}: ExpandableResourceRowProps) {
  const fallbackOperationSpec = useMemo(() => {
    const method = resource.method.toLowerCase();
    const supportedMethods = new Set([
      'get',
      'post',
      'put',
      'delete',
      'patch',
      'head',
      'options',
    ]);
    if (!supportedMethods.has(method)) {
      return null;
    }

    const operation: Record<string, unknown> = {
      responses: {
        default: {
          description: 'No response details available.',
        },
      },
    };

    if (resource.summary) {
      operation.summary = resource.summary;
    }

    return {
      openapi: '3.0.3',
      info: {
        title: `${resource.method.toUpperCase()} ${resource.path}`,
        version: '1.0.0',
      },
      paths: {
        [resource.path]: {
          [method]: operation,
        },
      },
    };
  }, [resource.method, resource.path, resource.summary]);

  const resolvedOperationSpec = operationSpec ?? fallbackOperationSpec;

  return (
    <Box sx={{ width: '100%', minWidth: 0, maxWidth: '100%' }}>
      <ResourceRow
        resource={resource}
        selected={selected}
        showSummary={showSummary}
        onClick={onRowClick}
        trailing={
          <IconButton
            size="small"
            onClick={(event) => {
              event.stopPropagation();
              onToggleOpen();
            }}
          >
            {isOpen ? <ChevronUp size={18} /> : <ChevronDown size={18} />}
          </IconButton>
        }
      />

      <Collapse in={isOpen} timeout="auto" unmountOnExit>
        <Box
          sx={{
            mt: 1,
            px: 1.5,
            py: 1,
            borderRadius: 1,
            border: '1px solid',
            borderColor: 'divider',
            minWidth: 0,
            backgroundColor: 'background.paper',
            maxWidth: '100%',
            overflow: 'hidden',
          }}
        >
          {resolvedOperationSpec ? (
            <SwaggerSpecViewer
              spec={resolvedOperationSpec}
              hideInfoSection
              hideServers
              hideAuthorizeButton
              disableTryOutBtn
              hideTagHeaders
              hideOperationHeader
              docExpansion="full"
              defaultModelsExpandDepth={-1}
              displayRequestDuration={false}
            />
          ) : (
            <Stack spacing={0.5} sx={{ minWidth: 0 }}>
              <Typography
                variant="body2"
                sx={{
                  fontWeight: 600,
                  minWidth: 0,
                  maxWidth: '100%',
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                }}
              >
                {resource.method} {resource.path}
              </Typography>
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{
                  minWidth: 0,
                  maxWidth: '100%',
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  display: 'block',
                }}
              >
                {resource.summary || noSummaryText}
              </Typography>
            </Stack>
          )}
        </Box>
      </Collapse>
    </Box>
  );
}
