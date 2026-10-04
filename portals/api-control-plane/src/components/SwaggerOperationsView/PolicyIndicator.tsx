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

import { Box, Tooltip, Typography } from '@wso2/oxygen-ui';
import { defineMessages, useIntl } from 'react-intl';

const messages = defineMessages({
  overflow: {
    id: 'apiControlPlane.components.SwaggerOperationsView.PolicyIndicator.overflow',
    defaultMessage: '{count, plural, one {# more policy} other {# more policies}}: {names}',
    description:
      'Tooltip on the "+N" circle standing for policies too numerous to draw. {names} is a comma-separated list of policy names, which are not translated.',
  },
  overflowCount: {
    id: 'apiControlPlane.components.SwaggerOperationsView.PolicyIndicator.overflowCount',
    defaultMessage: '+{count}',
    description:
      'Label inside the circle standing for policies too numerous to draw, e.g. "+3". Kept short enough to fit a 28px circle.',
  },
  policy: {
    id: 'apiControlPlane.components.SwaggerOperationsView.PolicyIndicator.policy',
    defaultMessage: '{name} · {version}',
    description:
      'Tooltip on one policy circle. {name} is a policy name and {version} its version; neither is translated.',
  },
});

/** Attached policies shown as overlapping, initialled circles. */

/** Circle diameter, and how much of the previous circle each one covers. */
const SIZE = 28;
const OVERLAP = 6;

/** Ring separating overlapping circles, matching the surface behind the cluster. */
const RING_WIDTH = 2;

/** Beyond this, the rest collapse into a single "+N" circle. */
const MAX_VISIBLE = 4;

/** Identity palette; colours stay distinct from semantic theme roles. */
const IDENTITY_COLORS = [
  '#FF6B35',
  '#1976D2',
  '#9C27B0',
  '#2E7D32',
  '#C62828',
  '#00838F',
  '#EF6C00',
  '#4527A0',
];

/** The "+N" circle is deliberately outside the identity palette — it names no one policy. */
const OVERFLOW_COLOR = '#78909C';

/** Extracts initial-bearing words from a namespaced policy name. */
const wordsOf = (name: string): string[] =>
  name
    .split('.')
    .pop()!
    .split(/[\s_-]+|(?<=[a-z0-9])(?=[A-Z])/)
    .filter(Boolean);

/** Up to two letters standing for a policy. */
export const policyInitials = (name: string): string => {
  const words = wordsOf(name);
  if (words.length === 0) return '?';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return (words[0][0] + words[1][0]).toUpperCase();
};

/** Stable identity colour derived from the policy name, not its position. */
export const policyColor = (name: string): string => {
  let hash = 0;
  for (let i = 0; i < name.length; i += 1) {
    hash = (hash * 31 + name.charCodeAt(i)) | 0;
  }
  return IDENTITY_COLORS[Math.abs(hash) % IDENTITY_COLORS.length];
};

/** One circle. `offset` is its index in the drawn row, not in the policy list. */
function Circle({
  color,
  label,
  offset,
  stackSize,
  title,
}: {
  color: string;
  label: string;
  offset: number;
  stackSize: number;
  title: string;
}) {
  return (
    <Tooltip arrow title={title}>
      <Box
        sx={(theme) => ({
          alignItems: 'center',
          bgcolor: color,
          border: `${RING_WIDTH}px ${theme.border.style}`,
          borderColor: 'background.paper',
          borderRadius: '50%',
          cursor: 'default',
          display: 'inline-flex',
          height: SIZE,
          justifyContent: 'center',
          ml: offset === 0 ? 0 : `-${OVERLAP}px`,
          width: SIZE,
          // Earlier circles sit on top, so the cluster reads left-to-right.
          zIndex: stackSize - offset,
        })}
      >
        <Typography
          sx={{
            color: 'common.white',
            fontSize: 10,
            fontWeight: 700,
            lineHeight: 1,
            userSelect: 'none',
          }}
        >
          {label}
        </Typography>
      </Box>
    </Tooltip>
  );
}

export type PolicyIndicatorPolicy = {
  name: string;
  version: string;
};

export function PolicyIndicator({ policies }: { policies: PolicyIndicatorPolicy[] }) {
  const intl = useIntl();
  if (policies.length === 0) return null;

  const visible = policies.slice(0, MAX_VISIBLE);
  const hidden = policies.slice(MAX_VISIBLE);
  const stackSize = visible.length + (hidden.length > 0 ? 1 : 0);

  return (
    <Box sx={{ alignItems: 'center', display: 'flex', flexShrink: 0 }}>
      {visible.map((policy, index) => (
        <Circle
          color={policyColor(policy.name)}
          key={`${policy.name}-${index}`}
          label={policyInitials(policy.name)}
          offset={index}
          stackSize={stackSize}
          title={intl.formatMessage(messages.policy, {
            name: policy.name,
            version: policy.version,
          })}
        />
      ))}
      {hidden.length > 0 && (
        <Circle
          color={OVERFLOW_COLOR}
          label={intl.formatMessage(messages.overflowCount, { count: hidden.length })}
          offset={visible.length}
          stackSize={stackSize}
          title={intl.formatMessage(messages.overflow, {
            count: hidden.length,
            names: hidden.map((policy) => policy.name).join(', '),
          })}
        />
      )}
    </Box>
  );
}
