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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {
  Box,
  Button,
  ButtonBase,
  Card,
  CardContent,
  Chip,
  Grid,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { ArrowRight, Network, PanelTop, Plus } from '@wso2/oxygen-ui-icons-react';
import type { ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useNavigate } from 'react-router-dom';

import { useApiPortals } from '@/api/resources/apiPortals';
import { useGateways } from '@/api/resources/gateways';
import { ConvergingStrandsIllustration } from '@/components/illustrations/ConvergingStrandsIllustration';
import { useGreetingName } from '@/hooks/useGreetingName';
import type { LimitedComponent } from '@/hooks/useResourceLimits';
import { Can } from '@/permissions/Can';
import { CanCreate } from '@/permissions/CanCreate';
import { routes } from '@/routes/paths';
import { ProjectHeaderActions } from './ProjectMetadata';
import { ApiTypeTicker } from './ApiTypeTicker';
import ExploreMoreCard from '../../organizations/components/ExploreMoreCard';

const messages = defineMessages({
  greeting: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectWelcome.greeting',
    defaultMessage: 'Hello, {name}',
    description: '{name} is the name the user is greeted by. Never translated.',
  },
  intro: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectWelcome.intro',
    defaultMessage:
      "You're in the <strong>{projectName}</strong> project. It's empty for now, so start creating.",
    description: '{projectName} is the name of the project the user is in. Never translated.',
  },
  startHere: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectWelcome.startHere',
    defaultMessage: 'Start here',
    description: 'Small label above the main call to action on an empty project.',
  },
  heroTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectWelcome.heroTitle',
    defaultMessage: 'Expose and secure APIs',
  },
  heroDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectWelcome.heroDescription',
    defaultMessage:
      'Turn any service into a managed API with OAuth2, rate limits, and policies. Import a definition or point to a backend URL.',
  },
  createApi: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectWelcome.createApi',
    defaultMessage: 'Create API',
  },
  gatewayTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectWelcome.gatewayTitle',
    defaultMessage: 'Run on any gateway',
  },
  gatewayDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectWelcome.gatewayDescription',
    defaultMessage: 'Deploy to a managed gateway or a self-hosted one in your own cluster.',
  },
  gatewayCount: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectWelcome.gatewayCount',
    defaultMessage: '{count, plural, one {# gateway} other {# gateways}}',
    description: 'Chip showing how many gateways the organization has.',
  },
  gatewayConnect: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectWelcome.gatewayConnect',
    defaultMessage: 'Connect a gateway',
  },
  gatewayManage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectWelcome.gatewayManage',
    defaultMessage: 'Manage gateways',
  },
  portalTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectWelcome.portalTitle',
    defaultMessage: 'Publish to developers',
  },
  portalDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectWelcome.portalDescription',
    defaultMessage: 'List APIs in a developer portal so consumers can subscribe and get keys.',
  },
  portalCount: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectWelcome.portalCount',
    defaultMessage: '{count, plural, one {# portal} other {# portals}}',
    description: 'Chip showing how many developer portals exist.',
  },
  portalCreate: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectWelcome.portalCreate',
    defaultMessage: 'Create a portal',
  },
  portalManage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectWelcome.portalManage',
    defaultMessage: 'Manage portals',
  },
});

type SideCardProps = {
  action: ReactNode;
  /**
   * Whether the action creates something. Only creating counts against the
   * organization's limit; opening what exists must stay available at the limit.
   */
  creates: boolean;
  /** The operation (and quota) the action is gated on, exactly as the organization overview gates its cards. */
  component?: LimitedComponent;
  operationId: string;
  chip?: string;
  description: string;
  icon: ReactNode;
  onAction: () => void;
  title: string;
};

