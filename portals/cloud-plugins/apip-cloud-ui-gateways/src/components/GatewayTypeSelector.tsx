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
import { ShieldCheck, Waypoints } from '@wso2/oxygen-ui-icons-react';
import GatewayTypeCard from './GatewayTypeCard';
import type { GatewayType } from '../types';

const TYPE_CARDS: { type: GatewayType; icon: FC<{ size?: number }>; label: string; badge?: string }[] = [
  { type: 'ai', icon: ShieldCheck, label: 'AI Gateway' },
  { type: 'event', icon: Waypoints, label: 'Event Gateway', badge: 'Beta' },
];

export type GatewayTypeSelectorProps = {
  value: GatewayType;
  onChange: (type: GatewayType) => void;
  /** A gateway's type is fixed at creation — show only the card matching `value`, not the full picker. */
  readOnly?: boolean;
};

const GatewayTypeSelector: FC<GatewayTypeSelectorProps> = ({ value, onChange, readOnly }) => (
  <Box sx={{ display: 'flex', gap: 2, mt: 1 }}>
    {TYPE_CARDS.map(({ type, icon: Icon, label, badge }) => {
      // Read-only mode keeps both flex slots (so the visible card is the same
      // width as in the create-mode two-card layout) but only renders the
      // card matching the gateway's actual type — the other slot stays empty.
      if (readOnly && type !== value) return <Box key={type} sx={{ flex: 1 }} />;
      return (
        <GatewayTypeCard
          key={type}
          icon={<Icon size={20} />}
          label={label}
          badge={badge}
          selected={value === type}
          onClick={() => onChange(type)}
        />
      );
    })}
  </Box>
);

export default GatewayTypeSelector;
