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

import { useEffect, useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Box,
  Button,
  Card,
  CardContent,
  Checkbox,
  Chip,
  Collapse,
  Divider,
  FormControl,
  FormHelperText,
  FormLabel,
  Grid,
  OutlinedInput,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { ArrowLeft, ChevronDown } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import {
  scopesOfRoles,
  useCreateServiceAccount,
  useRoles,
  type Role,
} from '@/api/resources/serviceAccounts';
import { useApiScope } from '@/api/core/scope';
import { SecondaryText } from '@/components/common/SecondaryText';
import { useNotifications } from '@/components/Notifications';
import { ErrorState, ForbiddenState, LoadingState } from '@/components/StateViews';
import { Can } from '@/permissions';
import { routes } from '@/routes/paths';
import { DEFAULT_ROLE } from './components/RolePicker';
import { SectionLabel } from './components/ServiceAccountEditParts';
import { holdCredentials } from './createdCredentials';

const P =
  'apiControlPlane.pages.appShell.appShellPages.orgManage.serviceAccounts.CreateServiceAccountPage';

const messages = defineMessages({
  back: { id: `${P}.back`, defaultMessage: 'Service accounts' },
  title: { id: `${P}.title`, defaultMessage: 'New service account' },
  subtitle: {
    id: `${P}.subtitle`,
    defaultMessage:
      'An identity for a script, pipeline or tool. You get a client ID and secret once it is created.',
  },
  details: { id: `${P}.details`, defaultMessage: 'Details' },
  displayName: { id: `${P}.displayName`, defaultMessage: 'Display name' },
  displayNamePlaceholder: {
    id: `${P}.displayNamePlaceholder`,
    defaultMessage: 'e.g. billing-sync-pipeline',
  },
  description: { id: `${P}.description`, defaultMessage: 'Description' },
  descriptionPlaceholder: {
    id: `${P}.descriptionPlaceholder`,
    defaultMessage: 'What the account is for, who owns it',
  },
  roles: { id: `${P}.roles`, defaultMessage: 'Roles' },
  rolesHint: { id: `${P}.rolesHint`, defaultMessage: 'Select one or more. Scopes are combined.' },
  rolesRequired: { id: `${P}.rolesRequired`, defaultMessage: 'Select at least one role.' },
  rolesLoading: { id: `${P}.rolesLoading`, defaultMessage: 'Loading roles' },
  rolesFailed: {
    id: `${P}.rolesFailed`,
    defaultMessage: 'Could not load the roles. Go back and try again.',
  },
  rolesNone: {
    id: `${P}.rolesNone`,
    defaultMessage: 'No roles are defined. Add one to the role-to-scope mapping on the server.',
  },
  scopeCount: {
    id: `${P}.scopeCount`,
    defaultMessage: '{count, plural, one {# scope} other {# scopes}}',
  },
  showScopes: { id: `${P}.showScopes`, defaultMessage: 'Show scopes of {role}' },
  summary: { id: `${P}.summary`, defaultMessage: 'Summary' },
  summaryName: { id: `${P}.summaryName`, defaultMessage: 'Name' },
  summaryRoles: { id: `${P}.summaryRoles`, defaultMessage: 'Roles' },
  summaryNoRoles: { id: `${P}.summaryNoRoles`, defaultMessage: 'None selected' },
  scopesGranted: {
    id: `${P}.scopesGranted`,
    defaultMessage: '{count, plural, one {scope granted} other {scopes granted}}',
  },
  afterTitle: { id: `${P}.afterTitle`, defaultMessage: 'After you create it' },
  afterBody: {
    id: `${P}.afterBody`,
    defaultMessage:
      'You’ll save the secret, then can get a test token on the same page. The secret is shown once.',
  },
  cancel: { id: `${P}.cancel`, defaultMessage: 'Cancel' },
  create: { id: `${P}.create`, defaultMessage: 'Create service account' },
  creating: { id: `${P}.creating`, defaultMessage: 'Creating…' },
  required: { id: `${P}.required`, defaultMessage: 'Required.' },
  tooLong: {
    id: `${P}.tooLong`,
    defaultMessage: 'Too long: the limit is {max} bytes, and some characters take more than one.',
  },
  created: { id: `${P}.created`, defaultMessage: 'Created "{name}".' },
  createFailed: {
    id: `${P}.createFailed`,
    defaultMessage: 'Could not create the service account.',
  },
  forbiddenTitle: {
    id: `${P}.forbiddenTitle`,
    defaultMessage: 'Service accounts are managed by administrators',
  },
  forbiddenBody: {
    id: `${P}.forbiddenBody`,
    defaultMessage: 'Ask an organization administrator if you need a service account.',
  },
  roleServiceAccount: {
    id: `${P}.roleServiceAccount`,
    defaultMessage: 'Read access for automated integrations.',
  },
  roleDeployer: {
    id: `${P}.roleDeployer`,
    defaultMessage: 'Build and deploy APIs, proxies and pipelines.',
  },
  roleSubscriptions: {
    id: `${P}.roleSubscriptions`,
    defaultMessage: 'Manage applications and subscriptions.',
  },
  roleAdmin: {
    id: `${P}.roleAdmin`,
    defaultMessage: 'Full control, including users and roles. Use sparingly.',
  },
});

/** What each well-known role is for; a role the server adds later shows no blurb. */
const ROLE_DESCRIPTIONS: Record<string, MessageDescriptor> = {
  ap_admin: messages.roleAdmin,
  ap_deployer: messages.roleDeployer,
  ap_service_account: messages.roleServiceAccount,
  ap_subscriptions: messages.roleSubscriptions,
};

/** The server's limits, in UTF-8 bytes of the trimmed value. */
const MAX = { description: 1023, displayName: 255 } as const;
type TextField = keyof typeof MAX;
const utf8 = new TextEncoder();

const textProblem = (value: string, field: TextField): MessageDescriptor | null => {
  const trimmed = value.trim();
  if (trimmed.length === 0) return field === 'displayName' ? messages.required : null;
  return utf8.encode(trimmed).length > MAX[field] ? messages.tooLong : null;
};

const REQUIRED_MARK = '*';
const EMPTY_MARK = '—';

const mono = { fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' } as const;

/** Create a service account, on its own page. Administrators only. */
export function CreateServiceAccountPage() {
  const intl = useIntl();
  const { org } = useApiScope();
  return (
    <Can
      do="createServiceAccount"
      fallback={
        <ForbiddenState
          description={intl.formatMessage(messages.forbiddenBody)}
          title={intl.formatMessage(messages.forbiddenTitle)}
        />
      }
    >
      {/* Keyed so switching organization drops the form and any one-time secret. */}
      <CreateForm key={org} />
    </Can>
  );
}

function CreateForm() {
  const intl = useIntl();
  const navigate = useNavigate();
  const { org } = useApiScope();
  const { notify } = useNotifications();
  const rolesQuery = useRoles();
  const createMutation = useCreateServiceAccount();
  const pending = createMutation.isPending;

  const [displayName, setDisplayName] = useState('');
  const [description, setDescription] = useState('');
  const [selected, setSelected] = useState<string[]>([]);
  const [defaulted, setDefaulted] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [expanded, setExpanded] = useState<string[]>([]);

  // The default role leads the list; the rest keep the server's order.
  const roles: readonly Role[] = [...(rolesQuery.data?.list ?? [])].sort(
    (a, b) => Number(b.name === DEFAULT_ROLE) - Number(a.name === DEFAULT_ROLE),
  );
  const listHref = routes.serviceAccounts(org);
  const goBack = () => navigate(listHref);

  // A new account starts with the default role; the admin may remove it.
  useEffect(() => {
    if (defaulted || !rolesQuery.data) return;
    setDefaulted(true);
    if (rolesQuery.data.list.some((role) => role.name === DEFAULT_ROLE))
      setSelected([DEFAULT_ROLE]);
  }, [defaulted, rolesQuery.data]);

  const problems = {
    description: textProblem(description, 'description'),
    displayName: textProblem(displayName, 'displayName'),
    roles: selected.length === 0,
  };
  const valid = !problems.description && !problems.displayName && !problems.roles;
  const granted = scopesOfRoles(selected, roles as Role[]).length;

  const textError = (field: TextField) => {
    const problem = problems[field];
    // "Required" waits for a submit; a too-long value shows while typing.
    if (!problem || (problem === messages.required && !submitted)) return null;
    return intl.formatMessage(problem, { max: MAX[field] });
  };

  const toggle = (name: string) =>
    setSelected((current) =>
      current.includes(name) ? current.filter((role) => role !== name) : [...current, name],
    );
  const toggleExpanded = (name: string) =>
    setExpanded((current) =>
      current.includes(name) ? current.filter((role) => role !== name) : [...current, name],
    );

  const submit = (event: FormEvent) => {
    event.preventDefault();
    setSubmitted(true);
    if (!valid || pending) return;
    const name = displayName.trim();
    createMutation.mutate(
      {
        displayName: name,
        roles: selected,
        ...(description.trim() ? { description: description.trim() } : {}),
      },
      {
        onError: (error) =>
          notify(error.message || intl.formatMessage(messages.createFailed), 'error'),
        onSuccess: (result) => {
          notify(intl.formatMessage(messages.created, { name }), 'success');
          holdCredentials(result);
          navigate(routes.serviceAccountCreated(org));
        },
      },
    );
  };

  const rolesBody = rolesQuery.isPending ? (
    <LoadingState label={intl.formatMessage(messages.rolesLoading)} />
  ) : rolesQuery.error ? (
    <ErrorState message={intl.formatMessage(messages.rolesFailed)} />
  ) : roles.length === 0 ? (
    <Typography color="error" variant="body2">
      <FormattedMessage {...messages.rolesNone} />
    </Typography>
  ) : (
    <Stack spacing={1.25}>
      {roles.map((role) => {
        const checked = selected.includes(role.name);
        const open = expanded.includes(role.name);
        const blurb = ROLE_DESCRIPTIONS[role.name];
        return (
          <Card
            key={role.name}
            sx={(theme) => ({
              bgcolor: checked
                ? `color-mix(in srgb, ${theme.palette.primary.main} 8%, transparent)`
                : 'transparent',
              border: 1,
              borderColor: checked ? 'primary.main' : 'divider',
              transition: 'background-color 120ms, border-color 120ms',
            })}
          >
            <CardContent sx={{ '&:last-child': { pb: 1.25 }, py: 1.25, px: 1.5 }}>
              <Stack alignItems="center" direction="row" spacing={1}>
                <Checkbox
                  checked={checked}
                  disabled={pending}
                  inputProps={{ 'aria-label': role.name }}
                  onChange={() => toggle(role.name)}
                  size="small"
                />
                <Box
                  sx={{ cursor: 'pointer', flex: 1, minWidth: 0 }}
                  onClick={() => {
                    if (!pending) toggle(role.name);
                  }}
                >
                  <Typography sx={{ ...mono, fontWeight: 600 }} variant="body2">
                    {role.name}
                  </Typography>
                  {blurb && (
                    <Typography color="text.secondary" variant="caption">
                      <FormattedMessage {...blurb} />
                    </Typography>
                  )}
                </Box>
                <Button
                  aria-expanded={open}
                  aria-label={intl.formatMessage(messages.showScopes, { role: role.name })}
                  color="inherit"
                  endIcon={
                    <ChevronDown
                      size={14}
                      style={{
                        transform: open ? 'rotate(180deg)' : undefined,
                        transition: 'transform 120ms',
                      }}
                    />
                  }
                  onClick={() => toggleExpanded(role.name)}
                  size="small"
                  sx={{ color: 'text.secondary', flexShrink: 0, fontWeight: 400 }}
                >
                  <FormattedMessage
                    {...messages.scopeCount}
                    values={{ count: role.scopes.length }}
                  />
                </Button>
              </Stack>
              <Collapse in={open} unmountOnExit>
                <Stack direction="row" flexWrap="wrap" gap={0.5} sx={{ pt: 1.5 }}>
                  {role.scopes.map((scope) => (
                    <Chip
                      key={scope}
                      label={scope}
                      size="small"
                      sx={{ ...mono, borderRadius: '4px' }}
                      variant="outlined"
                    />
                  ))}
                </Stack>
              </Collapse>
            </CardContent>
          </Card>
        );
      })}
    </Stack>
  );

  return (
    <Box sx={{ maxWidth: 1080, mx: 'auto', width: '100%' }}>
      <Button
        onClick={goBack}
        size="small"
        startIcon={<ArrowLeft size={16} />}
        sx={{ alignSelf: 'flex-start', color: 'text.secondary', mb: 1.5, ml: -1 }}
      >
        <FormattedMessage {...messages.back} />
      </Button>
      <Typography component="h1" sx={{ fontWeight: 600 }} variant="h4">
        <FormattedMessage {...messages.title} />
      </Typography>
      <SecondaryText sx={{ mb: 3, mt: 0.25 }}>
        <FormattedMessage {...messages.subtitle} />
      </SecondaryText>

      <Box component="form" noValidate onSubmit={submit}>
        <Grid container spacing={3}>
          <Grid size={{ xs: 12, md: 8 }}>
            <Card sx={{ p: 3 }}>
              <Stack spacing={2.5}>
                <SectionLabel>
                  <FormattedMessage {...messages.details} />
                </SectionLabel>
                <FormControl error={Boolean(textError('displayName'))} fullWidth required>
                  <FormLabel htmlFor="service-account-display-name">
                    <FormattedMessage {...messages.displayName} />
                  </FormLabel>
                  <OutlinedInput
                    autoFocus
                    id="service-account-display-name"
                    onChange={(event) => setDisplayName(event.target.value)}
                    placeholder={intl.formatMessage(messages.displayNamePlaceholder)}
                    value={displayName}
                  />
                  {textError('displayName') && (
                    <FormHelperText>{textError('displayName')}</FormHelperText>
                  )}
                </FormControl>

                <FormControl error={Boolean(textError('description'))} fullWidth>
                  <FormLabel htmlFor="service-account-description">
                    <FormattedMessage {...messages.description} />
                  </FormLabel>
                  <OutlinedInput
                    id="service-account-description"
                    minRows={3}
                    multiline
                    onChange={(event) => setDescription(event.target.value)}
                    placeholder={intl.formatMessage(messages.descriptionPlaceholder)}
                    value={description}
                  />
                  {textError('description') && (
                    <FormHelperText>{textError('description')}</FormHelperText>
                  )}
                </FormControl>

                <Divider />

                <Stack
                  alignItems="baseline"
                  direction="row"
                  justifyContent="space-between"
                  spacing={2}
                >
                  <SectionLabel>
                    <FormattedMessage {...messages.roles} /> {REQUIRED_MARK}
                  </SectionLabel>
                  <SecondaryText variant="caption">
                    <FormattedMessage {...messages.rolesHint} />
                  </SecondaryText>
                </Stack>
                <div role="group" aria-label={intl.formatMessage(messages.roles)}>
                  {rolesBody}
                </div>
                {submitted && problems.roles && !rolesQuery.isPending && roles.length > 0 && (
                  <Typography color="error" role="alert" variant="caption">
                    <FormattedMessage {...messages.rolesRequired} />
                  </Typography>
                )}
              </Stack>
            </Card>

            <Stack direction="row" justifyContent="flex-end" spacing={1.5} sx={{ mt: 3 }}>
              <Button disabled={pending} onClick={goBack} variant="outlined">
                <FormattedMessage {...messages.cancel} />
              </Button>
              <Button
                disabled={pending || !rolesQuery.data || !valid}
                type="submit"
                variant="contained"
              >
                <FormattedMessage {...(pending ? messages.creating : messages.create)} />
              </Button>
            </Stack>
          </Grid>

          <Grid size={{ xs: 12, md: 4 }}>
            <Stack spacing={2.5}>
              <Card sx={{ p: 2.5 }}>
                <Stack spacing={2}>
                  <SectionLabel>
                    <FormattedMessage {...messages.summary} />
                  </SectionLabel>
                  <Box>
                    <Typography color="text.secondary" variant="caption">
                      <FormattedMessage {...messages.summaryName} />
                    </Typography>
                    <Typography sx={{ overflowWrap: 'anywhere' }} variant="body1">
                      {displayName.trim() || EMPTY_MARK}
                    </Typography>
                  </Box>
                  <Box>
                    <Typography
                      color="text.secondary"
                      sx={{ display: 'block', mb: 0.5 }}
                      variant="caption"
                    >
                      <FormattedMessage {...messages.summaryRoles} />
                    </Typography>
                    {selected.length === 0 ? (
                      <Typography color="text.secondary" variant="body2">
                        <FormattedMessage {...messages.summaryNoRoles} />
                      </Typography>
                    ) : (
                      <Stack direction="row" flexWrap="wrap" gap={0.5}>
                        {selected.map((name) => (
                          <Chip
                            color="primary"
                            key={name}
                            label={name}
                            size="small"
                            sx={mono}
                            variant="outlined"
                          />
                        ))}
                      </Stack>
                    )}
                  </Box>
                  <Divider />
                  <Stack alignItems="baseline" direction="row" spacing={1}>
                    <Typography sx={{ fontWeight: 500 }} variant="h4">
                      {granted}
                    </Typography>
                    <Typography color="text.secondary" variant="body2">
                      <FormattedMessage {...messages.scopesGranted} values={{ count: granted }} />
                    </Typography>
                  </Stack>
                </Stack>
              </Card>
              <Box sx={{ px: 0.5 }}>
                <Typography sx={{ fontWeight: 600 }} variant="body2">
                  <FormattedMessage {...messages.afterTitle} />
                </Typography>
                <SecondaryText sx={{ mt: 0.25 }} variant="body2">
                  <FormattedMessage {...messages.afterBody} />
                </SecondaryText>
              </Box>
            </Stack>
          </Grid>
        </Grid>
      </Box>
    </Box>
  );
}