function SideCard({
  action,
  chip,
  creates,
  component,
  description,
  icon,
  onAction,
  operationId,
  title,
}: SideCardProps) {
  const actionButton = (
    <ButtonBase
      onClick={onAction}
      sx={{ alignSelf: 'flex-start', color: 'primary.main', gap: 0.5, mt: 2 }}
    >
      <Typography sx={{ fontWeight: 600 }} variant="body2">
        {action}
      </Typography>
      <ArrowRight size={16} />
    </ButtonBase>
  );
  return (
    <Card
      sx={{
        '@keyframes sideCardRiseIn': {
          from: { opacity: 0, transform: 'translateY(24px)' },
          to: { opacity: 1, transform: 'translateY(0)' },
        },
        // Both cards rise in together once the welcome view is on screen.
        '@media (prefers-reduced-motion: no-preference)': {
          animation: 'sideCardRiseIn 0.5s ease-out both',
        },
        display: 'flex',
        flex: 1,
        flexDirection: 'column',
        justifyContent: 'center',
        px: 3,
        py: 3,
      }}
    >
      <Stack spacing={0.75}>
        <Stack alignItems="center" direction="row" justifyContent="space-between" spacing={1}>
          <Stack alignItems="center" direction="row" spacing={1} sx={{ minWidth: 0 }}>
            <Box sx={{ color: 'primary.main', display: 'flex' }}>{icon}</Box>
            <Typography noWrap sx={{ fontWeight: 600 }} variant="subtitle2">
              {title}
            </Typography>
          </Stack>
          {chip && (
            <Chip
              color="success"
              label={chip}
              size="small"
              sx={{ flexShrink: 0 }}
              variant="outlined"
            />
          )}
        </Stack>
        <Typography color="text.secondary" sx={{ mt: 1.5 }} variant="body2">
          {description}
        </Typography>
      </Stack>
      {creates ? (
        <CanCreate component={component} denied="hide" do={operationId}>
          {actionButton}
        </CanCreate>
      ) : (
        <Can denied="hide" do={operationId}>
          {actionButton}
        </Can>
      )}
    </Card>
  );
}

export type ProjectWelcomeProps = {
  /** Opens API creation for this project. */
  onCreateApi: () => void;
  /** The greeting falls back to this when no extension supplies the user's name. */
  orgDisplayName: string;
  orgHandle: string;
  projectName: string;
};

/**
 * What an empty project shows in place of an API list: a greeting, the one
 * thing to do first, where the APIs will run and be published, and where to
 * read more. The gateway and portal cards follow what the organization already
 * has, so a user who has set those up sees their count and a way to manage them.
 */
