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

import { useEffect, useRef, useState, type FormEvent } from 'react';
import {
  Alert,
  AlertTitle,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  Form,
  FormControl,
  FormHelperText,
  FormLabel,
  IconButton,
  MenuItem,
  OutlinedInput,
  Select,
  Stack,
  Switch,
  Typography,
} from '@wso2/oxygen-ui';
import { Plus, X } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import { ErrorCode, isErrorCode, type ApiError } from '@/api/core/errors';
import {
  useCreateSubscriptionPlan,
  useUpdateSubscriptionPlan,
  type SubscriptionPlan,
} from '@/api/resources/subscriptionPlans';
import { useNotifications } from '@/components/Notifications';

/** One entry of `SubscriptionPlan['limits']`, generated from the spec. */
type SubscriptionPlanLimit = NonNullable<SubscriptionPlan['limits']>[number];

/** A limit row being edited. Numeric fields stay strings while the user types. */
type LimitDraft = {
  key: number;
  limitCount: string;
  timeUnit: SubscriptionPlanLimit['timeUnit'];
};

type FormState = {
  displayName: string;
  limits: LimitDraft[];
  stopOnQuotaReach: boolean;
  /** `yyyy-mm-dd`, or `''` for no expiry — the shape a native date input holds. */
  expiryDate: string;
};

/** Tracks whether a field was edited and then blurred, so a validation error
 * only shows once the user has actually rejected the field — not merely
 * moved focus off it (autofocus lands on the name field on open). */
type FieldVisit = { blurred: boolean; edited: boolean };

const UNVISITED: FieldVisit = { blurred: false, edited: false };

const settled = (visit: FieldVisit) => visit.blurred && visit.edited;

const NAME_FIELD = 'subscription-plan-name';
const EXPIRY_FIELD = 'subscription-plan-expiry';

const emptyForm = (): FormState => ({
  displayName: '',
  expiryDate: '',
  limits: [],
  stopOnQuotaReach: true,
});

/** URL-friendly handle platform-api requires on create; the user never sees it. */
const slugify = (value: string): string =>
  value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

const isPositiveInteger = (value: string): boolean => /^[1-9]\d*$/.test(value.trim());

