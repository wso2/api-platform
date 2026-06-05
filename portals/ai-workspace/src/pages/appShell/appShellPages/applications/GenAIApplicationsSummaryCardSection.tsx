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

import { Link as RouterLink } from 'react-router-dom';
import {
  Avatar,
  Box,
  Button,
  Card,
  CardContent,
  CardHeader,
  Divider,
  ParticleBackground,
  Skeleton,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Clock, Plus } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import {
  formatRelativeTime,
  useApplications,
} from '../../../../contexts/ApplicationsContext';
import NoApplications from '../../../../assets/images/NoApplications.svg';
import ErrorAlert from '../../../../Components/common/ErrorAlert';

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length === 0) return '';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}

function truncateWords(text: string, maxWords: number): string {
  const words = text.trim().split(/\s+/);
  if (words.length <= maxWords) return text.trim();
  return `${words.slice(0, maxWords).join(' ')}…`;
}

function getHttpStatusCode(error?: Error | null): number | null {
  if (!error) return null;

  const axiosStatus = (error as any)?.response?.status;
  if (typeof axiosStatus === 'number') return axiosStatus;

  const match = error.message?.match(/status:\s*(\d{3})/i);
  if (match) return parseInt(match[1], 10);

  return null;
}

type GenAIApplicationsSummaryCardSectionProps = {
  applicationsPath: string;
  newApplicationPath: string;
  onApplicationClick: (applicationId: string) => void;
};

export default function GenAIApplicationsSummaryCardSection({
  applicationsPath,
  newApplicationPath,
  onApplicationClick,
}: GenAIApplicationsSummaryCardSectionProps) {
  const {
    applicationsResponse,
    isLoading,
    error,
    refreshApplications,
  } = useApplications();

  const applications = applicationsResponse.list;
  const errorStatusCode = getHttpStatusCode(error);
  const isNotFoundError = errorStatusCode === 404;
  const hasApplications = !error && applications.length > 0;
  const isEmptyState =
    !isLoading && (isNotFoundError || (!error && applications.length === 0));
  const showSeeMore = applicationsResponse.count > 6;

  return (
    <Card
      sx={{
        height: '100%',
        width: '100%',
        minHeight: 300,
        ...(isEmptyState
          ? {
              display: 'flex',
              position: 'relative',
              overflow: 'hidden',
            }
          : {}),
      }}
    >
      {isEmptyState ? <ParticleBackground opacity={0.6} /> : null}

      {!isEmptyState ? (
        <CardHeader
          title="GenAI Applications"
          subheader={
            isLoading
              ? 'Loading…'
              : error && !isNotFoundError
                ? 'Total: 0'
                : `Total: ${applicationsResponse.count}`
          }
          slotProps={{
            title: { sx: { fontSize: '1rem', fontWeight: 700, marginBottom: 0.3 } },
            subheader: { sx: { fontSize: '0.82rem' } },
          }}
          action={
            !hasApplications ? null : showSeeMore ? (
              <Button component={RouterLink} to={applicationsPath} size="small">
                See more
              </Button>
            ) : (
              <Button component={RouterLink} to={newApplicationPath} size="small">
                + Add New
              </Button>
            )
          }
        />
      ) : null}

      <CardContent
        sx={{
          position: 'relative',
          zIndex: 1,
          ...(isEmptyState
            ? {
                flex: 1,
                minHeight: 300,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }
            : {}),
        }}
      >
        {isLoading ? (
          <Stack divider={<Divider />} spacing={1.5}>
            {[0, 1, 2].map((item) => (
              <Box
                key={item}
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: 1.5,
                  width: '100%',
                }}
              >
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 1.25,
                    minWidth: 0,
                    flex: 1,
                  }}
                >
                  <Skeleton variant="circular" width={36} height={36} />
                  <Box sx={{ minWidth: 0, width: '100%' }}>
                    <Skeleton variant="text" width="50%" height={24} />
                    <Skeleton variant="text" width="75%" height={18} />
                  </Box>
                </Box>
                <Skeleton variant="text" width={80} height={18} />
              </Box>
            ))}
          </Stack>
        ) : error && !isNotFoundError ? (
          <Box sx={{ py: 2 }}>
            <ErrorAlert error={error} onRetry={refreshApplications} />
          </Box>
        ) : applications.length === 0 || isNotFoundError ? (
          <Stack
            spacing={1.5}
            alignItems="center"
            justifyContent="center"
            sx={{ textAlign: 'center', py: 2, width: '100%' }}
          >
            <Box
              component="img"
              src={NoApplications}
              alt="No applications"
              sx={{ width: 140, maxWidth: '80%' }}
            />
            <Typography variant="h6" sx={{ fontWeight: 700 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.create.your.first.genai.application"
                defaultMessage="Create your first GenAI Application"
              />
            </Typography>
            <Typography
              variant="body2"
              color="text.secondary"
              sx={{ maxWidth: 420 }}
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.setup.genai.application.description"
                defaultMessage="Set up a GenAI application to securely consume AI services through your workspace."
              />
            </Typography>
            <Button
              variant="contained"
              component={RouterLink}
              to={newApplicationPath}
              startIcon={<Plus size={20} />}
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.applications.ApplicationsList.create.application"
                defaultMessage="Create Application"
              />
            </Button>
          </Stack>
        ) : (
          <Stack divider={<Divider />} spacing={1.5}>
            {applications.slice(0, 6).map((application) => (
              <Box
                key={application.id}
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: 1.5,
                  width: '100%',
                }}
              >
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 1.25,
                    minWidth: 0,
                    cursor: 'pointer',
                  }}
                  onClick={() => onApplicationClick(application.id)}
                >
                  <Avatar
                    sx={{
                      width: 36,
                      height: 36,
                      bgcolor: 'primary.light',
                      color: 'primary.contrastText',
                    }}
                  >
                    {getInitials(application.name || 'NA')}
                  </Avatar>
                  <Box sx={{ minWidth: 0, overflow: 'hidden' }}>
                    <Typography variant="body1" sx={{ fontWeight: 600 }} noWrap>
                      {truncateWords(application.name || 'No Name', 12)}
                    </Typography>
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      fontSize="0.7rem"
                      noWrap
                    >
                      {truncateWords(
                        application.description || 'No description',
                        12
                      )}
                    </Typography>
                  </Box>
                </Box>

                <Stack
                  direction="row"
                  spacing={0.75}
                  alignItems="center"
                  sx={{ flexShrink: 0, whiteSpace: 'nowrap' }}
                >
                  <Clock size={14} />
                  <Typography variant="caption" color="text.secondary" noWrap>
                    {formatRelativeTime(
                      application.lastUpdated ||
                        application.updatedAt ||
                        application.createdAt
                    )}
                  </Typography>
                </Stack>
              </Box>
            ))}
          </Stack>
        )}
      </CardContent>
    </Card>
  );
}
