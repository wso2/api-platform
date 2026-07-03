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

import { useCallback, useEffect, useState } from 'react';
import {
  Link as RouterLink,
  useLocation,
  useNavigate,
  useParams,
} from 'react-router-dom';
import {
  Alert,
  Avatar,
  Box,
  Button,
  Card,
  CircularProgress,
  IconButton,
  PageContent,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft, Clock, Edit } from '@wso2/oxygen-ui-icons-react';
import {
  formatRelativeTime,
  useApplications,
} from '../../../../contexts/ApplicationsContext';
import { useAppShell } from '../../../../contexts/AppShellContext';
import {
  buildOrgPath,
  buildProjectPath,
} from '../../../../utils/projectRouting';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { PLATFORM_API_BASE_URL } from '../../../../config.env';
import { applicationApis } from '../../../../apis/applicationApis';
import type { Application } from '../../../../utils/types';
import { ApplicationAssociationsProvider } from '../../../../contexts/ApplicationAssociationsContext';
import AssociationsTab from './OverviewTabs/AssociationsTab';

type LocationState = {
  applicationAdded?: boolean;
};

function getErrorDescription(error: unknown, fallback: string): string {
  return (
    (error as any)?.response?.data?.description ||
    (error as any)?.response?.data?.message ||
    (error instanceof Error ? error.message : null) ||
    fallback
  );
}


export default function ApplicationOverview() {
  const { applicationId } = useParams<{ applicationId: string }>();
  const { getApplicationById } = useApplications();

  const navigate = useNavigate();
  const location = useLocation();
  const showSnackbar = useAIWorkspaceSnackbar();
  const { currentProject, currentOrganization } = useAppShell();
  const apimBaseUrl = PLATFORM_API_BASE_URL;
  const isProjectLevel = Boolean(currentProject?.id);
  const applicationsPath = isProjectLevel
    ? buildProjectPath(currentOrganization, currentProject, '/applications')
    : buildOrgPath(currentOrganization, '/applications');

  const [application, setApplication] = useState<Application | null>(null);
  const [isApplicationLoading, setIsApplicationLoading] = useState(true);
  const [applicationError, setApplicationError] = useState<string | null>(null);

  const loadApplication = useCallback(async () => {
    if (!applicationId || !currentOrganization?.uuid) {
      setApplication(null);
      setIsApplicationLoading(false);
      return;
    }

    try {
      setIsApplicationLoading(true);
      setApplicationError(null);

      const cachedApplication = getApplicationById(applicationId);
      if (cachedApplication) {
        setApplication(cachedApplication);
      }

      const fetchedApplication = await applicationApis.getApplication(
        applicationId,
        apimBaseUrl
      );
      setApplication(fetchedApplication);
    } catch (error) {
      setApplicationError(
        getErrorDescription(error, 'Failed to load application details.')
      );
      setApplication(null);
    } finally {
      setIsApplicationLoading(false);
    }
  }, [
    applicationId,
    currentOrganization?.uuid,
    getApplicationById,
    apimBaseUrl,
  ]);

  useEffect(() => {
    void loadApplication();
  }, [loadApplication]);

  useEffect(() => {
    const state = location.state as LocationState | null;
    if (state?.applicationAdded) {
      showSnackbar('Application created successfully.', 'success');
      navigate(location.pathname, { replace: true, state: null });
    }
  }, [location.pathname, location.state, navigate, showSnackbar]);

  if (isApplicationLoading) {
    return (
      <PageContent fullWidth>
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
          <CircularProgress />
        </Box>
      </PageContent>
    );
  }

  if (applicationError || !application) {
    return (
      <PageContent fullWidth>
        <Stack spacing={2}>
          <Alert severity="error">
            {applicationError || 'Application not found'}
          </Alert>
          <Button component={RouterLink} to={applicationsPath}>
            Back to list
          </Button>
        </Stack>
      </PageContent>
    );
  }

  const lastUpdated =
    application.lastUpdated ?? application.updatedAt ?? application.createdAt;

  return (
    <PageContent fullWidth>
      <Button
        component={RouterLink}
        to={applicationsPath}
        size="small"
        startIcon={<ChevronLeft size={24} />}
      >
        Back to list
      </Button>

      <Stack spacing={3} sx={{ mt: 2 }}>
        <Card>
          <Box sx={{ p: 2 }}>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'flex-start',
                gap: 2,
              }}
            >
              <Avatar
                sx={{
                  width: 72,
                  height: 72,
                  fontWeight: 600,
                  flexShrink: 0,
                  fontSize: 28,
                  bgcolor: 'primary.light',
                  color: 'primary.contrastText',
                }}
              >
                {(application.displayName || '—')
                  .trim()
                  .slice(0, 2)
                  .toUpperCase()}
              </Avatar>

              <Stack spacing={0.75} sx={{ minWidth: 0, flex: 1 }}>
                <Stack
                  direction="row"
                  spacing={1}
                  alignItems="center"
                  flexWrap="wrap"
                >
                  <Typography variant="h3">
                    {application.displayName || '—'}
                  </Typography>
                  <Tooltip title="Edit Application">
                    <IconButton
                      component={RouterLink}
                      to={`${applicationsPath}/${applicationId}/edit`}
                      size="small"
                    >
                      <Edit size={16} />
                    </IconButton>
                  </Tooltip>
                </Stack>
                <Typography variant="body2" color="text.secondary">
                  {application.description || '—'}
                </Typography>
                <Stack direction="row" spacing={0.75} alignItems="center">
                  <Typography variant="caption" color="text.secondary">
                    Last updated
                  </Typography>
                  <Clock size={14} />
                  <Typography variant="caption" color="text.secondary">
                    {formatRelativeTime(lastUpdated)}
                  </Typography>
                </Stack>
              </Stack>
            </Box>
          </Box>
        </Card>

        <Box
          sx={{
            display: 'flex',
            alignItems: 'flex-start',
            justifyContent: 'space-between',
            gap: 2,
          }}
        >
          <Box>
            <Typography variant="h5" fontWeight={600}>
              Associated GenAI Resources
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
              Manage the AI providers, proxies, and API keys connected to this
              application.
            </Typography>
          </Box>
        </Box>

        <Card>
          <Box sx={{ p: 2 }}>
            {applicationId ? (
              <ApplicationAssociationsProvider applicationId={applicationId}>
                <AssociationsTab />
              </ApplicationAssociationsProvider>
            ) : null}
          </Box>
        </Card>
      </Stack>
    </PageContent>
  );
}
