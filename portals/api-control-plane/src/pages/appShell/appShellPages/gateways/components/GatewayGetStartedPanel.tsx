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

import { useState, type ReactElement, type ReactNode } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  Grid,
  Stack,
  Tab,
  Tabs,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Server } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import { useRotateGatewayToken, type Gateway } from '@/api/resources/gateways';
import dockerIconUrl from '@/assets/icons/docker.svg';
import helmIconUrl from '@/assets/icons/helm.svg';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useNotifications } from '@/components/Notifications';
import { gatewayEndpoint } from '../utils/gatewayDisplay';
import { environmentForGateway } from '../utils/gatewayEnvironments';
import {
  configureCommand,
  downloadCommand,
  enterDirectoryCommand,
  helmInstallCommand,
  runtimeCheckCommand,
  setupTarget,
  startCommand,
  TOKEN_PLACEHOLDER,
} from '../utils/gatewaySetup';
import { CopyableCommand } from './CopyableCommand';

const messages = defineMessages({
  chartHeading: {
    id: 'gateways.detail.GetStarted.k8s.chartHeading',
    defaultMessage: 'Installing the Chart',
    description: 'Heading above the Helm command that installs the gateway into a cluster.',
  },
  chartIntro: {
    id: 'gateways.detail.GetStarted.k8s.chartIntro',
    defaultMessage:
      'Run this command to install the gateway chart with control plane configurations:',
  },
  downloadIntro: {
    id: 'gateways.detail.GetStarted.download.intro',
    defaultMessage: 'Run this command in your terminal to download the gateway:',
  },
  environmentLabel: {
    id: 'gateways.detail.GetStarted.environment.label',
    defaultMessage: 'Associated Environment',
  },
  prerequisiteCurl: {
    id: 'gateways.detail.GetStarted.prerequisite.curl',
    defaultMessage: 'cURL installed',
  },
  prerequisiteDocker: {
    id: 'gateways.detail.GetStarted.prerequisite.docker',
    defaultMessage: 'Docker installed and running',
  },
  prerequisiteHelm: {
    id: 'gateways.detail.GetStarted.prerequisite.helm',
    defaultMessage: 'Helm 3.18+',
  },
  prerequisiteKubernetes: {
    id: 'gateways.detail.GetStarted.prerequisite.kubernetes',
    defaultMessage: 'Kubernetes 1.32+',
  },
  prerequisitesHeading: {
    id: 'gateways.detail.GetStarted.prerequisites',
    defaultMessage: 'Prerequisites',
  },
  prerequisiteUnzip: {
    id: 'gateways.detail.GetStarted.prerequisite.unzip',
    defaultMessage: 'unzip installed',
  },
  reconfigure: {
    id: 'gateways.detail.GetStarted.reconfigure',
    defaultMessage: 'Reconfigure',
    description: 'Generates a fresh registration token, revoking the gateway’s previous one.',
  },
  reconfigureConfirmBody: {
    id: 'gateways.detail.GetStarted.reconfigure.confirm.body',
    defaultMessage:
      'This revokes the current registration token and disconnects any gateway still using it. You will need to apply the new token to bring it back online.',
  },
  reconfigureConfirmButton: {
    id: 'gateways.detail.GetStarted.reconfigure.confirm.button',
    defaultMessage: 'Generate new token',
  },
  reconfigureConfirmCancel: {
    id: 'gateways.detail.GetStarted.reconfigure.confirm.cancel',
    defaultMessage: 'Cancel',
  },
  reconfigureConfirmTitle: {
    id: 'gateways.detail.GetStarted.reconfigure.confirm.title',
    defaultMessage: 'Generate a new registration token?',
  },
  reconfiguring: {
    id: 'gateways.detail.GetStarted.reconfiguring',
    defaultMessage: 'Generating…',
  },
  runtimeIntro: {
    id: 'gateways.detail.GetStarted.vm.runtimeIntro',
    defaultMessage: 'Ensure docker and docker compose commands are available.',
  },
  runtimeOptionColima: {
    id: 'gateways.detail.GetStarted.vm.runtime.colima',
    defaultMessage: 'Colima (macOS)',
    description: 'A container runtime option. Product name — do not translate “Colima”.',
  },
  runtimeOptionDockerDesktop: {
    id: 'gateways.detail.GetStarted.vm.runtime.dockerDesktop',
    defaultMessage: 'Docker Desktop (Windows / macOS)',
    description: 'A container runtime option. Product name — do not translate “Docker Desktop”.',
  },
  runtimeOptionDockerEngine: {
    id: 'gateways.detail.GetStarted.vm.runtime.dockerEngine',
    defaultMessage: 'Docker Engine + Compose plugin (Linux)',
    description: 'A container runtime option. Product names — do not translate.',
  },
  runtimeOptionRancher: {
    id: 'gateways.detail.GetStarted.vm.runtime.rancher',
    defaultMessage: 'Rancher Desktop (Windows / macOS)',
    description: 'A container runtime option. Product name — do not translate “Rancher Desktop”.',
  },
  runtimesIntro: {
    id: 'gateways.detail.GetStarted.vm.runtimesIntro',
    defaultMessage: 'A Docker-compatible container runtime such as:',
  },
  startEnter: {
    id: 'gateways.detail.GetStarted.start.enter',
    defaultMessage: '1. Navigate to the gateway folder.',
  },
  startRun: {
    id: 'gateways.detail.GetStarted.start.run',
    defaultMessage:
      '2. Run this command to start the gateway using the configs/keys.env file created in Step 2:',
    description: 'configs/keys.env is a file path — keep it exactly as written.',
  },
  stepConfigure: {
    id: 'gateways.detail.GetStarted.step.configure',
    defaultMessage: 'Step 2: Configure the Gateway',
  },
  stepDownload: {
    id: 'gateways.detail.GetStarted.step.download',
    defaultMessage: 'Step 1: Download the Gateway',
  },
  stepStart: {
    id: 'gateways.detail.GetStarted.step.start',
    defaultMessage: 'Step 3: Start the Gateway',
  },
  tabDocker: {
    id: 'gateways.detail.GetStarted.tab.docker',
    defaultMessage: 'Docker',
    description: 'Product name — do not translate.',
  },
  tabKubernetes: {
    id: 'gateways.detail.GetStarted.tab.kubernetes',
    defaultMessage: 'Kubernetes',
    description: 'Product name — do not translate.',
  },
  tabQuickStart: {
    id: 'gateways.detail.GetStarted.tab.quickStart',
    defaultMessage: 'Quick Start',
  },
  tabVirtualMachine: {
    id: 'gateways.detail.GetStarted.tab.virtualMachine',
    defaultMessage: 'Virtual Machine',
  },
  title: {
    id: 'gateways.detail.GetStarted.title',
    defaultMessage: 'Get Started',
  },
  tokenNoticeCompose: {
    id: 'gateways.detail.GetStarted.token.notice.compose',
    defaultMessage:
      'The registration token is single-use. If you need to reconfigure the gateway, generate a new token — this will revoke the old token and disconnect the gateway from the control plane.',
  },
  tokenNoticeHelm: {
    id: 'gateways.detail.GetStarted.token.notice.helm',
    defaultMessage:
      'The registration token is a one-time generated token for this gateway. If you need to install or update the gateway chart again, first reconfigure this gateway to generate a new registration token. Reconfiguring will revoke the previous token.',
  },
  tokenOnce: {
    id: 'gateways.detail.GetStarted.token.once',
    defaultMessage: 'Copy this now. The token is shown only once and cannot be retrieved again.',
  },
  urlLabel: {
    id: 'gateways.detail.GetStarted.url.label',
    defaultMessage: 'URL',
  },
});

