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

import { Box, Collapse, IconButton, Tooltip, Typography } from '@wso2/oxygen-ui';
import { ChevronDown, ChevronUp } from '@wso2/oxygen-ui-icons-react';
import { useId, useState, type ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { hairline } from '@/theme/receipes';
import { MethodBadge } from './MethodBadge';
import { methodPalette } from './methodPalette';

const messages = defineMessages({
  collapse: {
    id: 'apiControlPlane.components.SwaggerOperationsView.SwaggerResourceRow.collapse',
    defaultMessage: 'Hide details for {method} {path}',
    description:
      'Accessible label for the control that closes one resource row. {method} is an HTTP verb such as GET; {path} is a URL path. Neither is translated.',
  },
  expand: {
    id: 'apiControlPlane.components.SwaggerOperationsView.SwaggerResourceRow.expand',
    defaultMessage: 'Show details for {method} {path}',
    description:
      'Accessible label for the control that opens one resource row. {method} is an HTTP verb such as GET; {path} is a URL path. Neither is translated.',
  },
});

/**
 * A Swagger UI-styled API resource row with method badge and path. Flat by
 * default; with `children` it becomes expandable with a collapsible panel below.
 */
export type SwaggerResourceRowProps = {
  /** Trailing controls, placed before the disclosure chevron. */
  actions?: ReactNode;
  /** Shown between the summary and the trailing controls; a policy count, for instance. */
  badge?: ReactNode;
  /** The body panel's content. Supplying it is what makes the row expandable. */
  children?: ReactNode;
  defaultExpanded?: boolean;
  /** The operation's summary line. Omitted, the row stays a single line. */
  description?: string;
  /** Dims the row and blocks its controls — a staged deletion, or a pending save. */
  disabled?: boolean;
  method: string;
  path: string;
};

export function SwaggerResourceRow({
  actions,
  badge,
  children,
  defaultExpanded = false,
  description,
  disabled = false,
  method,
  path,
}: SwaggerResourceRowProps) {
  const intl = useIntl();
  const bodyId = useId();
  const [expanded, setExpanded] = useState(defaultExpanded);
  const expandable = Boolean(children);
  const toggle = () => setExpanded((open) => !open);
  const toggleLabel = intl.formatMessage(expanded ? messages.collapse : messages.expand, {
    method,
    path,
  });

  return (
    <Box sx={{ minWidth: 0, width: '100%' }}>
      <Box
        // The chevron below is the accessible control; clicking the row is a
        // mouse convenience on top of it, so this stays a plain element.
        onClick={expandable && !disabled ? toggle : undefined}
        sx={(theme) => {
          const tone = methodPalette(method);
          return {
            alignItems: 'center',
            bgcolor: tone.bg,
            border: hairline(theme),
            borderColor: tone.border,
            borderRadius: 0.75,
            cursor: expandable && !disabled ? 'pointer' : 'default',
            display: 'flex',
            gap: 1.25,
            minHeight: 44,
            minWidth: 0,
            opacity: disabled ? 0.45 : 1,
            px: 1.1,
            py: 0.6,
          };
        }}
      >
        <MethodBadge method={method} />

        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography noWrap sx={{ fontFamily: 'monospace', fontWeight: 700 }} variant="body2">
            {path}
          </Typography>
          {description && (
            <Typography color="text.secondary" noWrap variant="body2">
              {description}
            </Typography>
          )}
        </Box>

        {badge}

        {actions && (
          <Box
            onClick={(event) => event.stopPropagation()}
            sx={{ alignItems: 'center', display: 'flex', flexShrink: 0 }}
          >
            {actions}
          </Box>
        )}

        {expandable && (
          <Tooltip title={toggleLabel}>
            <IconButton
              aria-controls={bodyId}
              aria-expanded={expanded}
              aria-label={toggleLabel}
              disabled={disabled}
              onClick={(event) => {
                event.stopPropagation();
                toggle();
              }}
              size="small"
            >
              {expanded ? <ChevronUp size={18} /> : <ChevronDown size={18} />}
            </IconButton>
          </Tooltip>
        )}
      </Box>

      {expandable && (
        <Collapse in={expanded} timeout="auto" unmountOnExit>
          <Box
            id={bodyId}
            sx={(theme) => ({
              bgcolor: 'background.paper',
              border: hairline(theme),
              borderColor: 'divider',
              borderRadius: 1,
              minWidth: 0,
              mt: 1,
              px: 1.5,
              py: 1,
            })}
          >
            {children}
          </Box>
        </Collapse>
      )}
    </Box>
  );
}
