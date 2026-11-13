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

import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import {
  AdapterDateFns,
  Alert,
  AlertTitle,
  Box,
  Button,
  DatePickers,
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
  OutlinedInput,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { X } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import { isErrorCode } from '@/api/core/errors';
import { useMeters, useMetersById } from '@/api/resources/meters';
import {
  useCreateSubscriptionPlan,
  useSubscriptionPlan,
  useUpdateSubscriptionPlan,
  type SubscriptionPlan,
} from '@/api/resources/subscriptionPlans';
import { useNotifications } from '@/components/Notifications';
import { popupPaperSx } from '@/theme/receipes';
import { PlanConditionsEditor } from './PlanConditionsEditor';
import { PlanPricingSection } from './PlanPricingSection';
import {
  checkConditions,
  draftsFromConditions,
  MAX_DESCRIPTION_CHARS,
  type ConditionDraft,
} from './planConditions';
import {
  buildPricingBody,
  draftFromPricing,
  freeDraft,
  type PricingDraft,
} from './planPricingDraft';

type FormState = {
  displayName: string;
  description: string;
  /** The picked calendar day (local), or `null` for no expiry. Holds an Invalid
   * Date while the user is partway through typing one. */
  expiryDate: Date | null;
};

/** Tracks whether a field was edited and then blurred, so a validation error
 * only shows once the user has actually rejected the field — not merely
 * moved focus off it (autofocus lands on the name field on open). */
type FieldVisit = { blurred: boolean; edited: boolean };

const UNVISITED: FieldVisit = { blurred: false, edited: false };

const settled = (visit: FieldVisit) => visit.blurred && visit.edited;

const NAME_FIELD = 'subscription-plan-name';
const DESCRIPTION_FIELD = 'subscription-plan-description';
const EXPIRY_FIELD = 'subscription-plan-expiry';

const emptyForm = (): FormState => ({
  description: '',
  displayName: '',
  expiryDate: null,
});

/** Characters as the Platform API counts them (runes), not UTF-16 code units. */
const charCount = (value: string) => Array.from(value).length;

/** The longest plan handle the Platform API keeps. */
const MAX_HANDLE_CHARS = 40;

/** URL-friendly handle the Platform API requires on create; the user never sees it. */
const slugify = (value: string): string =>
  value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, MAX_HANDLE_CHARS)
    .replace(/-+$/, '');

/** The picked calendar day, as that day at UTC midnight. */
const toExpiryTime = (date: Date): string =>
  new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate())).toISOString();

/** The calendar day of a stored expiry timestamp, as a local `Date`. */
const fromExpiryTime = (timestamp: string): Date => {
  const utc = new Date(timestamp);
  return new Date(utc.getUTCFullYear(), utc.getUTCMonth(), utc.getUTCDate());
};

/** The code the Platform API answers a new plan whose handle is taken with. */
const PLAN_EXISTS = 'SUBSCRIPTION_PLAN_EXISTS';

const messages = defineMessages({
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
  descriptionLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.SubscriptionPlanFormDialog.description.label',
    defaultMessage: 'Description',
  },
  descriptionPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.SubscriptionPlanFormDialog.description.placeholder',
    defaultMessage: 'What subscribers get on this plan',
  },
  descriptionTooLong: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.SubscriptionPlanFormDialog.description.tooLong',
    defaultMessage: 'Use at most {max, number} characters ({count, number} now).',
  },
  editTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.title.edit',
    defaultMessage: 'Edit subscription plan',
  },
  expiryInvalid: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.expiry.error.invalid',
    defaultMessage: 'Enter a valid date',
  },
  expiryHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.expiry.helper',
    defaultMessage: 'The plan stops accepting subscriptions after this date.',
  },
  expiryLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.expiry.label',
    defaultMessage: 'Expiry date',
  },
  optionalField: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.SubscriptionPlanFormDialog.optionalField',
    defaultMessage: '{label} <muted>(optional)</muted>',
    description: 'Label of an optional field; {label} is the field name.',
  },
  subtitleCreate: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.SubscriptionPlanFormDialog.subtitle.create',
    defaultMessage: 'Define the conditions, pricing and availability of this plan.',
  },
  subtitleEdit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.SubscriptionPlanFormDialog.subtitle.edit',
    defaultMessage: 'Update the conditions and availability of this plan.',
  },
  inUseNote: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.inUseNote',
    defaultMessage: 'This plan is in use. To change its name or conditions, create a new plan.',
    description:
      'Shown when editing a plan that APIs offer or subscriptions use; the name and conditions are read-only.',
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
    defaultMessage: 'Plan name',
  },
  namePlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.name.placeholder',
    defaultMessage: 'e.g. Gold',
  },
  nameRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.name.error.required',
    defaultMessage: 'Enter a display name',
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
  titleCreate: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.SubscriptionPlanFormDialog.title.create',
    defaultMessage: 'Add subscription plan',
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

