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

import {
  Alert,
  Avatar,
  Box,
  Chip,
  DataGrid,
  ListingTable,
  PageTitle,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Shield } from '@wso2/oxygen-ui-icons-react';
import { useMemo, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useParams } from 'react-router-dom';

import {
  useGraphQLApi,
  useGraphQLApiSdl,
  useUpdateGraphQLApi,
  type GraphQLApiDetail,
} from '@/api/resources/graphqlApis';
import { useMetersById } from '@/api/resources/meters';
import { useRestApi, useUpdateRestApi, type Policy, type RestApi } from '@/api/resources/restApis';
import { useAllSubscriptionPlans, type SubscriptionPlan } from '@/api/resources/subscriptionPlans';
import { AppPage } from '@/components/AppPage';
import { SecondaryText } from '@/components/common/SecondaryText';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useNotifications } from '@/components/Notifications';
import { EmptyState, ErrorState, LoadingState } from '@/components/StateViews';
import { useCan } from '@/permissions';
import { routes } from '@/routes/paths';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { ScopeGate } from '@/scope/ScopeGate';
import {
  toEditableOperations,
  withMajorPolicyVersion,
  withPolicyEdits,
} from '../../apis/utils/developEdit';
import { PlanConditionChips, tipSlotProps } from '../../orgManage/components/PlanConditionChips';
import { pricingSummary, summaryMeterIds } from '../../orgManage/components/pricingFormat';
import { SelectionSummary } from '../../portals/components/SelectionSummary';
import { type ApiKind, parsePolicyKey, policyAttachment, policyLabel } from './planPolicies';
import { PolicyName } from '../policies/PolicyChip';
import { SaveBar } from '../SaveBar';
import { useDirtyTracking } from '../useDirtyTracking';
import {
  attachedPlanIdsOf,
  defaultPolicy,
  missingPolicies,
  policiesToAttach,
  policiesToRemove,
  withAttachedPlans,
  withPoliciesAdded,
  withPoliciesRemoved,
} from './usagePlans';

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.title',
    defaultMessage: 'Usage Plans',
    description: 'Heading of the page where the subscription plans an API offers are attached. Noun.',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.subtitle',
    defaultMessage:
      'The subscription plans this API offers. Attaching a plan offers to attach the policies that enforce it. When you publish, you choose which of these plans each API Portal lists.',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.loading',
    defaultMessage: 'Loading API',
  },
  notFound: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.notFound',
    defaultMessage: 'API not found',
  },
  loadError: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.loadError',
    defaultMessage: 'Unable to load the API.',
  },
  scopePrompt: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.scopePrompt',
    defaultMessage: 'Usage plans are attached to a single API.',
    description: 'Explains why an API must be picked before this page can render.',
  },
  plansLoading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.plansLoading',
    defaultMessage: 'Loading subscription plans',
  },
  plansError: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.plansError',
    defaultMessage: 'Unable to load subscription plans.',
  },
  emptyTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.emptyTitle',
    defaultMessage: 'No subscription plans',
  },
  emptyDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.emptyDescription',
    defaultMessage: 'Create subscription plans in Manage › Subscription Plans to attach them here.',
  },
  selectedSummary: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.selectedSummary',
    defaultMessage: '{count} of {total} plans attached',
    description: '{count} is a styled number rendered as a React element, not plain text; {total} is a plain number.',
  },
  attachedSummary: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.attachedSummary',
    defaultMessage: '{count, plural, one {<n>#</n> plan attached} other {<n>#</n> plans attached}}',
    description: '<n> wraps the number, which is styled.',
  },
  deployHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.deployHint',
    defaultMessage:
      'Saving the changes stores the plans and policies on the API. The gateway applies them the next time you deploy it.',
  },
  missingPolicies: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.missingPolicies',
    defaultMessage: 'You have not yet attached the following required policies:',
    description: 'Followed by a bullet list of the policies the attached plans need that the API lacks.',
  },
  columnName: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.columnName',
    defaultMessage: 'Plan',
  },
  columnDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.columnDescription',
    defaultMessage: 'Description',
  },
  columnConditions: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.columnConditions',
    defaultMessage: 'Conditions',
  },
  columnPricing: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.columnPricing',
    defaultMessage: 'Pricing',
  },
  inactive: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.inactive',
    defaultMessage: 'Inactive',
    description: 'Chip after the name of an attached plan that is no longer active.',
  },
  selectAll: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.selectAll',
    defaultMessage: 'Attach all plans',
  },
  clearAll: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.clearAll',
    defaultMessage: 'Detach all plans',
  },
  selectRow: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.selectRow',
    defaultMessage: 'Attach plan',
  },
  unselectRow: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.unselectRow',
    defaultMessage: 'Detach plan',
  },
  selectedPlans: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.selectedPlans',
    defaultMessage: 'The selected plans',
    description: 'Stands for a plan name in the attach dialog when several plans were attached at once.',
  },
  attachTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.attachTitle',
    defaultMessage: 'Attach required policies?',
  },
  attachMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.attachMessage',
    defaultMessage:
      '{count, plural, one {{plan} is} other {{plan} are}} enforced by these policies, which this API does not have yet.',
    description: '{plan} is a plan name (or "The selected plans"); the list of policies follows.',
  },
  attachQuestion: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.attachQuestion',
    defaultMessage:
      'They are added to the API’s global policies. Save and deploy the API for them to take effect.',
  },
  attachConfirm: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.attachConfirm',
    defaultMessage: 'Attach policies',
  },
  attachCancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.attachCancel',
    defaultMessage: 'Skip',
  },
  removeTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.removeTitle',
    defaultMessage: 'Remove unused policies?',
  },
  removeMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.removeMessage',
    defaultMessage: 'No plan attached to this API uses these policies any more.',
    description: 'The list of policies follows.',
  },
  removeQuestion: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.removeQuestion',
    defaultMessage: 'They are removed from the API’s global policies when you save.',
  },
  removeConfirm: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.removeConfirm',
    defaultMessage: 'Remove policies',
  },
  removeCancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.removeCancel',
    defaultMessage: 'Keep',
  },
  policiesAttached: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.policiesAttached',
    defaultMessage: 'Policies attached. Save and deploy the API to apply them.',
  },
  policiesRemoved: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.policiesRemoved',
    defaultMessage: 'Policies removed. Save and deploy the API to apply the change.',
  },
  saved: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.usagePlans.UsagePlansPage.saved',
    defaultMessage: 'Usage plans saved. Deploy the API to apply them on the gateway.',
  },
});