const messages = defineMessages({
  addLimit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.addLimit',
    defaultMessage: 'Add limit',
  },
  addPlan: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.action.addPlan',
    defaultMessage: 'Add plan',
  },
  adding: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.action.adding',
    defaultMessage: 'Adding…',
    description: 'Label on the submit button while a new plan is being created.',
  },
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.action.cancel',
    defaultMessage: 'Cancel',
  },
  close: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.action.close',
    defaultMessage: 'Close',
    description: 'Accessible name of the icon button that dismisses the dialog.',
  },
  conflictMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.conflict.message',
    defaultMessage:
      'A plan named "{name}" already exists in this organization. Choose a different name.',
  },
  conflictTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.conflict.title',
    defaultMessage: 'Plan not created',
  },
  createFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.notification.createFailed',
    defaultMessage: 'Failed to create the plan.',
  },
  created: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.notification.created',
    defaultMessage: 'Created plan "{name}".',
  },
  editTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.title.edit',
    defaultMessage: 'Edit subscription plan',
  },
  expiryHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.expiry.helper',
    defaultMessage: 'The plan stops accepting subscriptions after this date.',
  },
  expiryLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.expiry.label',
    defaultMessage: 'Expiry date (optional)',
  },
  limitCountLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.limit.countLabel',
    defaultMessage: 'Request count',
    description: 'Accessible name of the numeric field holding how many requests are allowed.',
  },
  limitError: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.limit.error',
    defaultMessage: 'Enter a whole number of at least 1.',
  },
  limitPer: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.limit.per',
    defaultMessage: '/ per',
  },
  limitTypeBandwidth: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.limit.type.bandwidth',
    defaultMessage: 'Bandwidth',
  },
  limitTypeComingSoon: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.limit.type.comingSoon',
    defaultMessage: 'Coming soon',
    description: 'Chip shown next to a limit type that is not selectable yet.',
  },
  limitTypeLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.limit.typeLabel',
    defaultMessage: 'Limit type',
  },
  limitTypeRequestCount: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.limit.type.requestCount',
    defaultMessage: 'Request count',
  },
  limitTypeTokenCount: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.limit.type.tokenCount',
    defaultMessage: 'Token count',
  },
  limitUnitLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.limit.unitLabel',
    defaultMessage: 'Time window unit',
  },
  limitsHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.limits.helper',
    defaultMessage: 'Leave empty for an unlimited plan.',
  },
  limitsLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.limits.label',
    defaultMessage: 'Limits',
  },
  nameConflict: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.name.error.conflict',
    defaultMessage: 'A plan with this name already exists',
  },
  nameInvalidChars: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.name.error.invalidChars',
    defaultMessage: 'Use letters or numbers in the name',
  },
  nameLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.name.label',
    defaultMessage: 'Name',
  },
  namePlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.name.placeholder',
    defaultMessage: 'Gold',
  },
  nameRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.name.error.required',
    defaultMessage: 'Enter a display name',
  },
  removeLimit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.limit.remove',
    defaultMessage: 'Remove limit',
  },
  saveChanges: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.action.saveChanges',
    defaultMessage: 'Save changes',
  },
  saving: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.action.saving',
    defaultMessage: 'Saving…',
    description: 'Label on the submit button while edits to an existing plan are being saved.',
  },
  stopHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.stop.helper',
    defaultMessage: 'Reject requests that exceed the limit.',
  },
  stopLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.stop.label',
    defaultMessage: 'Block requests when quota is reached',
  },
  titleCreate: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.title.create',
    defaultMessage: 'Add subscription plan',
  },
  unitDay: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.unit.day',
    defaultMessage: 'Day',
  },
  unitHour: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.unit.hour',
    defaultMessage: 'Hour',
  },
  unitMinute: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.unit.minute',
    defaultMessage: 'Minute',
  },
  unitMonth: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.unit.month',
    defaultMessage: 'Month',
  },
  updateFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.notification.updateFailed',
    defaultMessage: 'Failed to update the plan.',
  },
  updated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.notification.updated',
    defaultMessage: 'Updated plan "{name}".',
  },
});

/** One disabled `limitType` option, paired with a "Coming soon" chip so a
 * longer label doesn't visually crowd or touch the chip (a fixed `spacing`
 * gap, not space distributed across the row). */
const ComingSoonMenuItem = ({ label, value }: { label: MessageDescriptor; value: string }) => {
  const intl = useIntl();
  return (
    <MenuItem disabled value={value}>
      <Stack alignItems="center" direction="row" spacing={1}>
        <Typography component="span" variant="body2">
          <FormattedMessage {...label} />
        </Typography>
        <Chip
          label={intl.formatMessage(messages.limitTypeComingSoon)}
          size="small"
          variant="outlined"
        />
      </Stack>
    </MenuItem>
  );
};

export type SubscriptionPlanFormDialogProps = {
  open: boolean;
  /** `null` opens the dialog in create mode; a plan opens it in edit mode. */
  plan: SubscriptionPlan | null;
  onClose: () => void;
};

/** Add/edit dialog for a subscription plan. Status (active/inactive) is not
 * edited here — it lives only as the toggle in the list, so a new plan is
 * always created `ACTIVE` and editing an existing one preserves its status. */
