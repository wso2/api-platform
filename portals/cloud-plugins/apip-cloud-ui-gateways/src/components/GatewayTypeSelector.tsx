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

import type { FC } from 'react';
import { Box } from '@wso2/oxygen-ui';
import { Network, Sparkles, Zap } from '@wso2/oxygen-ui-icons-react';
import GatewayTypeCard from './GatewayTypeCard';
import { gatewayTypeLabel } from '../utils/gateway';
import type { GatewayType } from '../types';

/**
 * One card per gateway type. A `Record` rather than an array so every type a
 * host may offer is guaranteed to have a card — icons and labels match the
 * built-in console's own type selector.
 */
const TYPE_CARDS: Record<
  GatewayType,
  { icon: FC<{ size?: number }>; badge?: string; comingSoon?: boolean }
> = {
  regular: { icon: Network },
  ai: { icon: Sparkles },
  // Announced but not yet creatable: the card stays visible so the type is
  // discoverable, and is rendered unselectable rather than dropped from the row.
  event: { icon: Zap, badge: 'Coming soon', comingSoon: true },
};

export type GatewayTypeSelectorProps = {
  /** The types this host offers, in the order they are shown. */
  types: GatewayType[];
  value: GatewayType;
  onChange: (type: GatewayType) => void;
  /** A gateway's type is fixed at creation — show only the card matching `value`, not the full picker. */
  readOnly?: boolean;
};

const GatewayTypeSelector: FC<GatewayTypeSelectorProps> = ({ types, value, onChange, readOnly }) => {
  // Read-only mode shows just the gateway's own type — taken from `value`, not
  // from `types`, so a gateway of a type this host no longer offers on create
  // still renders its card. The remaining slots stay empty so the visible card
  // keeps the same width it has in the full picker.
  const visibleTypes = readOnly ? [value] : types;
  const spacerCount = readOnly ? Math.max(types.length - 1, 0) : 0;

  return (
    <Box sx={{ display: 'flex', gap: 2, mt: 1 }}>
      {visibleTypes.map((type) => {
        const { icon: Icon, badge, comingSoon } = TYPE_CARDS[type];
        return (
          <GatewayTypeCard
            key={type}
            icon={<Icon size={20} />}
            label={gatewayTypeLabel(type)}
            badge={badge}
            selected={value === type}
            disabled={comingSoon}
            onClick={() => onChange(type)}
          />
        );
      })}
      {Array.from({ length: spacerCount }, (_, index) => (
        <Box key={`spacer-${index}`} sx={{ flex: 1 }} />
      ))}
    </Box>
  );
};

export default GatewayTypeSelector;
