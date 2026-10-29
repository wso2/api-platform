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
import {
  Box,
  Button,
  Card,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Radio,
  RadioGroup,
  Typography,
} from '@wso2/oxygen-ui';
import { Cloud, Server } from '@wso2/oxygen-ui-icons-react';
import GatewayProvisioningCard from './GatewayProvisioningCard';
import { getActiveColorScheme, subscribeToColorSchemeChanges } from '../utils/colorScheme';

// The console toggles light/dark via a DOM attribute/class, not by swapping
// MUI's theme object, so `useTheme().palette.mode` never reflects the live
// toggle (it stays on whatever it was initialized with) — that's why the
// diagram kept rendering as if it were always in one mode regardless of the
// actual theme. Read the DOM directly instead, same as the rest of this
// codebase (see `ai-workspace/src/utils/colorScheme.ts`, this file's source).
const useActiveColorScheme = () => {
  const [scheme, setScheme] = useState(getActiveColorScheme);
  useEffect(() => subscribeToColorSchemeChanges(setScheme), []);
  return scheme;
};

export type NoGatewayStateProps = {
  /** The API being deployed; shown on the animation's chip and in the heading. */
  apiName: string;
  /** The pipeline's environments, none of which has a gateway yet. */
  environments: string[];
  /** Creates the self-hosted gateway in the chosen environment. */
  onAddSelfHosted: (environment: string) => void;
  /** Creates the managed gateway in the chosen environment. */
  onAddCloud: (environment: string) => void;
  /** Set while the managed gateway is being created and until it is active. */
  provisioning?: { startedAt: number; environment: string };
  /** Set while the self-hosted gateway registration request is in flight. */
  selfHostedPending?: { startedAt: number };
};

const SELF_HOSTED_STEP_LABELS: [string, string, string] = [
  'Registering gateway',
  'Provisioning configuration',
  'Waiting for it to connect',
];

const ACCENT = '#f07a3e';
const ACCENT_SOFT = '#ffb88f';
const SUCCESS = '#5cc98f';
const SLOT = 52;

// Ported 1:1 from the approved reference (gw-* class names/timings) rather than
// re-derived, so the motion matches exactly. Only the rgba(255,255,255,X) /
// rgba(0,0,0,X) "foreground" alphas are theme-branched — the reference was
// dark-only, and those faint white-based lines/borders are invisible on a
// light surface, which is what made light mode unreadable before.
const useDiagramColors = () => {
  const dark = useActiveColorScheme() === 'dark';
  const fg = dark ? '255,255,255' : '0,0,0';
  // Even at a bumped alpha, a pure-white line still read as "black"/invisible
  // against this console's warm near-black background — raising opacity
  // further wasn't enough, the hue itself needed to stop fighting the warm
  // backdrop. Dark mode now uses a warm pale tint (255,214,181) instead of
  // white for these structural lines/borders; light mode is untouched since
  // it was already fine.
  const warm = '255,214,181';
  return {
    apiBorder: dark ? `rgba(${warm},0.85)` : `rgba(${fg},0.18)`,
    apiBg: dark ? `rgba(${warm},0.12)` : `rgba(${fg},0.03)`,
    line: dark ? `rgba(${warm},0.8)` : `rgba(${fg},0.14)`,
    emptyBorder: dark ? `rgba(${warm},0.9)` : `rgba(${fg},0.25)`,
    travelDot: dark ? ACCENT_SOFT : '#c2540f',
    blockBorder: dark ? '#f07a3e' : 'rgba(194,84,15,0.8)',
    blockBgFrom: dark ? 'rgba(240,122,62,0.28)' : 'rgba(240,122,62,0.26)',
    blockBgTo: dark ? 'rgba(240,122,62,0.1)' : 'rgba(240,122,62,0.08)',
    bar1: dark ? ACCENT_SOFT : '#9a3f0d',
    bar2: dark ? 'rgba(255,184,143,0.85)' : 'rgba(154,63,13,0.65)',
    bar3: dark ? 'rgba(255,184,143,0.65)' : 'rgba(154,63,13,0.4)',
    outLine: dark ? '#ffb88f' : 'rgba(154,63,13,0.55)',
    outDot: dark ? SUCCESS : '#1f9d5c',
  };
};