/** A plan this page can render — `id` is optional on the generated type but always present. */
type DisplayablePlan = SubscriptionPlan & { id: string };

/** The dialog open after plans were ticked or unticked: the policies to attach or remove. */
type PolicyPrompt = { action: 'attach' | 'remove'; count: number; names: string[]; plan: string } | null;

/** One row of the plan list. */
type PlanRow = {
  conditions: SubscriptionPlan['conditions'];
  description?: string;
  id: string;
  inactive: boolean;
  name: string;
  pricing: SubscriptionPlan['pricing'];
};

/** One line of secondary text; the whole text on hover, only when it is cut short. */
function TruncatedText({ text }: { text?: string }) {
  const [cut, setCut] = useState(false);
  return (
    <Tooltip disableHoverListener={!cut} slotProps={tipSlotProps} title={text ?? ''}>
      <Typography
        color="text.secondary"
        noWrap
        onMouseEnter={(event) =>
          setCut(event.currentTarget.scrollWidth > event.currentTarget.clientWidth)
        }
        variant="body2"
      >
        {text}
      </Typography>
    </Tooltip>
  );
}

/** Every cell's content, centred in the fixed-height row. */
const cellSx = { alignItems: 'center', display: 'flex', height: '100%', minWidth: 0 } as const;

/** Condition chips a row shows before the rest fold into "+N". */
const MAX_ROW_CONDITIONS = 2;

