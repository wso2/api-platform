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

import { useState, type FC, type ReactNode } from 'react';
import { Box, Button, Typography } from '@wso2/oxygen-ui';
import {
  ArrowRight,
  Cloud,
  Database,
  Globe,
  LayoutGrid,
  Monitor,
  Network,
  Server,
} from '@wso2/oxygen-ui-icons-react';
import type { GatewayHosting } from '../types';

type CompareContent = {
  tabLabel: string;
  tabIcon: FC<{ size?: number }>;
  gatewayLabel: string;
  /** The three boxes on the "Backends" side of the diagram. */
  backendIcons: FC<{ size?: number }>[];
  summary: string;
  setupTime: string;
  trafficVia: string;
  bestFor: string;
};

const CONTENT: Record<GatewayHosting, CompareContent> = {
  managed: {
    tabLabel: 'WSO2 managed',
    tabIcon: Cloud,
    gatewayLabel: 'WSO2 cloud',
    backendIcons: [Cloud, Cloud, Cloud],
    summary:
      'Get APIs live instantly on fully managed cloud infrastructure. WSO2 runs, scales, and upgrades the gateway.',
    setupTime: 'Instant',
    trafficVia: 'WSO2 cloud',
    bestFor: 'Fast start, POCs',
  },
  'self-hosted': {
    tabLabel: 'Self-hosted',
    tabIcon: Server,
    gatewayLabel: 'Your infra',
    backendIcons: [Server, Database, Server],
    summary: 'Run the gateway on your own infrastructure. Traffic and data stay within your network.',
    setupTime: '1–5 minutes',
    trafficVia: 'Your infrastructure',
    bestFor: 'Enterprise, compliance',
  },
};

/** The three icons on the "Clients" side — the same for either hosting choice, since it's the gateway and backends that move, not who calls it. */
const CLIENT_ICONS: FC<{ size?: number }>[] = [Globe, Monitor, LayoutGrid];

const IconTile: FC<{ icon: ReactNode; tone?: 'neutral' | 'accent' }> = ({ icon, tone = 'neutral' }) => (
  <Box
    sx={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      width: 36,
      height: 36,
      borderRadius: '8px',
      border: '1px solid',
      borderColor: tone === 'accent' ? 'info.main' : 'divider',
      bgcolor: 'action.hover',
      color: tone === 'accent' ? 'info.main' : 'text.secondary',
    }}
  >
    {icon}
  </Box>
);

const Stat: FC<{ label: string; value: string }> = ({ label, value }) => (
  <Box>
    <Typography
      variant="caption"
      color="text.secondary"
      sx={{ fontWeight: 700, letterSpacing: 0.4, textTransform: 'uppercase', display: 'block', opacity: 0.7 }}
    >
      {label}
    </Typography>
    <Typography variant="body2" sx={{ fontWeight: 600 }}>
      {value}
    </Typography>
  </Box>
);

export type HostingCompareTableProps = {
  /**
   * Lets the comparison double as a picker: when given, a "Choose …" button
   * for each hosting appears below the diagram, and clicking one reports that
   * choice rather than just leaving the tab switched. Omit it to show the
   * comparison read-only, with no decision to make.
   */
  onChoose?: (hosting: GatewayHosting) => void;
};

/**
 * The content behind "Compare options" on the Hosting step: a self-contained
 * two-tab explainer (its tab is independent of the hosting radio above it —
 * it's for exploring both before deciding, not for confirming a choice
 * already made) with a client → gateway → backends diagram for whichever
 * hosting is being looked at.
 */
const HostingCompareTable: FC<HostingCompareTableProps> = ({ onChoose }) => {
  const [active, setActive] = useState<GatewayHosting>('managed');
  const content = CONTENT[active];

  return (
    <Box>
      <Box
        sx={{
          display: 'inline-flex',
          gap: 0.5,
          p: 0.5,
          mb: 2,
          borderRadius: '999px',
          bgcolor: 'action.hover',
        }}
      >
        {(Object.keys(CONTENT) as GatewayHosting[]).map((key) => {
          const tab = CONTENT[key];
          const selected = key === active;
          return (
            <Box
              key={key}
              role="tab"
              aria-selected={selected}
              tabIndex={0}
              onClick={() => setActive(key)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  setActive(key);
                }
              }}
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 0.75,
                px: 1.5,
                py: 0.75,
                borderRadius: '999px',
                cursor: 'pointer',
                bgcolor: selected ? 'primary.main' : 'transparent',
                color: selected ? 'primary.contrastText' : 'text.secondary',
                fontWeight: 600,
                fontSize: '0.8125rem',
                '&:hover': selected ? undefined : { bgcolor: 'action.selected' },
              }}
            >
              <tab.tabIcon size={14} />
              {tab.tabLabel}
            </Box>
          );
        })}
      </Box>

      <Box
        sx={{
          display: 'flex',
          flexWrap: 'wrap',
          gap: 3,
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: '10px',
          p: 2,
        }}
      >
        {/* Clients → Gateway → Backends */}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, alignItems: 'center' }}>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
              {CLIENT_ICONS.map((Icon, index) => (
                <IconTile key={index} icon={<Icon size={16} />} />
              ))}
            </Box>
            <Typography variant="caption" color="text.secondary">
              Clients
            </Typography>
          </Box>

          <Box sx={{ color: 'text.disabled', display: 'flex' }}>
            <ArrowRight size={16} />
          </Box>

          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, alignItems: 'center' }}>
            <Box
              sx={{
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                gap: 0.5,
                p: 1.25,
                border: '1px dashed',
                borderColor: 'primary.main',
                borderRadius: '10px',
              }}
            >
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  width: 40,
                  height: 40,
                  borderRadius: '8px',
                  bgcolor: 'primary.main',
                  color: 'primary.contrastText',
                }}
              >
                <Network size={18} />
              </Box>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                <content.tabIcon size={12} />
                <Typography variant="caption" sx={{ fontWeight: 600 }}>
                  {content.gatewayLabel}
                </Typography>
              </Box>
            </Box>
            <Typography variant="caption" color="text.secondary">
              Gateway
            </Typography>
          </Box>

          <Box sx={{ color: 'text.disabled', display: 'flex' }}>
            <ArrowRight size={16} />
          </Box>

          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, alignItems: 'center' }}>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
              {content.backendIcons.map((Icon, index) => (
                <IconTile key={index} icon={<Icon size={16} />} tone="accent" />
              ))}
            </Box>
            <Typography variant="caption" color="text.secondary">
              Backends
            </Typography>
          </Box>
        </Box>

        {/* Summary + stats */}
        <Box sx={{ flex: 1, minWidth: 220, display: 'flex', flexDirection: 'column', gap: 2 }}>
          <Typography variant="body1">{content.summary}</Typography>
          <Box sx={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
            <Stat label="Setup time" value={content.setupTime} />
            <Stat label="Traffic via" value={content.trafficVia} />
            <Stat label="Best for" value={content.bestFor} />
          </Box>
        </Box>
      </Box>

      {onChoose ? (
        <Box sx={{ display: 'flex', justifyContent: 'flex-end', alignItems: 'center', gap: 1, mt: 3 }}>
          {(Object.keys(CONTENT) as GatewayHosting[]).map((key) => (
            <Button key={key} variant={key === active ? 'contained' : 'text'} onClick={() => onChoose(key)}>
              Choose {CONTENT[key].tabLabel}
            </Button>
          ))}
        </Box>
      ) : null}
    </Box>
  );
};

export default HostingCompareTable;
