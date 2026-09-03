/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import type { ReactNode } from 'react';
import { useMemo, useState } from 'react';
import {
  Avatar,
  Box,
  Button,
  ButtonBase,
  Card,
  CardContent,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControl,
  FormLabel,
  Grid,
  IconButton,
  MenuItem,
  SearchBar,
  Select,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ArrowRight,
  Boxes,
  Clock,
  Network,
  PanelTop,
  Trash2,
  Workflow,
} from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useNavigate } from 'react-router-dom';

import { useOrganization } from '@/api/resources/organizations';
import { useGateways } from '@/api/resources/gateways';
import { useDeleteProject, type Project } from '@/api/resources/projects';
import { useRestApiCounts } from '@/api/resources/restApis';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useNotifications } from '@/components/Notifications';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { NewProjectDialog } from '@/pages/appShell/appShellPages/projects/components/NewProjectDialog';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { relativeTime } from '@/utils/relativeTime';
import ExploreMoreCard from './components/ExploreMoreCard';

const DOCS_BASE = 'https://wso2.com/api-platform/docs';

const messages = defineMessages({
  apiAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.apiAction',
    defaultMessage: 'Create an API',
  },
  apiDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.apiDescription',
    defaultMessage: 'Design, proxy, and version the APIs your organization builds.',
  },
  apiTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.apiTitle',
    defaultMessage: 'APIs',
  },
  bannerDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.bannerDescription',
    defaultMessage:
      'Design and build APIs inside projects, run them on gateways, and publish them to developer portals for your consumers.',
  },
  bannerTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.bannerTitle',
    defaultMessage: 'Welcome to {organizationName}',
    description: '{organizationName} is the display name of the organization. Never translated.',
  },
  developerPortalAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.developerPortalAction',
    defaultMessage: 'Create a portal',
  },
  developerPortalDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.developerPortalDescription',
    defaultMessage: 'Publish APIs so consumers can discover them and request access.',
  },
  developerPortalTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.developerPortalTitle',
    defaultMessage: 'Developer portals',
  },
  deleteAriaLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.deleteAriaLabel',
    defaultMessage: 'Delete {name}',
    description: 'Accessible label for the project row delete button.',
  },
  deleteConfirm: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.deleteConfirm',
    defaultMessage: 'Delete',
  },
  deleteConfirmInputLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.deleteConfirmInputLabel',
    defaultMessage: 'Project name',
  },
  deleteFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.deleteFailed',
    defaultMessage: 'Delete failed',
  },
  deleteMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.deleteMessage',
    defaultMessage:
      'This action is irreversible and all related project details will be lost. Type the project name below to confirm.',
  },
  deleteSucceeded: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.deleteSucceeded',
    defaultMessage: 'Deleted “{name}”.',
  },
  deleteTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.deleteTitle',
    defaultMessage: 'Are you sure you want to delete the project “{name}”?',
  },
  gatewayAction: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.gatewayAction',
    defaultMessage: 'Connect a gateway',
  },
  gatewayDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.gatewayDescription',
    defaultMessage: 'Run your APIs on a managed gateway or a self-hosted one.',
  },
  gatewayTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.gatewayTitle',
    defaultMessage: 'Gateways',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.loading',
    defaultMessage: 'Loading organization',
  },
  projectsAdd: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectsAdd',
    defaultMessage: 'New project',
  },
  projectsCount: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectsCount',
    defaultMessage: '{count, plural, one {# project} other {# projects}}',
  },
  projectsEmpty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectsEmpty',
    defaultMessage: 'No projects match your search.',
  },
  projectsSearch: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectsSearch',
    defaultMessage: 'Search projects',
  },
  projectApiCount: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectApiCount',
    defaultMessage: '{count} APIs',
  },
  projectsTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectsTitle',
    defaultMessage: 'Projects',
  },
  projectsViewAll: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectsViewAll',
    defaultMessage: 'View all',
  },
  projectSelectorCancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectSelectorCancel',
    defaultMessage: 'Cancel',
  },
  projectSelectorContinue: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectSelectorContinue',
    defaultMessage: 'Continue',
  },
  projectSelectorCreateProject: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectSelectorCreateProject',
    defaultMessage: 'Create project',
  },
  projectSelectorDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectSelectorDescription',
    defaultMessage: 'Select the project where you want to create the API.',
  },
  projectSelectorEmpty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectSelectorEmpty',
    defaultMessage: 'Create a project before creating an API.',
  },
  projectSelectorLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectSelectorLabel',
    defaultMessage: 'Project',
  },
  projectSelectorTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.projectSelectorTitle',
    defaultMessage: 'Choose a project',
  },
  unresolvedOrganization: {
    id: 'apiControlPlane.pages.appShell.appShellPages.organizations.OrganizationHomePage.unresolvedOrganization',
    defaultMessage:
      'Unable to resolve organization context. The organization API returned no usable organizations for this session.',
  },
});