/** Above this many plans the grid pages; below it the footer is hidden. */
const MAX_PAGE_ROWS = 100;

/** A policy key's name and version, as `labelOf` reads it ("subscription-validation v2"). */
const policyParts = (label: string) => {
  const at = label.lastIndexOf(' ');
  return at > 0 ? { name: label.slice(0, at), version: label.slice(at + 1) } : { name: label, version: '' };
};

/** Policies as the Policies page lists them: icon, display name from the Policy Hub, version. */
function PolicyRows({ labels }: { labels: string[] }) {
  return (
    <Stack
      component="ul"
      sx={{ border: 1, borderColor: 'divider', borderRadius: 1, listStyle: 'none', m: 0, overflow: 'hidden', p: 0 }}
      divider={<Box aria-hidden sx={{ borderTop: 1, borderColor: 'divider' }} />}
    >
      {labels.map(policyParts).map(({ name, version }) => (
        <Stack
          alignItems="center"
          aria-label={[name, version].filter(Boolean).join(' ')}
          component="li"
          direction="row"
          key={name}
          spacing={1.25}
          sx={{ px: 1.5, py: 1 }}
        >
          <Avatar sx={{ borderRadius: 0.75, height: 28, width: 28 }} variant="rounded">
            <Shield size={14} />
          </Avatar>
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography noWrap sx={{ fontWeight: 600 }} variant="body2">
              <PolicyName name={name} />
            </Typography>
            <Typography color="text.secondary" noWrap sx={{ fontFamily: 'monospace' }} variant="caption">
              {name}
            </Typography>
          </Box>
          {version && <Chip label={version} size="small" variant="outlined" />}
        </Stack>
      ))}
    </Stack>
  );
}

/** The missing policies under the warning: display name and version, one per line. */
function PolicyBullets({ labels }: { labels: string[] }) {
  return (
    <Box component="ul" sx={{ m: 0, mt: 0.5, pl: 2.5 }}>
      {labels.map(policyParts).map(({ name, version }) => (
        <Typography
          aria-label={[name, version].filter(Boolean).join(' ')}
          component="li"
          key={name}
          variant="body2"
        >
          <PolicyName name={name} />
          {version && (
            <Box component="span" sx={{ color: 'text.secondary', ml: 0.75 }}>
              {version}
            </Box>
          )}
        </Typography>
      ))}
    </Box>
  );
}

/** How many plans and API-level policies differ from what was saved, at least one when dirty. */
const countChanges = (
  saved: { planIds: string[]; policies: Policy[] },
  current: { planIds: string[]; policies: Policy[] },
) => {
  const differ = (a: string[], b: string[]) =>
    a.filter((x) => !b.includes(x)).length + b.filter((x) => !a.includes(x)).length;
  const names = (policies: Policy[]) => policies.map((policy) => policy.name);
  return (
    differ(saved.planIds, current.planIds) + differ(names(saved.policies), names(current.policies))
  );
};

/** What the panel needs from an API, whatever its kind. */
type PlanAttachmentProps = {
  /** Plan ids the API offers now (its `subscriptionPlans`). */
  attachedPlanIds: string[];
  /** The API's own (API-level) policies. */
  apiPolicies: Policy[];
  /** Per-operation policies (a REST API's resources); they count towards what applies to every
   * request but are not edited here. */
  operations?: Parameters<typeof policyAttachment>[0]['operations'];
  /** The kind of API: plan conditions it never counts ask for no policy. */
  kind: ApiKind;
  readOnly: boolean;
  saving: boolean;
  /** Save blocked for now (e.g. the API id, or a GraphQL schema still loading). */
  saveDisabled?: boolean;
  /** Stores the plans and API-level policies; calls `onSaved` once they are stored. */
  onSave: (planIds: string[], policies: Policy[], onSaved: () => void) => void;
};

