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

import { useEffect, useState, type FC, type ReactNode } from 'react';
import { Box, Typography } from '@wso2/oxygen-ui';
import { Check, Cloud } from '@wso2/oxygen-ui-icons-react';
import { getActiveColorScheme, subscribeToColorSchemeChanges } from '../utils/colorScheme';

// Same fix as `NoGatewayState.tsx`: the console toggles light/dark via a DOM
// attribute/class, so `useTheme().palette.mode` never reflects the live
// toggle — read the DOM directly instead.
const useActiveColorScheme = () => {
  const [scheme, setScheme] = useState(getActiveColorScheme);
  useEffect(() => subscribeToColorSchemeChanges(setScheme), []);
  return scheme;
};

export type GatewayProvisioningCardProps = {
  /** When creation was requested; the first two steps are paced from it. */
  startedAt: number;
  icon?: ReactNode;
  title?: string;
  description?: string;
  stepLabels?: [string, string, string];
};

const ACCENT = '#f07a3e';
const SUCCESS = '#5cc98f';

const DEFAULT_STEP_LABELS: [string, string, string] = [
  'Allocating gateway runtime',
  'Configuring TLS and routes',
  'Binding to environment',
];
// The first two steps are paced by time; the last waits for the gateway to be
// really active, so the bar creeps toward — but never reaches — the end.
const STEP_ONE_MS = 4000;
const STEP_TWO_MS = 9000;

const progressAt = (elapsed: number): number => {
  if (elapsed < STEP_ONE_MS) return (elapsed / STEP_ONE_MS) * 33;
  if (elapsed < STEP_TWO_MS) return 33 + ((elapsed - STEP_ONE_MS) / (STEP_TWO_MS - STEP_ONE_MS)) * 33;
  return 66 + 29 * (1 - Math.exp(-(elapsed - STEP_TWO_MS) / 20000));
};

const KEYFRAMES = `
@keyframes apip-gw-spin { to { transform: rotate(360deg); } }
@keyframes apip-gw-pulse { 0%, 100% { opacity: 1; } 50% { opacity: .45; } }
@media (prefers-reduced-motion: reduce) {
  .apip-gw-prov * { animation: none !important; }
}
`;

/** The managed-gateway card while the gateway is being built. */
const GatewayProvisioningCard: FC<GatewayProvisioningCardProps> = ({
  startedAt,
  icon = <Cloud size={20} />,
  title = 'WSO2 managed gateway',
  description = 'A gateway hosted and maintained by WSO2. Ready in one click and bound to your development environment.',
  stepLabels = DEFAULT_STEP_LABELS,
}) => {
  const dark = useActiveColorScheme() === 'dark';
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 250);
    return () => clearInterval(timer);
  }, []);

  const elapsed = Math.max(0, now - startedAt);
  const percent = Math.floor(progressAt(elapsed));
  const activeStep = elapsed < STEP_ONE_MS ? 0 : elapsed < STEP_TWO_MS ? 1 : 2;
  const accent = dark ? ACCENT : '#c2540f';

  return (
    <Box className="apip-gw-prov" sx={{ p: 2 }}>
      <style>{KEYFRAMES}</style>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
        <Box
          sx={{
            width: 40,
            height: 40,
            flexShrink: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            borderRadius: '10px',
            border: '1px solid',
            borderColor: 'divider',
          }}
        >
          {icon}
        </Box>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>
            {title}
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ opacity: 0.65 }}>
            {description}
          </Typography>
        </Box>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexShrink: 0 }}>
          <Box
            sx={{
              width: 22,
              height: 22,
              borderRadius: '50%',
              border: '2px solid',
              borderColor: 'divider',
              borderTopColor: accent,
              animation: 'apip-gw-spin 0.9s linear infinite',
            }}
          />
          <Typography
            sx={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace', fontSize: 14, minWidth: 44 }}
          >
            {percent} %
          </Typography>
        </Box>
      </Box>

      <Box sx={{ mt: 2, height: 4, borderRadius: 2, bgcolor: 'action.hover', overflow: 'hidden' }}>
        <Box
          sx={{
            height: '100%',
            width: `${percent}%`,
            bgcolor: accent,
            transition: 'width 0.3s linear',
          }}
        />
      </Box>

      <Box sx={{ mt: 1.5, display: 'flex', flexWrap: 'nowrap', gap: 2.5, overflow: 'hidden' }}>
        {stepLabels.map((label, index) => {
          const done = index < activeStep;
          const active = index === activeStep;
          return (
            <Box
              key={label}
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 1,
                minWidth: 0,
                whiteSpace: 'nowrap',
                opacity: done || active ? 1 : 0.45,
              }}
            >
              <Box
                sx={{
                  width: 18,
                  height: 18,
                  borderRadius: '50%',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  border: '1.5px solid',
                  borderColor: done ? 'transparent' : active ? accent : 'divider',
                  bgcolor: done ? 'rgba(92,201,143,0.18)' : 'transparent',
                  color: SUCCESS,
                  animation: active ? 'apip-gw-pulse 1.4s ease-in-out infinite' : undefined,
                }}
              >
                {done && <Check size={11} />}
              </Box>
              <Typography variant="body2">{label}</Typography>
            </Box>
          );
        })}
      </Box>
    </Box>
  );
};

export default GatewayProvisioningCard;