export function ProjectWelcome({
  onCreateApi,
  orgDisplayName,
  orgHandle,
  projectName,
}: ProjectWelcomeProps) {
  const intl = useIntl();
  const navigate = useNavigate();
  const greetingName = useGreetingName(orgDisplayName);
  const gatewaysQuery = useGateways({ limit: 100 });
  const portalsQuery = useApiPortals();

  const gatewayCount =
    gatewaysQuery.isPending || gatewaysQuery.error
      ? undefined
      : (gatewaysQuery.data?.pagination.total ?? 0);
  // Switching orgs keeps prior data via `keepPreviousData`; do not flash the
  // previous organization's count.
  const portalCount =
    portalsQuery.isPending || portalsQuery.isPlaceholderData || portalsQuery.error
      ? undefined
      : (portalsQuery.data?.pagination.total ?? 0);

  // While a count is unknown the card offers "Manage", which is right either way.
  const hasGateways = (gatewayCount ?? 0) > 0;
  const hasPortals = (portalCount ?? 0) > 0;
  const manageGateways = hasGateways || gatewayCount === undefined;
  const managePortals = hasPortals || portalCount === undefined;

  return (
    <Stack spacing={4}>
      <Stack
        alignItems={{ sm: 'flex-start' }}
        direction={{ sm: 'row', xs: 'column' }}
        justifyContent="space-between"
        spacing={2}
      >
        <Box sx={{ minWidth: 0 }}>
          <Typography sx={{ fontSize: '1.75rem', fontWeight: 800 }} variant="h3">
            <FormattedMessage
              {...messages.greeting}
              values={{
                name: (
                  <Box
                    component="span"
                    sx={(theme) => ({
                      backgroundImage: `linear-gradient(90deg, ${(theme.vars?.palette ?? theme.palette).primary.main}, ${(theme.vars?.palette ?? theme.palette).primary.light})`,
                      backgroundClip: 'text',
                      WebkitBackgroundClip: 'text',
                      color: 'transparent',
                      WebkitTextFillColor: 'transparent',
                    })}
                  >
                    {greetingName}
                  </Box>
                ),
              }}
            />
          </Typography>
          <Typography color="text.secondary" sx={{ mt: 1 }} variant="body1">
            <FormattedMessage
              {...messages.intro}
              values={{
                projectName,
                strong: (chunks) => (
                  <Box component="strong" sx={{ color: 'text.primary', fontWeight: 600 }}>
                    {chunks}
                  </Box>
                ),
              }}
            />
          </Typography>
        </Box>
        <ProjectHeaderActions />
      </Stack>

      <Grid container spacing={2}>
        <Grid size={{ md: 8, xs: 12 }}>
          <Card
            sx={(theme) => {
              const palette = theme.vars?.palette ?? theme.palette;
              return {
                // The page's own background with a faint primary glow from the top-left, and a
                // primary-tinted edge: the same treatment as the other products' start cards.
                backgroundColor: palette.background.default,
                backgroundImage: `linear-gradient(135deg, color-mix(in srgb, ${palette.primary.main} 6%, ${palette.background.default}) 0%, ${palette.background.default} 60%)`,
                borderColor: `color-mix(in srgb, ${palette.primary.main} 30%, transparent)`,
                height: '100%',
              };
            }}
          >
            <CardContent
              sx={{
                p: { md: 5, xs: 3 },
                // MUI's CardContent shrinks its last-child bottom padding to 24px, which made the
                // card's top and bottom padding unequal.
                '&:last-child': { pb: { md: 5, xs: 3 } },
              }}
            >
              <Grid alignItems="center" container spacing={3}>
                <Grid size={{ md: 6, xs: 12 }}>
                  <Stack alignItems="flex-start" spacing={2.5}>
                    <Typography
                      color="primary"
                      sx={{
                        fontSize: '0.75rem',
                        fontWeight: 600,
                        letterSpacing: 1.5,
                        textTransform: 'uppercase',
                      }}
                      variant="subtitle2"
                    >
                      <FormattedMessage {...messages.startHere} />
                    </Typography>
                    <Typography sx={{ fontWeight: 600 }} variant="h3">
                      <FormattedMessage {...messages.heroTitle} />
                    </Typography>
                    <Typography
                      color="text.secondary"
                      sx={{ fontWeight: 400, lineHeight: 1.7 }}
                      variant="h6"
                    >
                      <FormattedMessage {...messages.heroDescription} />
                    </Typography>
                    <ApiTypeTicker />
                    <CanCreate component="restApis" denied="hide" do="CreateRESTAPI">
                      <Button
                        onClick={onCreateApi}
                        startIcon={<Plus size={18} />}
                        variant="contained"
                      >
                        <FormattedMessage {...messages.createApi} />
                      </Button>
                    </CanCreate>
                  </Stack>
                </Grid>
                <Grid size={{ md: 6, xs: 12 }} sx={{ display: { md: 'block', xs: 'none' } }}>
                  <ConvergingStrandsIllustration />
                </Grid>
              </Grid>
            </CardContent>
          </Card>
        </Grid>

        <Grid size={{ md: 4, xs: 12 }}>
          <Stack spacing={2} sx={{ height: '100%' }}>
            <SideCard
              action={
                <FormattedMessage
                  {...(manageGateways ? messages.gatewayManage : messages.gatewayConnect)}
                />
              }
              creates={!manageGateways}
              chip={
                hasGateways
                  ? intl.formatMessage(messages.gatewayCount, { count: gatewayCount })
                  : undefined
              }
              description={intl.formatMessage(messages.gatewayDescription)}
              icon={<Network size={18} />}
              onAction={() => navigate(routes.gateways(orgHandle))}
              operationId="CreateGateway"
              title={intl.formatMessage(messages.gatewayTitle)}
            />
            <SideCard
              action={
                <FormattedMessage
                  {...(managePortals ? messages.portalManage : messages.portalCreate)}
                />
              }
              creates={!managePortals}
              chip={
                hasPortals
                  ? intl.formatMessage(messages.portalCount, { count: portalCount })
                  : undefined
              }
              description={intl.formatMessage(messages.portalDescription)}
              icon={<PanelTop size={18} />}
              component="portals"
              onAction={() => navigate(routes.managedApiPortals(orgHandle))}
              operationId="CreateApiPortal"
              title={intl.formatMessage(messages.portalTitle)}
            />
          </Stack>
        </Grid>
      </Grid>

      <ExploreMoreCard />
    </Stack>
  );
}