/**
 * Develop › Usage Plans: which of the organization's subscription plans this API offers.
 * Ticking a plan offers to attach, at API level, the policies that enforce it that the API lacks
 * (subscription-validation first); unticking one offers to remove those no remaining plan needs.
 * A note lists the policies the attached plans need that the API lacks, as edited. One save stores
 * the plans and the policies on the API; nothing is deployed.
 */
function PlanAttachmentPanel({
  apiPolicies,
  attachedPlanIds,
  kind,
  onSave,
  operations = [],
  readOnly,
  saveDisabled = false,
  saving,
}: PlanAttachmentProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const plansQuery = useAllSubscriptionPlans();

  const [planIds, setPlanIds] = useState<string[]>(attachedPlanIds);
  const [policies, setPolicies] = useState<Policy[]>(apiPolicies);
  const [prompt, setPrompt] = useState<PolicyPrompt>(null);

  const { dirty, markSaved, saved } = useDirtyTracking({ planIds, policies });
  const changeCount = dirty ? Math.max(1, countChanges(saved, { planIds, policies })) : 0;

  const allPlans = (plansQuery.data?.list ?? []).filter((plan): plan is DisplayablePlan =>
    Boolean(plan.id),
  );
  const plansById = new Map(allPlans.map((plan) => [plan.id, plan]));
  // The active plans, and any attached plan made inactive since, so it can be detached.
  const shownPlans = allPlans.filter(
    (plan) => plan.status === 'ACTIVE' || planIds.includes(plan.id),
  );
  // The unit a per-unit price counts (call or token) comes from its meter, as on the plans page.
  const { meters } = useMetersById(shownPlans.flatMap((plan) => summaryMeterIds(plan.pricing)));
  const attachment = policyAttachment({ operations, policies });

  // Each as `name version`: a custom policy's key carries its version; a built-in one's comes from
  // the API (when removing) or its default (when attaching).
  const labelOf = (key: string) => {
    if (parsePolicyKey(key).version) return policyLabel(key);
    const version =
      policies.find((policy) => policy.name === key)?.version ?? defaultPolicy(key).version;
    return `${key} ${version}`;
  };
  const missing = missingPolicies(
    planIds.flatMap((id) => plansById.get(id) ?? []),
    kind,
    attachment,
  );

  /**
   * Applies the grid's new selection. A single tick (or a select-all) attaches plans and offers the
   * policies they need; an untick (or clearing all) detaches them and offers to remove the policies
   * no remaining plan needs. The grid reports a select-all as "every row but these".
   */
  const handleSelection = (model: DataGrid.GridRowSelectionModel) => {
    if (readOnly) return;
    const ticked = (id: string) => (model.type === 'exclude' ? !model.ids.has(id) : model.ids.has(id));
    const shownIds = new Set(shownPlans.map((plan) => plan.id));
    // An attached id the list does not show (a deleted plan) is not the grid's to change.
    const next = [
      ...planIds.filter((id) => !shownIds.has(id)),
      ...shownPlans.filter((plan) => ticked(plan.id)).map((plan) => plan.id),
    ];
    const added = shownPlans.filter((plan) => ticked(plan.id) && !planIds.includes(plan.id));
    const removed = shownPlans.filter((plan) => !ticked(plan.id) && planIds.includes(plan.id));
    setPlanIds(next);
    const nameOf = (plans: DisplayablePlan[]) =>
      plans.length === 1 ? plans[0].displayName : intl.formatMessage(messages.selectedPlans);
    if (added.length > 0) {
      const names = policiesToAttach(added, kind, attachment);
      if (names.length > 0) {
        setPrompt({ action: 'attach', count: added.length, names, plan: nameOf(added) });
      }
    } else if (removed.length > 0) {
      const remaining = next.flatMap((id) => plansById.get(id) ?? []);
      const names = policiesToRemove(
        { conditions: removed.flatMap((plan) => plan.conditions ?? []) },
        remaining,
        kind,
        policies,
      );
      if (names.length > 0) {
        setPrompt({ action: 'remove', count: removed.length, names, plan: nameOf(removed) });
      }
    }
  };

  const selection = useMemo<DataGrid.GridRowSelectionModel>(
    () => ({ ids: new Set(planIds), type: 'include' }),
    [planIds],
  );

  const confirmPrompt = () => {
    if (!prompt) return;
    if (prompt.action === 'attach') {
      setPolicies(withPoliciesAdded(policies, prompt.names));
      notify(intl.formatMessage(messages.policiesAttached), 'success');
    } else {
      setPolicies(withPoliciesRemoved(policies, prompt.names));
      notify(intl.formatMessage(messages.policiesRemoved), 'success');
    }
    setPrompt(null);
  };

  const cancel = () => {
    setPlanIds(attachedPlanIds);
    setPolicies(apiPolicies);
    setPrompt(null);
  };

  const save = () =>
    onSave(planIds, policies, () => {
      markSaved();
      notify(intl.formatMessage(messages.saved), 'success');
    });

  const rows: PlanRow[] = shownPlans.map((plan) => ({
    conditions: plan.conditions,
    description: plan.description,
    id: plan.id,
    inactive: plan.status !== 'ACTIVE',
    name: plan.displayName,
    pricing: plan.pricing,
  }));
  const columns: DataGrid.GridColDef<PlanRow>[] = [
    {
      field: 'name',
      flex: 0.4,
      headerName: intl.formatMessage(messages.columnName),
      minWidth: 180,
      renderCell: ({ row }) => (
        <Box sx={{ ...cellSx, gap: 1 }}>
          <Typography noWrap variant="body2">{row.name}</Typography>
          {row.inactive && (
            <Typography color="text.secondary" variant="caption">
              <FormattedMessage {...messages.inactive} />
            </Typography>
          )}
        </Box>
      ),
      sortable: false,
    },
    {
      field: 'description',
      flex: 0.9,
      headerName: intl.formatMessage(messages.columnDescription),
      minWidth: 200,
      // One line in every row; the full text on hover.
      renderCell: ({ row }) => (
        <Box sx={cellSx}>
          <TruncatedText text={row.description} />
        </Box>
      ),
      sortable: false,
    },
    {
      field: 'conditions',
      flex: 1.2,
      headerName: intl.formatMessage(messages.columnConditions),
      minWidth: 300,
      renderCell: ({ row }) => (
        <Box sx={cellSx}>
          <PlanConditionChips conditions={row.conditions} max={MAX_ROW_CONDITIONS} singleLine />
        </Box>
      ),
      sortable: false,
    },
    {
      field: 'pricing',
      flex: 0.4,
      headerName: intl.formatMessage(messages.columnPricing),
      minWidth: 110,
      renderCell: ({ row }) => (
        <Box sx={cellSx}>
          <Typography color={row.pricing ? 'text.primary' : 'text.secondary'} noWrap variant="body2">
            {pricingSummary(intl, row.pricing, meters)}
          </Typography>
        </Box>
      ),
      sortable: false,
    },
  ];

  let plansBody;
  if (plansQuery.isPending) {
    plansBody = <LoadingState label={intl.formatMessage(messages.plansLoading)} />;
  } else if (plansQuery.error) {
    plansBody = <ErrorState message={intl.formatMessage(messages.plansError)} />;
  } else if (shownPlans.length === 0) {
    plansBody = (
      <EmptyState
        description={intl.formatMessage(messages.emptyDescription)}
        title={intl.formatMessage(messages.emptyTitle)}
      />
    );
  } else {
    plansBody = (
      <Stack spacing={2}>
        <SelectionSummary
          count={planIds.filter((id) => plansById.has(id)).length}
          published={messages.attachedSummary}
          readOnly={readOnly}
          selected={messages.selectedSummary}
          total={shownPlans.length}
        />
        <ListingTable.Container>
          <ListingTable.DataGrid
            checkboxSelection
            columns={columns}
            density="standard"
            disableColumnMenu
            disableColumnResize
            disableRowSelectionOnClick
            hideFooter={rows.length <= MAX_PAGE_ROWS}
            hideFooterSelectedRowCount
            isRowSelectable={() => !readOnly && !saving}
            localeText={{
              checkboxSelectionSelectAllRows: intl.formatMessage(messages.selectAll),
              checkboxSelectionSelectRow: intl.formatMessage(messages.selectRow),
              checkboxSelectionUnselectAllRows: intl.formatMessage(messages.clearAll),
              checkboxSelectionUnselectRow: intl.formatMessage(messages.unselectRow),
            }}
            onRowSelectionModelChange={handleSelection}
            rows={rows}
            rowSelectionModel={selection}
          />
        </ListingTable.Container>
      </Stack>
    );
  }

  return (
    <Stack spacing={2}>
      <PageTitle>
        <PageTitle.Header>
          <FormattedMessage {...messages.title} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <SecondaryText component="span" variant="inherit">
            <FormattedMessage {...messages.subtitle} />
          </SecondaryText>
        </PageTitle.SubHeader>
      </PageTitle>

      {plansBody}

      {plansQuery.data && missing.length > 0 && (
        <Alert severity="warning">
          <FormattedMessage {...messages.missingPolicies} />
          <PolicyBullets labels={missing.map(labelOf)} />
        </Alert>
      )}
      <SecondaryText variant="body2">
        <FormattedMessage {...messages.deployHint} />
      </SecondaryText>

      {!readOnly && (
        <SaveBar
          changeCount={changeCount}
          dirty={dirty}
          disabled={saveDisabled}
          onCancel={cancel}
          onSave={save}
          saving={saving}
        />
      )}

      <ConfirmDialog
        cancelLabel={intl.formatMessage(
          prompt?.action === 'remove' ? messages.removeCancel : messages.attachCancel,
        )}
        confirmLabel={intl.formatMessage(
          prompt?.action === 'remove' ? messages.removeConfirm : messages.attachConfirm,
        )}
        maxWidth="sm"
        message={
          prompt
            ? intl.formatMessage(
                prompt.action === 'remove' ? messages.removeMessage : messages.attachMessage,
                { count: prompt.count, plan: prompt.plan },
              )
            : ''
        }
        onCancel={() => setPrompt(null)}
        onConfirm={confirmPrompt}
        open={prompt !== null}
        title={intl.formatMessage(
          prompt?.action === 'remove' ? messages.removeTitle : messages.attachTitle,
        )}
      >
        {prompt && (
          <>
            <PolicyRows labels={prompt.names.map(labelOf)} />
            <Typography color="text.secondary" variant="caption">
              <FormattedMessage
                {...(prompt.action === 'remove' ? messages.removeQuestion : messages.attachQuestion)}
              />
            </Typography>
          </>
        )}
      </ConfirmDialog>
    </Stack>
  );
}