type OverviewCardProps = {
  action: string;
  description: string;
  icon: ReactNode;
  metric: string;
  onAction: () => void;
  title: string;
};

function OverviewCard({ action, description, icon, metric, onAction, title }: OverviewCardProps) {
  return (
    <Card sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <CardContent sx={{ flexGrow: 1 }}>
        <Stack alignItems="flex-start" direction="row" spacing={1.5}>
          <Avatar
            sx={{
              bgcolor: 'action.hover',
              color: 'primary.main',
              height: 40,
              width: 40,
            }}
            variant="rounded"
          >
            {icon}
          </Avatar>
          <Stack spacing={0.75} sx={{ minWidth: 0 }}>
            <Stack alignItems="center" direction="row" spacing={1}>
              <Typography sx={{ fontWeight: 800 }} variant="h6">
                {title}
              </Typography>
              <Divider flexItem orientation="vertical" />
              <Typography color="text.secondary" sx={{ opacity: 0.7 }} variant="caption">
                {metric}
              </Typography>
            </Stack>
            <Typography color="text.secondary" sx={{ opacity: 0.7 }} variant="body2">
              {description}
            </Typography>
          </Stack>
        </Stack>
      </CardContent>
      <Divider />
      <ButtonBase onClick={onAction} sx={{ textAlign: 'left', width: '100%' }}>
        <Box
          sx={{
            alignItems: 'center',
            color: 'primary.main',
            display: 'flex',
            justifyContent: 'space-between',
            px: 2,
            py: 1.25,
            width: '100%',
          }}
        >
          <Typography sx={{ fontWeight: 700 }} variant="body2">
            {action}
          </Typography>
          <ArrowRight size={16} />
        </Box>
      </ButtonBase>
    </Card>
  );
}