/** Which install path the panel is showing. */
type SetupTab = 'quickStart' | 'virtualMachine' | 'docker' | 'kubernetes';

/** Icon size shared by every tab label. */
const TAB_ICON_SIZE = 18;

/**
 * Vendor mark from `src/assets/icons`, sized to match the lucide icons.
 * `aria-hidden` because the tab label already provides the name.
 * It must forward `className` so `Tab` can apply icon spacing.
 */
function BrandTabIcon({ className, src }: { className?: string; src: string }) {
  return (
    <Box
      alt=""
      aria-hidden
      // `Tab` clones its icon to attach `MuiTab-icon`, and its own
      // `& > .MuiTab-icon` rule is what puts a gap between icon and label.
      // Dropping the prop here silently drops the spacing with it.
      className={className}
      component="img"
      src={src}
      sx={{
        display: 'block',
        height: TAB_ICON_SIZE,
        objectFit: 'contain',
        width: TAB_ICON_SIZE,
      }}
    />
  );
}

/** Install paths, in display order. */
const TABS: {
  icon?: ReactElement;
  label: MessageDescriptor;
  prerequisites: MessageDescriptor[];
  value: SetupTab;
}[] = [
  {
    label: messages.tabQuickStart,
    prerequisites: [
      messages.prerequisiteCurl,
      messages.prerequisiteUnzip,
      messages.prerequisiteDocker,
    ],
    value: 'quickStart',
  },
  {
    icon: <Server size={TAB_ICON_SIZE} />,
    label: messages.tabVirtualMachine,
    prerequisites: [messages.prerequisiteCurl, messages.prerequisiteUnzip],
    value: 'virtualMachine',
  },
  {
    icon: <BrandTabIcon src={dockerIconUrl} />,
    label: messages.tabDocker,
    prerequisites: [messages.prerequisiteCurl, messages.prerequisiteUnzip],
    value: 'docker',
  },
  {
    icon: <BrandTabIcon src={helmIconUrl} />,
    label: messages.tabKubernetes,
    prerequisites: [
      messages.prerequisiteCurl,
      messages.prerequisiteUnzip,
      messages.prerequisiteKubernetes,
      messages.prerequisiteHelm,
    ],
    value: 'kubernetes',
  },
];