/** Develop › Usage Plans for a REST API: its plans and API-level policies in one update. */
export function UsagePlansPanel({ api }: { api: RestApi }) {
  const update = useUpdateRestApi();
  // As on the Policies page: a gateway-owned API, or a caller who may not update it, sees the
  // page but cannot change it.
  const canUpdate = useCan('UpdateRESTAPI');
  // Resource policies are not edited here; they count towards what applies to every request.
  const operations = useMemo(() => toEditableOperations(api), [api]);
  return (
    <PlanAttachmentPanel
      apiPolicies={api.policies ?? []}
      attachedPlanIds={attachedPlanIdsOf(api)}
      kind="rest"
      onSave={(planIds, policies, onSaved) => {
        if (!api.id) return;
        update.mutate(
          {
            restApiId: api.id,
            body: withAttachedPlans(withPolicyEdits(api, { operations, policies }), planIds),
          },
          // No `onError`: the query client's `onMutationError` already notifies.
          { onSuccess: onSaved },
        );
      }}
      operations={operations}
      readOnly={api.readOnly === true || !canUpdate}
      saveDisabled={!api.id}
      saving={update.isPending}
    />
  );
}

/**
 * The `metadata` of a GraphQL API update that stores its plans and API-level policies: the whole
 * API, with the schema resupplied (introspection as is, anything else as the resolved SDL inline)
 * so the update does not re-derive it.
 */