// One 4.5s loop, ported 1:1 from the approved reference: the slot sits empty
// (dashed) while a request dot travels in, the gateway block then pops into
// the slot, the 3 output lines cascade in just after (staggered via
// animation-delay), everything holds, then the block fades back out to the
// empty dashed state and the loop repeats.
const KEYFRAMES = `
@keyframes apip-gw-slot {
  0%, 8%   { opacity: 0; transform: translateY(-14px) scale(.85); }
  22%, 82% { opacity: 1; transform: translateY(0) scale(1); }
  94%,100% { opacity: 0; transform: translateY(0) scale(1); }
}
@keyframes apip-gw-empty {
  0%, 8%   { opacity: 1; }
  22%, 82% { opacity: 0; }
  94%,100% { opacity: 1; }
}
@keyframes apip-gw-req {
  0%, 24%  { left: 0; opacity: 0; }
  28%      { opacity: 1; }
  44%      { left: 100%; opacity: 1; }
  46%,100% { left: 100%; opacity: 0; }
}
@keyframes apip-gw-out {
  0%, 40%  { opacity: 0; transform: translateX(-4px); }
  50%, 80% { opacity: 1; transform: translateX(0); }
  92%,100% { opacity: 0; }
}
@media (prefers-reduced-motion: reduce) {
  .apip-gw-anim * { animation: none !important; opacity: 1 !important; }
  .apip-gw-anim .apip-gw-empty { opacity: 0 !important; }
  .apip-gw-anim .apip-gw-block, .apip-gw-anim .apip-gw-out { opacity: 1 !important; transform: none !important; }
}
`;

const LOOP_TIME = '3.6s';