/** Container runtimes the virtual-machine path accepts. */
const VM_RUNTIMES = [
  messages.runtimeOptionDockerDesktop,
  messages.runtimeOptionRancher,
  messages.runtimeOptionColima,
  messages.runtimeOptionDockerEngine,
];

/** A bulleted list of translated lines. */
function BulletList({ items }: { items: MessageDescriptor[] }) {
  return (
    <Box component="ul" sx={{ m: 0, pl: 3 }}>
      {items.map((item) => (
        <Typography color="text.secondary" component="li" key={item.id} variant="body2">
          <FormattedMessage {...item} />
        </Typography>
      ))}
    </Box>
  );
}

/**
 * One numbered step. The heading takes the primary tone so the steps stay
 * findable while scrolling a wall of shell commands.
 */
function SetupStep({ children, title }: { children: ReactNode; title: MessageDescriptor }) {
  return (
    <Stack spacing={1}>
      <Typography color="primary.main" sx={{ fontWeight: 600 }} variant="subtitle1">
        <FormattedMessage {...title} />
      </Typography>
      {children}
    </Stack>
  );
}

export type GatewayGetStartedPanelProps = {
  /** Control plane host the gateway agent registers against. */
  controlPlaneHost: string;
  gateway: Gateway;
  gatewayId: string;
};

/**
 * Everything a user needs to bring a self-hosted gateway online: where it will
 * live, and the commands to get it there on each supported platform.
 *
 * The generated token is deliberately component state and nothing else. It is
 * returned once by the rotate call and is not retrievable afterwards, so it is
 * never written to the query cache; leaving the page is what discards it, and
 * that is the intended lifetime.
 */