export function SubscriptionPlanFormDialog({
  open,
  plan,
  onClose,
}: SubscriptionPlanFormDialogProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const createMutation = useCreateSubscriptionPlan();
  const updateMutation = useUpdateSubscriptionPlan();

  const isEdit = plan !== null;
  const mutation = isEdit ? updateMutation : createMutation;

  // Stable keys for limit rows across add/remove — a plain counter, since
  // limits carry no id of their own until the plan is saved.
  const nextLimitKey = useRef(0);

  const [form, setForm] = useState<FormState>(emptyForm);
  const [submitted, setSubmitted] = useState(false);
  const [conflict, setConflict] = useState(false);
  const [nameVisit, setNameVisit] = useState<FieldVisit>(UNVISITED);
  const [limitVisit, setLimitVisit] = useState<FieldVisit>(UNVISITED);

  useEffect(() => {
    if (!open) return;
    setForm(
      plan
        ? {
            displayName: plan.displayName,
            expiryDate: plan.expiryTime ? plan.expiryTime.slice(0, 10) : '',
            limits: (plan.limits ?? []).map((limit) => ({
              key: nextLimitKey.current++,
              limitCount: String(limit.limitCount),
              timeUnit: limit.timeUnit,
            })),
            stopOnQuotaReach: plan.limits?.[0]?.stopOnQuotaReach ?? true,
          }
        : emptyForm(),
    );
    setSubmitted(false);
    setConflict(false);
    setNameVisit(UNVISITED);
    setLimitVisit(UNVISITED);
  }, [open, plan]);

  const addLimit = () => {
    setForm((current) => ({
      ...current,
      limits: [
        ...current.limits,
        { key: nextLimitKey.current++, limitCount: '', timeUnit: 'MONTH' },
      ],
    }));
  };

  const removeLimit = (key: number) => {
    setForm((current) => ({
      ...current,
      limits: current.limits.filter((limit) => limit.key !== key),
    }));
  };

  const updateLimit = (key: number, patch: Partial<LimitDraft>) => {
    setForm((current) => ({
      ...current,
      limits: current.limits.map((limit) => (limit.key === key ? { ...limit, ...patch } : limit)),
    }));
  };

  const trimmedName = form.displayName.trim();
  const slug = slugify(trimmedName);
  const limitCountInvalid = form.limits.map((limit) => !isPositiveInteger(limit.limitCount));
  const hasLimitError = limitCountInvalid.some(Boolean);

  // Order matters: an empty name is reported before a bad slug, and a 409
  // (checked only after a real submit) only makes sense once the rest passes.
  const nameErrorMessage: MessageDescriptor | null =
    trimmedName.length === 0
      ? messages.nameRequired
      : !isEdit && slug.length === 0
        ? messages.nameInvalidChars
        : conflict
          ? messages.nameConflict
          : null;
  const hasNameError = nameErrorMessage !== null;

  const showNameError = (submitted || settled(nameVisit)) && hasNameError;
  const showLimitError = (submitted || settled(limitVisit)) && hasLimitError;
  const canSubmit = !hasNameError && !hasLimitError && !mutation.isPending;

  // A 409 surfaces as an inline field error (the name is likely the culprit);
  // anything else is a generic toast. Shared so create and update can't drift
  // on which error code means what.
  const notifyMutationError = (error: ApiError, fallback: MessageDescriptor) => {
    if (isErrorCode(error, ErrorCode.CONFLICT)) {
      setConflict(true);
      return;
    }
    notify(error.message || intl.formatMessage(fallback), 'error');
  };

  const handleSubmit = (event: FormEvent) => {
    // A real submit, so Enter in the name field saves the plan.
    event.preventDefault();
    setSubmitted(true);
    if (hasNameError || hasLimitError || mutation.isPending) return;

    const limits: SubscriptionPlanLimit[] = form.limits.map((limit) => ({
      limitCount: Number(limit.limitCount),
      limitType: 'REQUEST_COUNT',
      stopOnQuotaReach: form.stopOnQuotaReach,
      timeAmount: 1,
      timeUnit: limit.timeUnit,
    }));
    const expiryTime = form.expiryDate ? `${form.expiryDate}T00:00:00Z` : undefined;

    if (isEdit && plan) {
      updateMutation.mutate(
        {
          body: { ...plan, displayName: trimmedName, expiryTime, limits },
          subscriptionPlanId: plan.id ?? '',
        },
        {
          onError: (error) => notifyMutationError(error, messages.updateFailed),
          onSuccess: () => {
            notify(intl.formatMessage(messages.updated, { name: trimmedName }), 'success');
            onClose();
          },
        },
      );
      return;
    }

    createMutation.mutate(
      { displayName: trimmedName, expiryTime, id: slug, limits, status: 'ACTIVE' },
      {
        onError: (error) => notifyMutationError(error, messages.createFailed),
        onSuccess: () => {
          notify(intl.formatMessage(messages.created, { name: trimmedName }), 'success');
          onClose();
        },
      },
    );
  };

  return (
    <Dialog
      fullWidth
      maxWidth="sm"
      onClose={mutation.isPending ? undefined : onClose}
      open={open}
      slotProps={{ paper: { sx: { maxHeight: 'calc(100% - 64px)' } } }}
    >
      <DialogTitle>
        <Stack alignItems="flex-start" direction="row" spacing={1.5}>
          <Box sx={{ flex: 1 }}>
            <FormattedMessage {...(isEdit ? messages.editTitle : messages.titleCreate)} />
          </Box>
          <IconButton
            aria-label={intl.formatMessage(messages.close)}
            disabled={mutation.isPending}
            onClick={onClose}
            size="small"
          >
            <X size={18} />
          </IconButton>
        </Stack>
      </DialogTitle>
      <Box component="form" noValidate onSubmit={handleSubmit}>
        <DialogContent>
          <Stack spacing={3}>
            {conflict && (
              <Alert severity="error">
                <AlertTitle sx={{ fontWeight: 600 }}>
                  <FormattedMessage {...messages.conflictTitle} />
                </AlertTitle>
                <FormattedMessage {...messages.conflictMessage} values={{ name: trimmedName }} />
              </Alert>
            )}

            <FormControl error={showNameError} fullWidth required>
              <FormLabel htmlFor={NAME_FIELD}>
                <FormattedMessage {...messages.nameLabel} />
              </FormLabel>
              <OutlinedInput
                aria-describedby={`${NAME_FIELD}-helper-text`}
                autoFocus
                id={NAME_FIELD}
                onBlur={() => setNameVisit((visit) => ({ ...visit, blurred: true }))}
                onChange={(event) => {
                  setForm((current) => ({ ...current, displayName: event.target.value }));
                  setNameVisit((visit) => ({ ...visit, edited: true }));
                  setConflict(false);
                }}
                placeholder={intl.formatMessage(messages.namePlaceholder)}
                value={form.displayName}
              />
              <FormHelperText id={`${NAME_FIELD}-helper-text`}>
                {showNameError && nameErrorMessage ? (
                  <FormattedMessage {...nameErrorMessage} />
                ) : null}
              </FormHelperText>
            </FormControl>

            <Stack spacing={1.5}>
              <Stack alignItems="center" direction="row" justifyContent="space-between">
                <Form.Header sx={{ fontWeight: 600, typography: 'body2' }}>
                  <FormattedMessage {...messages.limitsLabel} />
                </Form.Header>
                {form.limits.length === 0 && (
                  <Button
                    onClick={addLimit}
                    size="small"
                    startIcon={<Plus size={16} />}
                    variant="outlined"
                  >
                    <FormattedMessage {...messages.addLimit} />
                  </Button>
                )}
              </Stack>

              {form.limits.map((limit, index) => (
                <Box
                  key={limit.key}
                  sx={{
                    alignItems: 'center',
                    display: 'grid',
                    gap: 1,
                    gridTemplateColumns: 'minmax(0,1.5fr) minmax(0,1fr) auto minmax(0,1.2fr) auto',
                  }}
                >
                  <Select
                    inputProps={{ 'aria-label': intl.formatMessage(messages.limitTypeLabel) }}
                    onChange={() => updateLimit(limit.key, {})}
                    size="small"
                    value="REQUEST_COUNT"
                  >
                    <MenuItem value="REQUEST_COUNT">
                      <FormattedMessage {...messages.limitTypeRequestCount} />
                    </MenuItem>
                    <ComingSoonMenuItem label={messages.limitTypeBandwidth} value="BANDWIDTH" />
                    <ComingSoonMenuItem
                      label={messages.limitTypeTokenCount}
                      value="TOTAL_TOKEN_COUNT"
                    />
                  </Select>
                  <OutlinedInput
                    error={showLimitError && limitCountInvalid[index]}
                    inputProps={{
                      'aria-label': intl.formatMessage(messages.limitCountLabel),
                      inputMode: 'numeric',
                      min: 1,
                    }}
                    onBlur={() => setLimitVisit((visit) => ({ ...visit, blurred: true }))}
                    onChange={(event) => {
                      updateLimit(limit.key, { limitCount: event.target.value });
                      setLimitVisit((visit) => ({ ...visit, edited: true }));
                    }}
                    placeholder="10000"
                    size="small"
                    type="number"
                    value={limit.limitCount}
                  />
                  <Typography color="text.secondary" variant="body2">
                    <FormattedMessage {...messages.limitPer} />
                  </Typography>
                  <Select
                    inputProps={{ 'aria-label': intl.formatMessage(messages.limitUnitLabel) }}
                    onChange={(event) =>
                      updateLimit(limit.key, {
                        timeUnit: event.target.value as LimitDraft['timeUnit'],
                      })
                    }
                    size="small"
                    value={limit.timeUnit}
                  >
                    <MenuItem value="MINUTE">
                      <FormattedMessage {...messages.unitMinute} />
                    </MenuItem>
                    <MenuItem value="HOUR">
                      <FormattedMessage {...messages.unitHour} />
                    </MenuItem>
                    <MenuItem value="DAY">
                      <FormattedMessage {...messages.unitDay} />
                    </MenuItem>
                    <MenuItem value="MONTH">
                      <FormattedMessage {...messages.unitMonth} />
                    </MenuItem>
                  </Select>
                  <IconButton
                    aria-label={intl.formatMessage(messages.removeLimit)}
                    color="error"
                    onClick={() => removeLimit(limit.key)}
                    size="small"
                  >
                    <X size={16} />
                  </IconButton>
                </Box>
              ))}

              {showLimitError && (
                <Typography color="error" variant="caption">
                  <FormattedMessage {...messages.limitError} />
                </Typography>
              )}
              <Typography color="text.secondary" variant="caption">
                <FormattedMessage {...messages.limitsHelper} />
              </Typography>
            </Stack>

            <Divider />

            <Stack
              alignItems="flex-start"
              direction="row"
              justifyContent="space-between"
              spacing={2}
            >
              <Box>
                <Typography sx={{ fontWeight: 600 }} variant="body2">
                  <FormattedMessage {...messages.stopLabel} />
                </Typography>
                <Typography color="text.secondary" variant="caption">
                  <FormattedMessage {...messages.stopHelper} />
                </Typography>
              </Box>
              <Switch
                checked={form.stopOnQuotaReach}
                onChange={(event) =>
                  setForm((current) => ({ ...current, stopOnQuotaReach: event.target.checked }))
                }
                slotProps={{ input: { 'aria-label': intl.formatMessage(messages.stopLabel) } }}
              />
            </Stack>

            <FormControl fullWidth>
              <FormLabel htmlFor={EXPIRY_FIELD}>
                <FormattedMessage {...messages.expiryLabel} />
              </FormLabel>
              <OutlinedInput
                aria-describedby={`${EXPIRY_FIELD}-helper-text`}
                id={EXPIRY_FIELD}
                onChange={(event) =>
                  setForm((current) => ({ ...current, expiryDate: event.target.value }))
                }
                type="date"
                value={form.expiryDate}
              />
              <FormHelperText id={`${EXPIRY_FIELD}-helper-text`}>
                <FormattedMessage {...messages.expiryHelper} />
              </FormHelperText>
            </FormControl>
          </Stack>
        </DialogContent>
        <Divider />
        <DialogActions>
          <Button disabled={mutation.isPending} onClick={onClose} variant="outlined">
            <FormattedMessage {...messages.cancel} />
          </Button>
          <Button disabled={!canSubmit} type="submit" variant="contained">
            {mutation.isPending ? (
              <FormattedMessage {...(isEdit ? messages.saving : messages.adding)} />
            ) : (
              <FormattedMessage {...(isEdit ? messages.saveChanges : messages.addPlan)} />
            )}
          </Button>
        </DialogActions>
      </Box>
    </Dialog>
  );
}