export function OrganizationHomePage() {
  const intl = useIntl();
  const navigate = useNavigate();
  const { notify } = useNotifications();
  const { isLoading, organization, organizations, params, projects } = useConsoleScope();
  const orgHandle = params.orgHandle || '';
  const [createOpen, setCreateOpen] = useState(false);
  const [projectSelectorOpen, setProjectSelectorOpen] = useState(false);
  const [selectedProject, setSelectedProject] = useState('');
  const [search, setSearch] = useState('');
  const [projectToDelete, setProjectToDelete] = useState<Project | null>(null);
  const organizationQuery = useOrganization(orgHandle);
  const gatewaysQuery = useGateways({ limit: 100 });
  const deleteProjectMutation = useDeleteProject();
  const restApiCountsQuery = useRestApiCounts(projects.map((project) => project.id));
  const currentOrganization = organizationQuery.data || organization || organizations[0];

  const sortedProjects = useMemo(
    () =>
      [...projects].sort(
        (left, right) =>
          new Date(right.updatedAt || right.createdAt || 0).getTime() -
          new Date(left.updatedAt || left.createdAt || 0).getTime(),
      ),
    [projects],
  );

  const visibleProjects = useMemo(() => {
    const query = search.trim().toLowerCase();
    return sortedProjects
      .filter(
        (project) =>
          !query ||
          project.displayName.toLowerCase().includes(query) ||
          project.description?.toLowerCase().includes(query),
      )
      .slice(0, 5);
  }, [search, sortedProjects]);

  const createApi = () => {
    if (sortedProjects.length === 1) {
      navigate(routes.newApi(orgHandle, sortedProjects[0].id));
      return;
    }
    setSelectedProject(sortedProjects[0]?.id ?? '');
    setProjectSelectorOpen(true);
  };

  const continueToApiCreation = () => {
    if (!selectedProject) return;
    setProjectSelectorOpen(false);
    navigate(routes.newApi(orgHandle, selectedProject));
  };

  const confirmDeleteProject = () => {
    if (!projectToDelete) return;
    const { displayName, id } = projectToDelete;
    deleteProjectMutation.mutate(
      { projectId: id },
      {
        onError: () => notify(intl.formatMessage(messages.deleteFailed), 'error'),
        onSuccess: () => {
          notify(intl.formatMessage(messages.deleteSucceeded, { name: displayName }), 'success');
          setProjectToDelete(null);
        },
      },
    );
  };

  if (isLoading && !currentOrganization) {
    return <LoadingState label={intl.formatMessage(messages.loading)} />;
  }
  if (!currentOrganization) {
    return <ErrorState message={intl.formatMessage(messages.unresolvedOrganization)} />;
  }

  return (
    <>
      <Stack spacing={3}>
        <Box sx={{ py: 1 }}>
          <Typography sx={{ fontWeight: 800 }} variant="h3">
            <FormattedMessage
              {...messages.bannerTitle}
              values={{ organizationName: currentOrganization.displayName }}
            />
          </Typography>
          <Typography color="text.secondary" sx={{ mt: 1, maxWidth: 800 }} variant="body1">
            <FormattedMessage {...messages.bannerDescription} />
          </Typography>
        </Box>

        <Grid container spacing={2}>
          <Grid size={{ md: 4, xs: 12 }}>
            <OverviewCard
              action={intl.formatMessage(messages.apiAction)}
              description={intl.formatMessage(messages.apiDescription)}
              icon={<Workflow size={22} />}
              metric={
                restApiCountsQuery.isPending || restApiCountsQuery.error
                  ? '—'
                  : intl.formatNumber(restApiCountsQuery.total, {
                      minimumIntegerDigits: 2,
                      useGrouping: false,
                    })
              }
              onAction={createApi}
              title={intl.formatMessage(messages.apiTitle)}
            />
          </Grid>
          <Grid size={{ md: 4, xs: 12 }}>
            <OverviewCard
              action={intl.formatMessage(messages.gatewayAction)}
              description={intl.formatMessage(messages.gatewayDescription)}
              icon={<Network size={22} />}
              metric={
                gatewaysQuery.isPending || gatewaysQuery.error
                  ? '—'
                  : intl.formatNumber(gatewaysQuery.data?.pagination.total ?? 0, {
                      minimumIntegerDigits: 2,
                      useGrouping: false,
                    })
              }
              onAction={() => navigate(routes.gateways(orgHandle))}
              title={intl.formatMessage(messages.gatewayTitle)}
            />
          </Grid>
          <Grid size={{ md: 4, xs: 12 }}>
            <OverviewCard
              action={intl.formatMessage(messages.developerPortalAction)}
              description={intl.formatMessage(messages.developerPortalDescription)}
              icon={<PanelTop size={22} />}
              metric={intl.formatNumber(0, {
                minimumIntegerDigits: 2,
                useGrouping: false,
              })}
              onAction={() => window.open(`${DOCS_BASE}/cloud/dev-portal/`, '_blank', 'noopener')}
              title={intl.formatMessage(messages.developerPortalTitle)}
            />
          </Grid>
        </Grid>

        <Card sx={{ minHeight: 320 }}>
          <Box
            sx={{
              alignItems: { md: 'center' },
              display: 'flex',
              flexDirection: { md: 'row', xs: 'column' },
              gap: 2,
              justifyContent: 'space-between',
              p: 2,
            }}
          >
            <Stack alignItems="baseline" direction="row" spacing={1.5}>
              <Typography sx={{ fontWeight: 700 }} variant="h6">
                <FormattedMessage {...messages.projectsTitle} />
              </Typography>
              <Typography color="text.secondary" variant="body2">
                <FormattedMessage {...messages.projectsCount} values={{ count: projects.length }} />
              </Typography>
            </Stack>
            <Stack
              alignItems={{ sm: 'center' }}
              direction={{ sm: 'row', xs: 'column' }}
              spacing={1}
              width={{ md: 'auto', xs: '100%' }}
            >
              <SearchBar
                onChange={(event) => setSearch(event.target.value)}
                placeholder={intl.formatMessage(messages.projectsSearch)}
                size="small"
                value={search}
              />
              <Button
                onClick={() => navigate(routes.projects(orgHandle))}
                size="small"
                variant="text"
              >
                <FormattedMessage {...messages.projectsViewAll} />
              </Button>
              <Button onClick={() => setCreateOpen(true)} size="small" variant="outlined">
                <FormattedMessage {...messages.projectsAdd} />
              </Button>
            </Stack>
          </Box>
          <Divider />
          {visibleProjects.length ? (
            <>
              <Stack divider={<Divider />}>
                {visibleProjects.map((project) => (
                  <Box
                    key={project.id}
                    sx={{
                      alignItems: 'center',
                      display: 'flex',
                      maxWidth: '100%',
                      overflow: 'hidden',
                      '&:focus-within .project-delete-action, &:hover .project-delete-action': {
                        opacity: 1,
                      },
                    }}
                  >
                    <ButtonBase
                      onClick={() => navigate(routes.projectHome(orgHandle, project.id))}
                      sx={{
                        flexGrow: 1,
                        justifyContent: 'stretch',
                        minWidth: 0,
                        overflow: 'hidden',
                        textAlign: 'left',
                      }}
                    >
                      <Box
                        sx={{
                          alignItems: 'center',
                          display: 'flex',
                          gap: 1.5,
                          minWidth: 0,
                          pl: 2,
                          py: 1.5,
                          width: '100%',
                        }}
                      >
                        <Avatar
                          sx={{
                            bgcolor: 'primary.main',
                            color: 'primary.contrastText',
                            height: 32,
                            width: 32,
                          }}
                        >
                          {project.displayName.charAt(0).toUpperCase()}
                        </Avatar>
                        <Box sx={{ flexGrow: 1, minWidth: 0, overflow: 'hidden' }}>
                          <Typography noWrap sx={{ fontWeight: 600 }} variant="body2">
                            {project.displayName}
                          </Typography>
                          {project.description && (
                            <Typography
                              color="text.secondary"
                              noWrap
                              sx={{ display: 'block', maxWidth: '80%', overflow: 'hidden' }}
                              variant="caption"
                            >
                              {project.description}
                            </Typography>
                          )}
                        </Box>
                        <Typography
                          color="text.secondary"
                          sx={{ flexShrink: 0, minWidth: 72 }}
                          variant="caption"
                        >
                          <FormattedMessage
                            {...messages.projectApiCount}
                            values={{ count: restApiCountsQuery.counts[project.id] ?? '—' }}
                          />
                        </Typography>
                        {project.updatedAt && (
                          <Stack
                            alignItems="center"
                            direction="row"
                            justifyContent="flex-start"
                            spacing={0.5}
                            sx={{
                              flexShrink: 0,
                              maxWidth: 128,
                              minWidth: 104,
                              // width: 128,
                            }}
                          >
                            <Clock aria-hidden="true" size={14} />
                            <Typography color="text.secondary" noWrap variant="caption">
                              {relativeTime(project.updatedAt)}
                            </Typography>
                          </Stack>
                        )}
                      </Box>
                    </ButtonBase>
                    <IconButton
                      aria-label={intl.formatMessage(messages.deleteAriaLabel, {
                        name: project.displayName,
                      })}
                      className="project-delete-action"
                      color="error"
                      onClick={() => setProjectToDelete(project)}
                      size="small"
                      sx={{
                        flexShrink: 0,
                        mr: 1.5,
                        opacity: { md: 0, xs: 1 },
                      }}
                    >
                      <Trash2 size={18} />
                    </IconButton>
                  </Box>
                ))}
              </Stack>
              {visibleProjects.length === 1 && <Divider />}
            </>
          ) : (
            <Stack alignItems="center" spacing={1} sx={{ py: 4 }}>
              <Boxes size={24} />
              <Typography color="text.secondary" variant="body2">
                <FormattedMessage {...messages.projectsEmpty} />
              </Typography>
            </Stack>
          )}
        </Card>

        <ExploreMoreCard />
      </Stack>

      <NewProjectDialog
        onClose={() => setCreateOpen(false)}
        open={createOpen}
        orgHandle={orgHandle}
      />
      <Dialog
        fullWidth
        maxWidth="xs"
        onClose={() => setProjectSelectorOpen(false)}
        open={projectSelectorOpen}
      >
        <DialogTitle>
          <FormattedMessage {...messages.projectSelectorTitle} />
        </DialogTitle>
        <DialogContent>
          <Stack spacing={1.5} sx={{ pt: 0.5 }}>
            <Typography color="text.secondary" variant="body2">
              <FormattedMessage
                {...(projects.length
                  ? messages.projectSelectorDescription
                  : messages.projectSelectorEmpty)}
              />
            </Typography>
            {projects.length > 0 && (
              <FormControl fullWidth>
                <FormLabel id="create-api-project-label">
                  <FormattedMessage {...messages.projectSelectorLabel} />
                </FormLabel>
                <Select
                  autoFocus
                  labelId="create-api-project-label"
                  onChange={(event) => setSelectedProject(String(event.target.value))}
                  size="small"
                  value={selectedProject}
                >
                  {sortedProjects.map((project) => (
                    <MenuItem key={project.id} value={project.id}>
                      {project.displayName}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            )}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button color="inherit" onClick={() => setProjectSelectorOpen(false)} variant="outlined">
            <FormattedMessage {...messages.projectSelectorCancel} />
          </Button>
          {projects.length ? (
            <Button disabled={!selectedProject} onClick={continueToApiCreation} variant="contained">
              <FormattedMessage {...messages.projectSelectorContinue} />
            </Button>
          ) : (
            <Button
              onClick={() => {
                setProjectSelectorOpen(false);
                setCreateOpen(true);
              }}
              variant="contained"
            >
              <FormattedMessage {...messages.projectSelectorCreateProject} />
            </Button>
          )}
        </DialogActions>
      </Dialog>
      <ConfirmDialog
        confirmInputLabel={intl.formatMessage(messages.deleteConfirmInputLabel)}
        confirmLabel={intl.formatMessage(messages.deleteConfirm)}
        confirmPhrase={projectToDelete?.displayName ?? ''}
        destructive
        loading={deleteProjectMutation.isPending}
        maxWidth="sm"
        message={projectToDelete ? intl.formatMessage(messages.deleteMessage) : ''}
        onCancel={() => setProjectToDelete(null)}
        onConfirm={confirmDeleteProject}
        open={projectToDelete !== null}
        title={intl.formatMessage(messages.deleteTitle, {
          name: projectToDelete?.displayName ?? '',
        })}
      />
    </>
  );
}