export function GatewayGetStartedPanel({
  controlPlaneHost,
  gateway,
  gatewayId,
}: GatewayGetStartedPanelProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const rotateToken = useRotateGatewayToken(gatewayId);

  const [tab, setTab] = useState<SetupTab>('quickStart');
  const [token, setToken] = useState<string>();
  const [confirmOpen, setConfirmOpen] = useState(false);

  const target = setupTarget(gateway, controlPlaneHost);
  const tokenValue = token ?? TOKEN_PLACEHOLDER;
  const environment = environmentForGateway(gateway);
  const activeTab = TABS.find((entry) => entry.value === tab) ?? TABS[0];

  const reconfigure = () => {
    setConfirmOpen(false);
    rotateToken.mutate(undefined, {
      onSuccess: (result) => {
        setToken(result.token);
        notify(intl.formatMessage(messages.tokenOnce), 'success');
      },
    });
  };

  /**
   * Step 2 is the same on every path: the same warning, the same button, and —
   * once a token exists; the same env file. Only Kubernetes leaves out the
   * file, because the token goes into the Helm command instead.
   */
  const configureStep = (notice: MessageDescriptor, showEnvFile: boolean) => (
    <SetupStep title={messages.stepConfigure}>
      <Typography color="text.secondary" variant="body2">
        <FormattedMessage {...notice} />
      </Typography>
      <Box>
        <Button
          disabled={rotateToken.isPending}
          onClick={() => setConfirmOpen(true)}
          variant="outlined"
        >
          <FormattedMessage
            {...(rotateToken.isPending ? messages.reconfiguring : messages.reconfigure)}
          />
        </Button>
      </Box>
      {token && (
        <Alert severity="warning">
          <FormattedMessage {...messages.tokenOnce} />
        </Alert>
      )}
      {token && showEnvFile && <CopyableCommand code={configureCommand(target, token)} />}
    </SetupStep>
  );

  return (
    <Card sx={{ p: 3 }}>
      <Stack spacing={3}>
        <Typography sx={{ fontWeight: 700 }} variant="h5">
          <FormattedMessage {...messages.title} />
        </Typography>

        {/* Where this gateway sits. Read-only: the address is fixed at
            provisioning, and the environment is a property of the record; both
            are shown here because every command below depends on them. */}
        <Grid container spacing={2}>
          <Grid size={{ md: 6, xs: 12 }}>
            <TextField
              fullWidth
              label={intl.formatMessage(messages.urlLabel)}
              slotProps={{ input: { readOnly: true } }}
              value={gatewayEndpoint(gateway)}
            />
          </Grid>
          <Grid size={{ md: 6, xs: 12 }}>
            <TextField
              fullWidth
              label={intl.formatMessage(messages.environmentLabel)}
              slotProps={{ input: { readOnly: true } }}
              // Environment names are data, not copy — passed through, never
              // translated.
              value={environment.name}
            />
          </Grid>
        </Grid>

        <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
          <Tabs
            onChange={(_event, next: SetupTab) => setTab(next)}
            value={tab}
            variant="scrollable"
          >
            {TABS.map((entry) => (
              <Tab
                icon={entry.icon}
                iconPosition="start"
                key={entry.value}
                label={intl.formatMessage(entry.label)}
                value={entry.value}
              />
            ))}
          </Tabs>
        </Box>

        <Stack spacing={3}>
          <Stack spacing={1}>
            <Typography sx={{ fontWeight: 600 }} variant="subtitle1">
              <FormattedMessage {...messages.prerequisitesHeading} />
            </Typography>
            <BulletList items={activeTab.prerequisites} />
          </Stack>

          {/* The virtual-machine path is the only one that has to say what
              "a container runtime" means, and how to check for one. */}
          {tab === 'virtualMachine' && (
            <Stack spacing={1}>
              <Typography color="text.secondary" variant="body2">
                <FormattedMessage {...messages.runtimesIntro} />
              </Typography>
              <BulletList items={VM_RUNTIMES} />
              <Typography color="text.secondary" variant="body2">
                <FormattedMessage {...messages.runtimeIntro} />
              </Typography>
              <CopyableCommand code={runtimeCheckCommand()} />
            </Stack>
          )}

          {tab === 'kubernetes' ? (
            <>
              {configureStep(messages.tokenNoticeHelm, false)}
              <SetupStep title={messages.chartHeading}>
                <Typography color="text.secondary" variant="body2">
                  <FormattedMessage {...messages.chartIntro} />
                </Typography>
                <CopyableCommand code={helmInstallCommand(target, gatewayId, tokenValue)} />
              </SetupStep>
            </>
          ) : (
            <>
              <SetupStep title={messages.stepDownload}>
                <Typography color="text.secondary" variant="body2">
                  <FormattedMessage {...messages.downloadIntro} />
                </Typography>
                <CopyableCommand code={downloadCommand(target)} />
              </SetupStep>

              {configureStep(messages.tokenNoticeCompose, true)}

              <SetupStep title={messages.stepStart}>
                <Typography color="text.secondary" variant="body2">
                  <FormattedMessage {...messages.startEnter} />
                </Typography>
                <CopyableCommand code={enterDirectoryCommand(target)} />
                <Typography color="text.secondary" variant="body2">
                  <FormattedMessage {...messages.startRun} />
                </Typography>
                <CopyableCommand code={startCommand()} />
              </SetupStep>
            </>
          )}
        </Stack>
      </Stack>

      <ConfirmDialog
        cancelLabel={intl.formatMessage(messages.reconfigureConfirmCancel)}
        confirmLabel={intl.formatMessage(messages.reconfigureConfirmButton)}
        destructive
        loading={rotateToken.isPending}
        message={intl.formatMessage(messages.reconfigureConfirmBody)}
        onCancel={() => setConfirmOpen(false)}
        onConfirm={reconfigure}
        open={confirmOpen}
        title={intl.formatMessage(messages.reconfigureConfirmTitle)}
      />
    </Card>
  );
}