/** The "(optional)" after a field's label, in a quieter colour. */
const optionalMuted = (chunks: ReactNode[]) => (
  <Typography color="text.secondary" component="span" variant="inherit">
    {chunks}
  </Typography>
);

export type SubscriptionPlanFormDialogProps = {
  open: boolean;
  /** `null` opens the dialog in create mode; a plan opens it in edit mode. */
  plan: SubscriptionPlan | null;
  onClose: () => void;
};

/** Add/edit dialog for a subscription plan. Status (active/inactive) is not
 * edited here — it lives only as the toggle in the list, so a new plan is
 * always created `ACTIVE` and editing an existing one preserves its status.
 * Pricing is set only when a plan is created, in the same request; an existing
 * plan's pricing is shown read-only. */
export function SubscriptionPlanFormDialog({
  open,
  plan,
  onClose,
}: SubscriptionPlanFormDialogProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const createMutation = useCreateSubscriptionPlan();
  const updateMutation = useUpdateSubscriptionPlan();
  // Whether the organization has a meter at all, for adding a usage price; the prices' meters are
  // read by id, and a price's meter is found by searching (MeterPicker).
  const anyMeterQuery = useMeters({ limit: 1 });
  const hasMeters = (anyMeterQuery.data?.pagination.total ?? 0) > 0;

  const isEdit = plan !== null;
  const mutation = isEdit ? updateMutation : createMutation;
  // A plan in use keeps its name and conditions (the server refuses a change); its description,
  // status and expiry can still change.
  const usageQuery = useSubscriptionPlan(open && isEdit ? plan.id : undefined);
  const inUse = isEdit && usageQuery.data?.usage?.inUse === true;

  const [form, setForm] = useState<FormState>(emptyForm);
  const [conditions, setConditions] = useState<ConditionDraft[]>([]);
  // Bumped on every open, so the conditions editor starts afresh (collapsed, nothing touched).
  const [session, setSession] = useState(0);
  const [pricing, setPricing] = useState<PricingDraft>(freeDraft);
  const { meters, isPending: metersPending } = useMetersById(
    pricing.prices.map((price) => price.meterId),
  );
  const [submitted, setSubmitted] = useState(false);
  const [conflict, setConflict] = useState(false);
  const [nameVisit, setNameVisit] = useState<FieldVisit>(UNVISITED);

  useEffect(() => {
    if (!open) return;
    setForm(
      plan
        ? {
            description: plan.description ?? '',
            displayName: plan.displayName,
            expiryDate: plan.expiryTime ? fromExpiryTime(plan.expiryTime) : null,
          }
        : emptyForm(),
    );
    setConditions(draftsFromConditions(plan?.conditions));
    setSession((current) => current + 1);
    setPricing(draftFromPricing(plan?.pricing));
    setSubmitted(false);
    setConflict(false);
    setNameVisit(UNVISITED);
  }, [open, plan]);

  const trimmedName = form.displayName.trim();
  const trimmedDescription = form.description.trim();
  const slug = slugify(trimmedName);
  // A plan in use keeps the conditions it has: they are neither checked nor sent.
  const conditionsCheck = checkConditions(conditions);
  const hasConditionsError = !inUse && conditionsCheck.body === null;

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
  const descriptionLength = charCount(trimmedDescription);
  const hasDescriptionError = descriptionLength > MAX_DESCRIPTION_CHARS;
  const hasExpiryError = form.expiryDate !== null && Number.isNaN(form.expiryDate.getTime());
  // An existing plan's pricing is only shown, so it is neither checked nor sent.
  const pricingResult = isEdit ? { body: null, problem: null } : buildPricingBody(pricing, meters);
  const hasPricingError = pricingResult.problem !== null;
  const busy = mutation.isPending;
  // New pricing is checked against its meters, so it waits until they are read.
  const canSubmit =
    !hasNameError &&
    !hasDescriptionError &&
    !hasConditionsError &&
    !hasExpiryError &&
    !busy &&
    (isEdit || !metersPending);

  const handleSubmit = (event: FormEvent) => {
    // A real submit, so Enter in the name field saves the plan.
    event.preventDefault();
    setSubmitted(true);
    if (
      hasNameError ||
      hasDescriptionError ||
      hasConditionsError ||
      hasExpiryError ||
      hasPricingError ||
      busy
    ) {
      return;
    }

    const expiryTime = form.expiryDate ? toExpiryTime(form.expiryDate) : undefined;

    if (isEdit && plan) {
      updateMutation.mutate(
        {
          // The editable fields only: a body with pricing is refused, and the
          // plan keeps the pricing it was created with. null, not undefined: an
          // omitted expiryTime means "keep", so clearing the date has to be sent
          // explicitly. A plan in use keeps its conditions, so they are left out (omitted means
          // "keep"); the description is sent only when it changed ("" clears it).
          body: {
            displayName: trimmedName,
            expiryTime: expiryTime ?? null,
            id: plan.id,
            status: plan.status,
            ...(inUse ? {} : { conditions: conditionsCheck.body ?? [] }),
            ...(trimmedDescription !== (plan.description ?? '').trim()
              ? { description: trimmedDescription }
              : {}),
          },
          subscriptionPlanId: plan.id ?? '',
        },
        {
          // The server's own reason, not a taken name: an edit keeps the plan's handle.
          onError: (error) =>
            notify(error.message || intl.formatMessage(messages.updateFailed), 'error'),
          onSuccess: () => {
            notify(intl.formatMessage(messages.updated, { name: trimmedName }), 'success');
            onClose();
          },
        },
      );
      return;
    }

    // A paid plan is created with its pricing in the one request.
    createMutation.mutate(
      {
        conditions: conditionsCheck.body ?? [],
        displayName: trimmedName,
        expiryTime,
        id: slug,
        status: 'ACTIVE',
        ...(trimmedDescription ? { description: trimmedDescription } : {}),
        ...(pricingResult.body ? { pricing: pricingResult.body } : {}),
      },
      {
        // A taken handle surfaces as an inline field error (the name is the
        // culprit); anything else, such as refused pricing, as the server's reason.
        onError: (error) => {
          if (isErrorCode(error, PLAN_EXISTS)) {
            setConflict(true);
            return;
          }
          notify(error.message || intl.formatMessage(messages.createFailed), 'error');
        },
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
      onClose={busy ? undefined : onClose}
      open={open}
      slotProps={{ paper: { sx: { maxHeight: 'calc(100% - 64px)' } } }}
    >
      <DialogTitle>
        <Stack alignItems="flex-start" direction="row" spacing={1.5}>
          <Box sx={{ flex: 1 }}>
            <Typography sx={{ fontWeight: 700 }} variant="h6">
              <FormattedMessage {...(isEdit ? messages.editTitle : messages.titleCreate)} />
            </Typography>
            <Typography color="text.secondary" sx={{ opacity: 0.7 }} variant="body2">
              <FormattedMessage {...(isEdit ? messages.subtitleEdit : messages.subtitleCreate)} />
            </Typography>
          </Box>
          <IconButton
            aria-label={intl.formatMessage(messages.close)}
            disabled={busy}
            onClick={onClose}
            size="small"
          >
            <X size={18} />
          </IconButton>
        </Stack>
      </DialogTitle>
      <Divider />
      {/* The form is the flex column the paper's height limit passes on, so the content scrolls
          between the title and the actions. */}
      <Box
        component="form"
        noValidate
        onSubmit={handleSubmit}
        sx={{ display: 'flex', flexDirection: 'column', minHeight: 0 }}
      >
        <DialogContent>
          <Form.Stack spacing={3}>
            {inUse && (
              <Alert severity="info">
                <FormattedMessage {...messages.inUseNote} />
              </Alert>
            )}

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
                disabled={inUse}
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

            <FormControl error={hasDescriptionError} fullWidth>
              <FormLabel htmlFor={DESCRIPTION_FIELD}>
                <FormattedMessage
                  {...messages.optionalField}
                  values={{ label: intl.formatMessage(messages.descriptionLabel), muted: optionalMuted }}
                />
              </FormLabel>
              <OutlinedInput
                aria-describedby={`${DESCRIPTION_FIELD}-helper-text`}
                disabled={busy}
                id={DESCRIPTION_FIELD}
                minRows={2}
                multiline
                onChange={(event) =>
                  setForm((current) => ({ ...current, description: event.target.value }))
                }
                placeholder={intl.formatMessage(messages.descriptionPlaceholder)}
                value={form.description}
              />
              <FormHelperText id={`${DESCRIPTION_FIELD}-helper-text`}>
                {hasDescriptionError ? (
                  <FormattedMessage
                    {...messages.descriptionTooLong}
                    values={{ count: descriptionLength, max: MAX_DESCRIPTION_CHARS }}
                  />
                ) : null}
              </FormHelperText>
            </FormControl>

            <PlanConditionsEditor
              check={conditionsCheck}
              disabled={busy}
              key={session}
              onChange={setConditions}
              readOnly={inUse}
              showAllErrors={submitted}
              value={conditions}
            />

            <PlanPricingSection
              disabled={busy}
              hasMeters={hasMeters}
              meters={meters}
              onChange={setPricing}
              problem={submitted ? pricingResult.problem : null}
              readOnly={isEdit}
              value={pricing}
            />

            <FormControl error={hasExpiryError} fullWidth>
              <FormLabel htmlFor={EXPIRY_FIELD}>
                <FormattedMessage
                  {...messages.optionalField}
                  values={{ label: intl.formatMessage(messages.expiryLabel), muted: optionalMuted }}
                />
              </FormLabel>
              <DatePickers.LocalizationProvider dateAdapter={AdapterDateFns}>
                <DatePickers.DatePicker
                  disablePast
                  onChange={(date) => setForm((current) => ({ ...current, expiryDate: date }))}
                  slotProps={{
                    desktopPaper: { sx: popupPaperSx },
                    field: { clearable: true },
                    mobilePaper: { sx: popupPaperSx },
                    textField: {
                      error: hasExpiryError,
                      id: EXPIRY_FIELD,
                      inputProps: { 'aria-describedby': `${EXPIRY_FIELD}-helper-text` },
                    },
                  }}
                  value={form.expiryDate}
                />
              </DatePickers.LocalizationProvider>
              <FormHelperText id={`${EXPIRY_FIELD}-helper-text`}>
                <FormattedMessage
                  {...(hasExpiryError ? messages.expiryInvalid : messages.expiryHelper)}
                />
              </FormHelperText>
            </FormControl>
          </Form.Stack>
        </DialogContent>
        <Divider />
        <DialogActions>
          <Button disabled={busy} onClick={onClose} variant="outlined">
            <FormattedMessage {...messages.cancel} />
          </Button>
          <Button disabled={!canSubmit} type="submit" variant="contained">
            {busy ? (
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