const GatewayAnimation: FC<{ apiName: string }> = ({ apiName }) => {
  const c = useDiagramColors();

  return (
    <Box className="apip-gw-anim" aria-hidden sx={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
      <style>{KEYFRAMES}</style>

      <Box
        title={apiName}
        sx={{
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
          fontSize: '11.5px',
          px: '11px',
          py: '7px',
          maxWidth: 160,
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
          borderRadius: '8px',
          border: '1px solid',
          borderColor: c.apiBorder,
          bgcolor: c.apiBg,
        }}
      >
        {apiName}
      </Box>

      <Box sx={{ position: 'relative', width: 56, height: '1px', bgcolor: c.line }}>
        <Box
          sx={{
            position: 'absolute',
            top: -2,
            left: 0,
            ml: '-2.5px',
            width: 5,
            height: 5,
            borderRadius: '50%',
            bgcolor: c.travelDot,
            boxShadow: `0 0 8px ${c.travelDot}cc`,
            animation: `apip-gw-req ${LOOP_TIME} ease-in infinite`,
          }}
        />
      </Box>

      <Box sx={{ position: 'relative', width: SLOT, height: SLOT }}>
        <Box
          className="apip-gw-empty"
          sx={{
            position: 'absolute',
            inset: 0,
            borderRadius: '13px',
            border: '1px dashed',
            borderColor: c.emptyBorder,
            animation: `apip-gw-empty ${LOOP_TIME} ease-in-out infinite`,
          }}
        />
        <Box
          className="apip-gw-block"
          sx={{
            position: 'absolute',
            inset: 0,
            borderRadius: '13px',
            border: '1px solid',
            borderColor: c.blockBorder,
            background: `linear-gradient(180deg, ${c.blockBgFrom}, ${c.blockBgTo})`,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            gap: '4px',
            animation: `apip-gw-slot ${LOOP_TIME} cubic-bezier(.2,.8,.2,1) infinite`,
          }}
        >
          <Box sx={{ width: 22, height: 3, borderRadius: '2px', bgcolor: c.bar1 }} />
          <Box sx={{ width: 22, height: 3, borderRadius: '2px', bgcolor: c.bar2 }} />
          <Box sx={{ width: 22, height: 3, borderRadius: '2px', bgcolor: c.bar3 }} />
        </Box>
      </Box>

      <Box sx={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
        {[0, 1, 2].map((i) => (
          <Box
            key={i}
            className="apip-gw-out"
            sx={{
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              animation: `apip-gw-out ${LOOP_TIME} ease-out infinite`,
              animationDelay: `${i * 0.12}s`,
            }}
          >
            <Box sx={{ width: 26, height: '1px', bgcolor: c.outLine }} />
            <Box
              sx={{
                width: 6,
                height: 6,
                borderRadius: '50%',
                bgcolor: c.outDot,
                boxShadow: `0 0 6px ${c.outDot}b3`,
              }}
            />
          </Box>
        ))}
      </Box>
    </Box>
  );
};

type OptionProps = {
  icon: ReactNode;
  title: string;
  description: string;
  action: string;
  primary?: boolean;
  onClick: () => void;
};

const Option: FC<OptionProps> = ({ icon, title, description, action, primary, onClick }) => {
  const c = useDiagramColors();

  return (
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
          borderColor: primary ? c.blockBorder : 'divider',
          color: primary ? c.travelDot : 'text.primary',
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
      <Button
        variant={primary ? 'contained' : 'outlined'}
        onClick={onClick}
        sx={{
          flexShrink: 0,
          borderRadius: 999,
          textTransform: 'none',
          ...(primary && {
            bgcolor: ACCENT,
            color: '#1a0d05',
            '&:hover': { bgcolor: ACCENT_SOFT },
          }),
        }}
      >
        {action}
      </Button>
    </Box>
  );
};

/**
 * Shown instead of the pipeline when the project has environments but none of
 * them has a gateway, so there is nowhere to deploy yet.
 */
const NoGatewayState: FC<NoGatewayStateProps> = ({
  apiName,
  environments,
  onAddSelfHosted,
  onAddCloud,
  provisioning,
  selfHostedPending,
}) => {
  // Which kind of gateway the environment picker is choosing for; null when closed.
  const [picking, setPicking] = useState<'cloud' | 'self-hosted' | null>(null);
  const [selected, setSelected] = useState('');

  const create = (kind: 'cloud' | 'self-hosted', environment: string) =>
    kind === 'cloud' ? onAddCloud(environment) : onAddSelfHosted(environment);

  // One environment needs no question; several ask which one gets the gateway.
  const handleAdd = (kind: 'cloud' | 'self-hosted') => {
    if (environments.length === 1) {
      create(kind, environments[0]);
      return;
    }
    setSelected(environments[0] ?? '');
    setPicking(kind);
  };

  return (
    <Box sx={{ maxWidth: 640, mx: 'auto', pt: 6, textAlign: 'center' }}>
      <Box sx={{ display: 'flex', justifyContent: 'center', mb: 3 }}>
        <GatewayAnimation apiName={apiName} />
      </Box>
      <Typography variant="h4" sx={{ mb: 0.5, fontWeight: 500 }}>
        Add a gateway to deploy{' '}
        <Box component="span" sx={{ fontWeight: 700 }}>
          {apiName}
        </Box>
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 4, opacity: 0.65 }}>
        Gateways run your API in an environment. Add one to start building and deploying.
      </Typography>
      <Card sx={{ border: '1px solid', borderColor: 'divider', textAlign: 'left', overflow: 'hidden' }}>
        {selfHostedPending ? (
          <GatewayProvisioningCard
            startedAt={selfHostedPending.startedAt}
            icon={<Server size={20} />}
            title="Self-hosted gateway"
            description="Run the gateway in your own infrastructure with full control over network, data and scaling."
            stepLabels={SELF_HOSTED_STEP_LABELS}
          />
        ) : (
          <Option
            primary
            icon={<Server size={20} />}
            title="Self-hosted gateway"
            description="Run the gateway in your own infrastructure with full control over network, data and scaling."
            action="Add self-hosted gateway"
            onClick={() => handleAdd('self-hosted')}
          />
        )}
        {provisioning ? (
          <GatewayProvisioningCard startedAt={provisioning.startedAt} />
        ) : (
          <Option
            icon={<Cloud size={20} />}
            title="WSO2 managed gateway"
            description="A gateway hosted and maintained by WSO2. Ready in one click and bound to your development environment."
            action="Add Cloud gateway"
            onClick={() => handleAdd('cloud')}
          />
        )}
      </Card>

      <Dialog open={picking !== null} onClose={() => setPicking(null)} maxWidth="xs" fullWidth>
        <DialogTitle sx={{ fontSize: 16, fontWeight: 600 }}>Choose an environment</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
            The gateway is created in the environment you pick.
          </Typography>
          <RadioGroup value={selected} onChange={(event) => setSelected(event.target.value)}>
            {environments.map((name) => (
              <FormControlLabel key={name} value={name} control={<Radio />} label={name} />
            ))}
          </RadioGroup>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button onClick={() => setPicking(null)}>Cancel</Button>
          <Button
            variant="contained"
            disabled={!selected}
            onClick={() => {
              if (picking) create(picking, selected);
              setPicking(null);
            }}
          >
            Create gateway
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default NoGatewayState;
