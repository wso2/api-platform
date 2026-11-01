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
import { Box, Button, Card, Tooltip, Typography } from '@wso2/oxygen-ui';
import { Cloud, Globe, Server } from '@wso2/oxygen-ui-icons-react';
import { getActiveColorScheme, subscribeToColorSchemeChanges } from '../utils/colorScheme';
import type { GatewayHosting } from '../types';

// The console toggles light/dark through a DOM attribute/class rather than by
// swapping MUI's theme object, so `useTheme().palette.mode` never follows the
// toggle. Read the DOM instead, as the rest of this codebase does.
const useActiveColorScheme = () => {
  const [scheme, setScheme] = useState(getActiveColorScheme);
  useEffect(() => subscribeToColorSchemeChanges(setScheme), []);
  return scheme;
};

export type NoGatewayStateProps = {
  /** Opens the Add Gateway form with the chosen hosting already selected. */
  onAdd: (hosting: GatewayHosting) => void;
  /** The organization's gateway quota is full — both buttons are disabled. */
  disabled?: boolean;
  /** Why creating is blocked; shown on hover over a disabled button. */
  disabledMessage?: string;
};

// One 6s loop: the empty slot (ghost) is replaced by the gateway block, its bars
// drop in, a ring pulses, packets flow from the APIs into the gateway and out to
// the environment, then everything fades back to the empty slot.
const KEYFRAMES = `
.gw-flow{display:flex;align-items:center;gap:16px;
  --gw-api-border:rgba(247,190,150,.6);--gw-api-bg:rgba(60,32,18,.55);--gw-api-text:#f3e3d6;
  --gw-line-l:rgba(247,160,102,.6);--gw-line-r:rgba(255,214,181,.5);
  --gw-fill-from:#5a2c14;--gw-fill-to:#3a1c0c;
  --gw-env-idle:rgba(255,214,181,.5);--gw-env-idle-text:#c9bdb4}
.gw-flow.light{--gw-api-border:rgba(194,84,15,.4);--gw-api-bg:rgba(240,122,62,.08);--gw-api-text:#4a2a16;
  --gw-line-l:rgba(194,84,15,.4);--gw-line-r:rgba(0,0,0,.2);
  --gw-fill-from:#fbd9c0;--gw-fill-to:#f7c3a0;
  --gw-env-idle:rgba(0,0,0,.25);--gw-env-idle-text:#7a6a60}
.gw-api{display:flex;align-items:center;gap:10px;height:52px;padding:0 18px;border-radius:12px;border:1px solid var(--gw-api-border);background:var(--gw-api-bg);font:15px ui-monospace,SFMono-Regular,Menlo,monospace;color:var(--gw-api-text)}
.gw-api b{color:#f7a066;font-weight:400}
.gw-line{position:relative;width:96px;height:2px;background:repeating-linear-gradient(90deg,var(--gw-line-l) 0 6px,transparent 6px 12px)}
.gw-line.right{background:repeating-linear-gradient(90deg,var(--gw-line-r) 0 6px,transparent 6px 12px)}
.gw-line .lit{position:absolute;inset:0;background:rgba(247,160,102,.55);animation:gwLinkL 6s ease-in-out infinite}
.gw-line.right .lit{animation-name:gwLinkR}
.gw-line .packets{position:absolute;inset:0;animation:gwFlowL 6s ease-in-out infinite}
.gw-line.right .packets{animation-name:gwFlowR}
.gw-line .packets i{position:absolute;top:-3px;width:8px;height:8px;margin-left:-4px;border-radius:50%;background:#ffb27a;box-shadow:0 0 10px #f7882f;animation:gwPacket 1.2s linear infinite}
.gw-line .packets i:nth-child(2){animation-delay:.4s}
.gw-line .packets i:nth-child(3){animation-delay:.8s}
.gw-line.right .packets i{animation-delay:.2s}
.gw-line.right .packets i:nth-child(2){animation-delay:.6s}
.gw-line.right .packets i:nth-child(3){animation-delay:1s}
.gw-node{position:relative;width:80px;height:80px}
.gw-node > *{position:absolute;inset:0;border-radius:20px}
.gw-ghost{border:1.5px dashed rgba(247,136,47,.55);animation:gwGhost 6s ease-in-out infinite}
.gw-ring{border:1.5px solid #f7882f;animation:gwRing 6s ease-out infinite}
.gw-fill{border:1.5px solid #f7882f;background:linear-gradient(180deg,var(--gw-fill-from),var(--gw-fill-to));animation:gwOn 6s ease-in-out infinite}
.gw-bars{display:flex;flex-direction:column;align-items:center;justify-content:center;gap:6px}
.gw-bars i{width:36px;height:4px;border-radius:2px;animation:gwBar 6s ease-out infinite}
.gw-bars i:nth-child(1){background:#ffb27a}
.gw-bars i:nth-child(2){background:#f7a066;animation-delay:.25s}
.gw-bars i:nth-child(3){background:#c98057;animation-delay:.5s}
.gw-env{width:64px;height:64px;border-radius:16px;border:1px dashed var(--gw-env-idle);color:var(--gw-env-idle-text);display:flex;align-items:center;justify-content:center;animation:gwEnvOn 6s ease-in-out infinite}
@keyframes gwGhost{0%,16%{opacity:1}28%,90%{opacity:0}100%{opacity:1}}
@keyframes gwOn{0%,16%{opacity:0}28%,90%{opacity:1}100%{opacity:0}}
@keyframes gwBar{0%,12%{opacity:0;transform:translateY(-10px) scaleX(.3)}24%,90%{opacity:1;transform:none}97%,100%{opacity:0}}
@keyframes gwRing{0%,30%{transform:scale(1);opacity:0}34%{opacity:.8}50%,100%{transform:scale(1.5);opacity:0}}
@keyframes gwLinkL{0%,32%{opacity:0}40%,88%{opacity:1}96%,100%{opacity:0}}
@keyframes gwLinkR{0%,44%{opacity:0}52%,88%{opacity:1}96%,100%{opacity:0}}
@keyframes gwFlowL{0%,36%{opacity:0}42%,86%{opacity:1}92%,100%{opacity:0}}
@keyframes gwFlowR{0%,48%{opacity:0}54%,86%{opacity:1}92%,100%{opacity:0}}
@keyframes gwPacket{0%{left:0;opacity:0}15%,85%{opacity:1}100%{left:100%;opacity:0}}
@keyframes gwEnvOn{
  0%,54%,96%,100%{border-color:var(--gw-env-idle);color:var(--gw-env-idle-text);box-shadow:none}
  62%,88%{border-color:rgba(247,136,47,.85);color:#f7882f;box-shadow:0 0 24px rgba(247,136,47,.3)}
}
@media (prefers-reduced-motion:reduce){
  .gw-flow *{animation:none!important}
  .gw-ghost{opacity:0}
  .gw-fill,.gw-bars i{opacity:1!important}
}
`;