export const graphqlUsagePlansMetadata = (
  api: GraphQLApiDetail,
  planIds: string[],
  policies: Policy[],
  sdl: string | undefined,
) => ({
  ...withAttachedPlans(api, planIds),
  policies: policies.map(withMajorPolicyVersion),
  ...(api.schemaSource !== 'introspection' && api.schemaSource !== undefined
    ? { schemaSource: 'inline' as const, sdl: sdl ?? '' }
    : { schemaSource: 'introspection' as const }),
});

/**
 * Develop › Usage Plans for a GraphQL API. Saved as the GraphQL Policies page saves: the whole API
 * as metadata, with the schema resupplied (introspection as is, anything else as the resolved SDL
 * inline) so the update does not re-derive it.
 */
export function GraphqlUsagePlansPanel({ api }: { api: GraphQLApiDetail }) {
  const update = useUpdateGraphQLApi();
  const sdlQuery = useGraphQLApiSdl(api.id);
  const needsSdlForSave = api.schemaSource !== 'introspection' && api.schemaSource !== undefined;
  const canUpdate = useCan('UpdateGraphQLAPI');
  return (
    <PlanAttachmentPanel
      apiPolicies={api.policies ?? []}
      attachedPlanIds={attachedPlanIdsOf(api)}
      kind="graphql"
      onSave={(planIds, policies, onSaved) => {
        if (!api.id) return;
        update.mutate(
          {
            graphqlApiId: api.id,
            body: { metadata: graphqlUsagePlansMetadata(api, planIds, policies, sdlQuery.data) },
          },
          { onSuccess: onSaved },
        );
      }}
      readOnly={Boolean(api.readOnly) || !canUpdate}
      saveDisabled={!api.id || (needsSdlForSave && sdlQuery.isPending)}
      saving={update.isPending}
    />
  );
}