const GatewayFlow: FC = () => {
  const light = useActiveColorScheme() === 'light';
  return (
    <>
      <style>{KEYFRAMES}</style>
      <div className={`gw-flow${light ? ' light' : ''}`} aria-hidden>
        <div className="gw-api">
          <b>{'{ }'}</b>
          <span>Your APIs</span>
        </div>
        <div className="gw-line">
          <span className="lit" />
          <span className="packets">
            <i />
            <i />
            <i />
          </span>
        </div>
        <div className="gw-node">
          <span className="gw-ghost" />
          <span className="gw-ring" />
          <span className="gw-fill" />
          <div className="gw-bars">
            <i />
            <i />
            <i />
          </div>
        </div>
        <div className="gw-line right">
          <span className="lit" />
          <span className="packets">
            <i />
            <i />
            <i />
          </span>
        </div>
        <div className="gw-env" title="Environment">
          <Globe size={28} />
        </div>
      </div>
    </>
  );
};

const ACCENT = '#f7882f';

type OptionProps = {
  icon: ReactNode;
  title: string;
  description: string;
  action: string;
  primary?: boolean;
  onClick: () => void;
  /** Quota is full — the button is disabled and explains why on hover. */
  disabled?: boolean;
  disabledMessage?: string;
};

const Option: FC<OptionProps> = ({
  icon,
  title,
  description,
  action,
  primary,
  onClick,
  disabled,
  disabledMessage,
}) => (
  <Box
    sx={{
      display: 'flex',
      alignItems: 'center',
      gap: 2,
      p: 2,
      '&:not(:last-of-type)': { borderBottom: '1px solid', borderColor: 'divider' },
    }}
  >
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
        borderColor: primary ? ACCENT : 'divider',
        color: primary ? ACCENT : 'text.primary',
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
    <Tooltip title={disabled ? disabledMessage ?? '' : ''}>
      <span style={{ display: 'inline-flex', flexShrink: 0 }}>
        <Button
          variant={primary ? 'contained' : 'outlined'}
          onClick={onClick}
          disabled={disabled}
          sx={{
            flexShrink: 0,
            borderRadius: 999,
            textTransform: 'none',
            ...(primary
              ? {
                  color: '#fff',
                  border: 0,
                  background: 'linear-gradient(90deg, #f0903f, #e0553d)',
                  '&:hover': { background: 'linear-gradient(90deg, #f7a066, #ea6a4f)' },
                }
              : { color: ACCENT, borderColor: ACCENT, '&:hover': { borderColor: ACCENT, bgcolor: 'rgba(247,136,47,.08)' } }),
          }}
        >
          {action}
        </Button>
      </span>
    </Tooltip>
  </Box>
);

/**
 * The Gateways page's empty state: an animated picture of APIs flowing through a
 * gateway into an environment, and the two ways to add the first one. Each button
 * opens the Add Gateway form with that hosting already chosen.
 */
const NoGatewayState: FC<NoGatewayStateProps> = ({ onAdd, disabled, disabledMessage }) => (
  <Box sx={{ maxWidth: 640, mx: 'auto', pt: 6, textAlign: 'center' }}>
    <Box sx={{ display: 'flex', justifyContent: 'center', mb: 3 }}>
      <GatewayFlow />
    </Box>
    <Typography variant="h4" sx={{ mb: 0.5, fontWeight: 500 }}>
      Connect your first gateway
    </Typography>
    <Typography variant="body2" color="text.secondary" sx={{ mb: 4, opacity: 0.65 }}>
      A gateway routes traffic to your APIs in an environment. Choose how you want to run it.
    </Typography>
    <Card sx={{ border: '1px solid', borderColor: 'divider', textAlign: 'left', overflow: 'hidden' }}>
      <Option
        primary
        icon={<Server size={20} />}
        title="Self-hosted gateway"
        description="Run the gateway in your own infrastructure with full control over network, data and scaling."
        action="Add self-hosted gateway"
        onClick={() => onAdd('self-hosted')}
        disabled={disabled}
        disabledMessage={disabledMessage}
      />
      <Option
        icon={<Cloud size={20} />}
        title="WSO2 managed gateway"
        description="A gateway hosted and maintained by WSO2. Ready in one click and bound to your development environment."
        action="Add Cloud gateway"
        onClick={() => onAdd('managed')}
        disabled={disabled}
        disabledMessage={disabledMessage}
      />
    </Card>
  </Box>
);

export default NoGatewayState;