/** The GraphQL route's page: it lives outside the REST API scope, so it reads its own param. */
export function GraphqlUsagePlansPage() {
  const intl = useIntl();
  const { graphqlApiHandler } = useParams();
  const apiQuery = useGraphQLApi(graphqlApiHandler);
  const content = apiQuery.isPending ? (
    <LoadingState label={intl.formatMessage(messages.loading)} />
  ) : apiQuery.error || !apiQuery.data ? (
    <ErrorState title={intl.formatMessage(messages.notFound)} />
  ) : (
    <GraphqlUsagePlansPanel api={apiQuery.data} key={apiQuery.data.id} />
  );
  return <AppPage>{content}</AppPage>;
}

export function UsagePlansPage() {
  const intl = useIntl();
  const { params } = useConsoleScope();
  const apiQuery = useRestApi(params.apiHandler);

  const content = apiQuery.isPending ? (
    <LoadingState label={intl.formatMessage(messages.loading)} />
  ) : apiQuery.error ? (
    <ErrorState title={intl.formatMessage(messages.loadError)} />
  ) : !apiQuery.data ? (
    <ErrorState title={intl.formatMessage(messages.notFound)} />
  ) : (
    // Keyed by the API, so moving to another API starts from its own plans and policies.
    <UsagePlansPanel api={apiQuery.data} key={apiQuery.data.id} />
  );

  return (
    <AppPage>
      <ScopeGate
        prompt={intl.formatMessage(messages.scopePrompt)}
        requires="api"
        to={routes.apiDevelopUsagePlans}
      >
        {content}
      </ScopeGate>
    </AppPage>
  );
}
