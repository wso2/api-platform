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

import { useId, useState, type ReactNode } from 'react';
import {
  Box,
  Button,
  ButtonBase,
  Chip,
  Collapse,
  Form,
  FormControl,
  FormHelperText,
  FormLabel,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  OutlinedInput,
  Select,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Activity,
  ChevronRight,
  Coins,
  Infinity as InfinityIcon,
  Plus,
  Puzzle,
  Radio,
  Trash2,
  Wrench,
  X,
} from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl, type IntlShape } from 'react-intl';

import { hairline, iconTileSx } from '@/theme/receipes';
import {
  BUILT_IN_TYPES,
  fieldKeys,
  MAX_CONDITIONS,
  MAX_CUSTOM_ID_CHARS,
  MAX_WINDOWS,
  newCondition,
  newPair,
  newWindow,
  TIME_UNITS,
  TOKEN_TYPES,
  type ConditionDraft,
  type ConditionType,
  type ConditionsCheck,
  type FieldError,
  type OnExceed,
  type TimeUnit,
  type TokenType,
  type WindowDraft,
} from './planConditions';

const messages = defineMessages({
  addCondition: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.addCondition',
    defaultMessage: 'Add condition',
  },
  addLimit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.addLimit',
    defaultMessage: 'Add limit',
  },
  addValue: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.addValue',
    defaultMessage: 'Add value',
  },
  collapse: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.collapse',
    defaultMessage: 'Hide {name} settings',
  },
  conditionsHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.helper',
    defaultMessage:
      'A plan with no conditions is unlimited. A condition takes effect on an API only where the developer attaches the policy that enforces it.',
  },
  conditionsLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.label',
    defaultMessage: 'Conditions',
  },
  conditionTypes: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.conditionTypes',
    defaultMessage: 'Condition types',
    description: 'Accessible name of the menu of condition types to add.',
  },
  countLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.window.count',
    defaultMessage: 'Count',
  },
  countPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.window.countPlaceholder',
    defaultMessage: 'count',
  },
  customIdHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.custom.idHint',
    defaultMessage:
      'Lowercase letters, digits and hyphens, starting with a letter. The policy finds the condition by this ID.',
  },
  customIdLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.custom.idLabel',
    defaultMessage: 'ID',
  },
  customIdPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.custom.idPlaceholder',
    defaultMessage: 'e.g. business-hours',
  },
  empty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.empty',
    defaultMessage: 'No conditions: this plan is unlimited.',
  },
  expand: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.expand',
    defaultMessage: 'Show {name} settings',
  },
  keyLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.pair.key',
    defaultMessage: 'Key',
  },
  keyPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.pair.keyPlaceholder',
    defaultMessage: 'key',
  },
  noLimitYet: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.summary.noLimit',
    defaultMessage: 'No limit yet',
  },
  noValuesYet: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.summary.noValues',
    defaultMessage: 'No values yet',
  },
  onExceedAllow: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.onExceed.allow',
    defaultMessage: 'Allow when reached',
  },
  onExceedLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.onExceed.label',
    defaultMessage: 'When a limit is reached',
  },
  onExceedReject: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.onExceed.reject',
    defaultMessage: 'Reject when reached',
  },
  overageAllowed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.summary.overageAllowed',
    defaultMessage: 'overage allowed',
  },
  pairEquals: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.pair.equals',
    defaultMessage: '=',
    description: 'Sits between the key and the value of a custom condition pair.',
  },
  pairSummary: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.summary.pair',
    defaultMessage: '{name} = {value}',
  },
  per: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.window.per',
    defaultMessage: 'per',
  },
  removeCondition: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.removeCondition',
    defaultMessage: 'Remove {name} condition',
  },
  removeLimit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.removeLimit',
    defaultMessage: 'Remove limit',
  },
  removeValue: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.removeValue',
    defaultMessage: 'Remove value',
  },
  tokenTypeLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.tokenType.label',
    defaultMessage: 'Tokens counted',
  },
  tokenTypeOption: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.tokenType.option',
    defaultMessage:
      '{tokenType, select, prompt {Prompt tokens} completion {Completion tokens} other {Total tokens}}',
  },
  typeDescription: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.type.description',
    defaultMessage:
      '{type, select, requestCount {REST APIs only. Counts every request.} eventCount {WebSocket and WebSub APIs only. Counts every event.} toolCallCount {MCP proxies only. Counts tools/call.} tokenCount {LLM proxies only. Counts model tokens.} other {An ID and key-value pairs, read by a policy your team writes.}}',
    description: 'What a condition type counts, and on which APIs, in the Add condition menu.',
  },
  typeName: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.type.name',
    defaultMessage:
      '{type, select, requestCount {Request count} eventCount {Event count} toolCallCount {Tool-call count} tokenCount {Token count} other {Custom}}',
  },
  unitOption: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.window.unitOption',
    defaultMessage: '{unit, select, MINUTE {Minute} HOUR {Hour} DAY {Day} other {Month}}',
  },
  unitLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.window.unit',
    defaultMessage: 'Window unit',
  },
  valueLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.pair.value',
    defaultMessage: 'Value',
  },
  valuePlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.pair.valuePlaceholder',
    defaultMessage: 'value',
  },
  windowLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.window.amount',
    defaultMessage: 'Window length',
  },
  windowSummary: {
    id: 'apiControlPlane.pages.appShell.appShellPages.orgManage.components.PlanConditionsEditor.summary.window',
    defaultMessage:
      '{count} {what, select, events {events} toolCalls {tool calls} total {tokens} prompt {prompt tokens} completion {completion tokens} other {requests}} per {amount, plural, one {{unit, select, MINUTE {minute} HOUR {hour} DAY {day} other {month}}} other {# {unit, select, MINUTE {minutes} HOUR {hours} DAY {days} other {months}}}}',
    description: 'One limit of a condition, e.g. "1,000 requests per minute".',
  },
});

const TYPE_ICONS: Record<ConditionType, ReactNode> = {
  custom: <Puzzle size={16} />,
  'event-count': <Radio size={16} />,
  'request-count': <Activity size={16} />,
  'token-count': <Coins size={16} />,
  'tool-call-count': <Wrench size={16} />,
};

/** The ICU select key of each condition type (ICU keys take no hyphens). */
const TYPE_KEYS: Record<ConditionType, string> = {
  custom: 'custom',
  'event-count': 'eventCount',
  'request-count': 'requestCount',
  'token-count': 'tokenCount',
  'tool-call-count': 'toolCallCount',
};

const WHAT: Record<string, string> = {
  'event-count': 'events',
  'request-count': 'requests',
  'tool-call-count': 'toolCalls',
};

/** The one line a condition's header shows, as the API Portal's editor writes it. */
const summaryOf = (intl: IntlShape, draft: ConditionDraft): string => {
  if (draft.type === 'custom') {
    const pairs = draft.pairs
      .filter((pair) => pair.name.trim())
      .map((pair) =>
        intl.formatMessage(messages.pairSummary, {
          name: pair.name.trim(),
          value: pair.value.trim(),
        }),
      );
    return pairs.length > 0 ? pairs.join(' · ') : intl.formatMessage(messages.noValuesYet);
  }
  const what = draft.type === 'token-count' ? draft.tokenType : WHAT[draft.type];
  const parts = draft.windows
    .filter((window) => /^\d+$/.test(window.count.trim()) && Number(window.count) >= 1)
    .map((window) =>
      intl.formatMessage(messages.windowSummary, {
        amount: Number(window.timeAmount) >= 1 ? Number(window.timeAmount) : 1,
        count: intl.formatNumber(Number(window.count)),
        unit: window.timeUnit,
        what,
      }),
    );
  if (parts.length === 0) return intl.formatMessage(messages.noLimitYet);
  if (draft.onExceed === 'allow') parts.push(intl.formatMessage(messages.overageAllowed));
  return parts.join(' · ');
};

export type PlanConditionsEditorProps = {
  value: ConditionDraft[];
  onChange: (next: ConditionDraft[]) => void;
  /** The result of `checkConditions(value)`. */
  check: ConditionsCheck;
  /** Shows every problem, not only those of fields the user has left. */
  showAllErrors: boolean;
  /** A plan in use keeps its conditions: they are shown, not edited. */
  readOnly: boolean;
  disabled?: boolean;
};

/** The conditions of a plan: one card per condition, and a menu to add the built-in types not yet
 * used, or a custom condition. Follows the API Portal's plan editor. */
export function PlanConditionsEditor({
  value,
  onChange,
  check,
  showAllErrors,
  readOnly,
  disabled = false,
}: PlanConditionsEditorProps) {
  const intl = useIntl();
  const menuId = useId();
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  // The type picked from the menu, added once the menu has closed: while it is open it holds the
  // focus, which the new condition's first field takes.
  const [picked, setPicked] = useState<ConditionType | null>(null);
  // Which conditions are expanded; a saved plan's conditions open collapsed, new ones expanded.
  const [open, setOpen] = useState<Set<number>>(() => new Set());
  // Fields the user has left, whose problems now show.
  const [touched, setTouched] = useState<Set<string>>(() => new Set());

  const touch = (key: string) =>
    setTouched((current) => (current.has(key) ? current : new Set(current).add(key)));
  /** The problem of a field, once the user has left it or any other field of its row (`row`). */
  const errorOf = (key: string, row: string[] = [key]): FieldError | undefined =>
    showAllErrors || row.some((other) => touched.has(other)) ? check.errors.get(key) : undefined;

  const used = new Set(value.map((draft) => draft.type));
  const addable: ConditionType[] = [...BUILT_IN_TYPES.filter((type) => !used.has(type)), 'custom'];
  const atMax = value.length >= MAX_CONDITIONS;
  const fieldsDisabled = disabled || readOnly;

  const update = (key: number, patch: Partial<ConditionDraft>) =>
    onChange(value.map((draft) => (draft.key === key ? { ...draft, ...patch } : draft)));

  const add = (type: ConditionType) => {
    const draft = newCondition(type);
    setOpen((current) => new Set(current).add(draft.key));
    onChange([...value, draft]);
  };

  return (
    <Stack spacing={1.5}>
      <Stack alignItems="flex-start" direction="row" justifyContent="space-between" spacing={2}>
        <Box>
          <Stack alignItems="center" direction="row" spacing={1}>
            <Form.Header sx={{ fontWeight: 600, typography: 'body2' }}>
              <FormattedMessage {...messages.conditionsLabel} />
            </Form.Header>
            {value.length > 0 && <Chip label={value.length} size="small" />}
          </Stack>
          <Typography
            color="text.secondary"
            sx={{ display: 'block', opacity: 0.7 }}
            variant="caption"
          >
            <FormattedMessage {...messages.conditionsHelper} />
          </Typography>
        </Box>
        {!readOnly && (
          <>
            <Button
              aria-controls={menuAnchor ? menuId : undefined}
              aria-expanded={menuAnchor ? 'true' : undefined}
              aria-haspopup="menu"
              color="secondary"
              disabled={disabled || atMax}
              onClick={(event) => setMenuAnchor(event.currentTarget)}
              size="small"
              startIcon={<Plus size={16} />}
              sx={{ flexShrink: 0 }}
              variant="outlined"
            >
              <FormattedMessage {...messages.addCondition} />
            </Button>
            <Menu
              anchorEl={menuAnchor}
              anchorOrigin={{ horizontal: 'right', vertical: 'bottom' }}
              // Focus goes to the new condition's first field, not back to this button.
              disableRestoreFocus={picked !== null}
              id={menuId}
              onClose={() => setMenuAnchor(null)}
              open={menuAnchor !== null}
              slotProps={{
                list: { 'aria-label': intl.formatMessage(messages.conditionTypes) },
                transition: {
                  onExited: () => {
                    if (picked) add(picked);
                    setPicked(null);
                  },
                },
              }}
              transformOrigin={{ horizontal: 'right', vertical: 'top' }}
            >
              {addable.map((type) => (
                <MenuItem
                  key={type}
                  onClick={() => {
                    setPicked(type);
                    setMenuAnchor(null);
                  }}
                  sx={{ maxWidth: 380, whiteSpace: 'normal' }}
                >
                  <ListItemIcon>{TYPE_ICONS[type]}</ListItemIcon>
                  <ListItemText
                    primary={intl.formatMessage(messages.typeName, { type: TYPE_KEYS[type] })}
                    secondary={intl.formatMessage(messages.typeDescription, {
                      type: TYPE_KEYS[type],
                    })}
                  />
                </MenuItem>
              ))}
            </Menu>
          </>
        )}
      </Stack>

      {value.length === 0 && (
        <Stack
          alignItems="center"
          direction="row"
          spacing={1.5}
          sx={{
            border: '1px dashed',
            borderColor: 'divider',
            borderRadius: '10px',
            color: 'text.secondary',
            px: 2,
            py: 2,
          }}
        >
          <InfinityIcon size={16} />
          <Typography variant="body2">
            <FormattedMessage {...messages.empty} />
          </Typography>
        </Stack>
      )}

      {value.map((draft) => (
        <ConditionCard
          check={check}
          disabled={fieldsDisabled}
          draft={draft}
          errorOf={errorOf}
          key={draft.key}
          onChange={(patch) => update(draft.key, patch)}
          onRemove={() => onChange(value.filter((other) => other.key !== draft.key))}
          onToggle={() =>
            setOpen((current) => {
              const next = new Set(current);
              if (next.has(draft.key)) next.delete(draft.key);
              else next.add(draft.key);
              return next;
            })
          }
          // A condition with a problem showing stays open, so the problem can be seen.
          open={open.has(draft.key) || (showAllErrors && check.conditionsWithErrors.has(draft.key))}
          readOnly={readOnly}
          touch={touch}
        />
      ))}

      {check.errors.get(fieldKeys.conditions()) && (
        <ErrorText error={check.errors.get(fieldKeys.conditions())} />
      )}
    </Stack>
  );
}

const ErrorText = ({ error }: { error: FieldError | undefined }) =>
  error ? (
    <Typography color="error" variant="caption">
      <FormattedMessage {...error.message} values={error.values} />
    </Typography>
  ) : null;

type ConditionCardProps = {
  draft: ConditionDraft;
  check: ConditionsCheck;
  open: boolean;
  readOnly: boolean;
  disabled: boolean;
  errorOf: (key: string, row?: string[]) => FieldError | undefined;
  touch: (key: string) => void;
  onChange: (patch: Partial<ConditionDraft>) => void;
  onRemove: () => void;
  onToggle: () => void;
};

function ConditionCard({
  draft,
  check,
  open,
  readOnly,
  disabled,
  errorOf,
  touch,
  onChange,
  onRemove,
  onToggle,
}: ConditionCardProps) {
  const intl = useIntl();
  const id = useId();
  const name = intl.formatMessage(messages.typeName, { type: TYPE_KEYS[draft.type] });
  const hasError = check.conditionsWithErrors.has(draft.key);

  return (
    <Box
      aria-labelledby={`${id}-title`}
      role="group"
      sx={(theme) => ({
        border: hairline(theme),
        borderColor: hasError && !open ? 'error.main' : 'divider',
        borderRadius: '10px',
      })}
    >
      <Stack alignItems="center" direction="row" spacing={1} sx={{ pr: 1 }}>
        <ButtonBase
          aria-controls={`${id}-body`}
          aria-expanded={open}
          aria-label={intl.formatMessage(open ? messages.collapse : messages.expand, { name })}
          onClick={onToggle}
          sx={{
            borderRadius: '10px',
            flex: 1,
            gap: 1.25,
            justifyContent: 'flex-start',
            minWidth: 0,
            px: 1.5,
            py: 1.25,
            textAlign: 'left',
          }}
        >
          <Box
            component="span"
            sx={{
              color: 'text.secondary',
              display: 'flex',
              transform: open ? 'rotate(90deg)' : 'none',
              transition: 'transform 150ms',
            }}
          >
            <ChevronRight size={16} />
          </Box>
          <Box component="span" sx={(theme) => iconTileSx(theme, 30)}>
            {TYPE_ICONS[draft.type]}
          </Box>
          <Box component="span" sx={{ display: 'flex', flexDirection: 'column', minWidth: 0 }}>
            <Typography
              component="span"
              id={`${id}-title`}
              sx={{ fontWeight: 600 }}
              variant="body2"
            >
              {name}
            </Typography>
            <Typography
              color={hasError && !open ? 'error' : 'text.secondary'}
              component="span"
              noWrap
              variant="caption"
            >
              {summaryOf(intl, draft)}
            </Typography>
          </Box>
        </ButtonBase>
        {draft.type === 'custom' && draft.id.trim() && (
          <Chip
            label={draft.id.trim()}
            size="small"
            sx={{ fontFamily: 'monospace', maxWidth: 200 }}
            variant="outlined"
          />
        )}
        {!readOnly && (
          <IconButton
            aria-label={intl.formatMessage(messages.removeCondition, { name })}
            color="error"
            disabled={disabled}
            onClick={onRemove}
            size="small"
          >
            <Trash2 size={16} />
          </IconButton>
        )}
      </Stack>
      <Collapse in={open} unmountOnExit>
        <Box id={`${id}-body`} sx={{ px: 2, pb: 2, pt: 0.5 }}>
          {draft.type === 'custom' ? (
            <CustomFields
              disabled={disabled}
              draft={draft}
              errorOf={errorOf}
              onChange={onChange}
              readOnly={readOnly}
              touch={touch}
            />
          ) : (
            <LimitFields
              disabled={disabled}
              draft={draft}
              errorOf={errorOf}
              onChange={onChange}
              readOnly={readOnly}
              touch={touch}
            />
          )}
        </Box>
      </Collapse>
    </Box>
  );
}

type FieldsProps = {
  draft: ConditionDraft;
  readOnly: boolean;
  disabled: boolean;
  errorOf: (key: string, row?: string[]) => FieldError | undefined;
  touch: (key: string) => void;
  onChange: (patch: Partial<ConditionDraft>) => void;
};

/** A built-in condition: what it counts (tokens), what happens once a limit is used up, and its
 * windows. */
function LimitFields({ draft, readOnly, disabled, errorOf, touch, onChange }: FieldsProps) {
  const intl = useIntl();
  const id = useId();
  const setWindow = (key: number, patch: Partial<WindowDraft>) =>
    onChange({
      windows: draft.windows.map((window) =>
        window.key === key ? { ...window, ...patch } : window,
      ),
    });
  const windowsError = errorOf(fieldKeys.windows(draft.key));

  return (
    <Stack spacing={1.5}>
      <Box
        sx={{
          display: 'grid',
          gap: 1.5,
          gridTemplateColumns:
            draft.type === 'token-count' ? 'repeat(2, minmax(0, 1fr))' : 'minmax(0, 1fr)',
        }}
      >
        {draft.type === 'token-count' && (
          <FormControl fullWidth>
            <FormLabel id={`${id}-token-type`}>
              <FormattedMessage {...messages.tokenTypeLabel} />
            </FormLabel>
            <Select
              disabled={disabled}
              labelId={`${id}-token-type`}
              onChange={(event) => onChange({ tokenType: event.target.value as TokenType })}
              size="small"
              value={draft.tokenType}
            >
              {TOKEN_TYPES.map((tokenType) => (
                <MenuItem key={tokenType} value={tokenType}>
                  <FormattedMessage {...messages.tokenTypeOption} values={{ tokenType }} />
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        )}
        <FormControl fullWidth>
          <FormLabel id={`${id}-on-exceed`}>
            <FormattedMessage {...messages.onExceedLabel} />
          </FormLabel>
          <Select
            disabled={disabled}
            labelId={`${id}-on-exceed`}
            onChange={(event) => onChange({ onExceed: event.target.value as OnExceed })}
            size="small"
            value={draft.onExceed}
          >
            <MenuItem value="reject">
              <FormattedMessage {...messages.onExceedReject} />
            </MenuItem>
            <MenuItem value="allow">
              <FormattedMessage {...messages.onExceedAllow} />
            </MenuItem>
          </Select>
        </FormControl>
      </Box>

      <Stack spacing={1}>
        {draft.windows.map((window) => {
          const countKey = fieldKeys.count(draft.key, window.key);
          const amountKey = fieldKeys.amount(draft.key, window.key);
          const unitKey = fieldKeys.unit(draft.key, window.key);
          const row = [countKey, amountKey, unitKey];
          const countError = errorOf(countKey, row);
          const amountError = errorOf(amountKey, row);
          const unitError = errorOf(unitKey, row);
          const rowError = countError ?? amountError ?? unitError;
          return (
            <Box key={window.key}>
              <Box
                sx={{
                  alignItems: 'center',
                  display: 'grid',
                  gap: 1,
                  gridTemplateColumns: 'minmax(0, 1.4fr) auto minmax(0, 0.7fr) minmax(0, 1fr) auto',
                }}
              >
                <OutlinedInput
                  autoFocus={!readOnly && window.count === ''}
                  disabled={disabled}
                  error={Boolean(countError)}
                  inputProps={{
                    'aria-label': intl.formatMessage(messages.countLabel),
                    inputMode: 'numeric',
                    min: 1,
                  }}
                  onBlur={() => touch(countKey)}
                  onChange={(event) => setWindow(window.key, { count: event.target.value })}
                  placeholder={intl.formatMessage(messages.countPlaceholder)}
                  size="small"
                  sx={{ fontFamily: 'monospace' }}
                  type="number"
                  value={window.count}
                />
                <Typography color="text.secondary" variant="body2">
                  <FormattedMessage {...messages.per} />
                </Typography>
                <OutlinedInput
                  disabled={disabled}
                  error={Boolean(amountError)}
                  inputProps={{
                    'aria-label': intl.formatMessage(messages.windowLabel),
                    inputMode: 'numeric',
                    min: 1,
                  }}
                  onBlur={() => touch(amountKey)}
                  onChange={(event) => setWindow(window.key, { timeAmount: event.target.value })}
                  placeholder="1"
                  size="small"
                  sx={{ fontFamily: 'monospace' }}
                  type="number"
                  value={window.timeAmount}
                />
                <Select
                  disabled={disabled}
                  error={Boolean(unitError)}
                  inputProps={{ 'aria-label': intl.formatMessage(messages.unitLabel) }}
                  onChange={(event) => {
                    setWindow(window.key, { timeUnit: event.target.value as TimeUnit });
                    touch(unitKey);
                  }}
                  size="small"
                  value={window.timeUnit}
                >
                  {TIME_UNITS.map((unit) => (
                    <MenuItem key={unit} value={unit}>
                      <FormattedMessage {...messages.unitOption} values={{ unit }} />
                    </MenuItem>
                  ))}
                </Select>
                {readOnly ? (
                  <Box sx={{ width: 34 }} />
                ) : (
                  <IconButton
                    aria-label={intl.formatMessage(messages.removeLimit)}
                    color="error"
                    // A built-in condition keeps at least one limit; remove the condition instead.
                    disabled={disabled || draft.windows.length <= 1}
                    onClick={() =>
                      onChange({
                        windows: draft.windows.filter((other) => other.key !== window.key),
                      })
                    }
                    size="small"
                  >
                    <X size={16} />
                  </IconButton>
                )}
              </Box>
              {rowError && (
                <FormHelperText error>
                  <FormattedMessage {...rowError.message} values={rowError.values} />
                </FormHelperText>
              )}
            </Box>
          );
        })}
      </Stack>
      <ErrorText error={windowsError} />
      {!readOnly && (
        <Box>
          <Button
            disabled={disabled || draft.windows.length >= MAX_WINDOWS}
            onClick={() => onChange({ windows: [...draft.windows, newWindow()] })}
            size="small"
            startIcon={<Plus size={16} />}
            variant="text"
          >
            <FormattedMessage {...messages.addLimit} />
          </Button>
        </Box>
      )}
    </Stack>
  );
}

/** A custom condition: its ID, and the key-value pairs a policy reads. */
function CustomFields({ draft, readOnly, disabled, errorOf, touch, onChange }: FieldsProps) {
  const intl = useIntl();
  const id = useId();
  const idKey = fieldKeys.customId(draft.key);
  const idError = errorOf(idKey);
  // Too many bytes in all: shown once any value has been left.
  const paramsError = errorOf(
    fieldKeys.params(draft.key),
    draft.pairs.map((pair) => `${fieldKeys.pairName(draft.key, pair.key)}.value`),
  );

  return (
    <Stack spacing={1.5}>
      <FormControl error={Boolean(idError)} fullWidth required>
        <FormLabel htmlFor={`${id}-id`}>
          <FormattedMessage {...messages.customIdLabel} />
        </FormLabel>
        <OutlinedInput
          autoFocus={!readOnly && draft.id === ''}
          disabled={disabled}
          id={`${id}-id`}
          inputProps={{ 'aria-describedby': `${id}-id-helper`, maxLength: MAX_CUSTOM_ID_CHARS }}
          onBlur={() => touch(idKey)}
          onChange={(event) => onChange({ id: event.target.value })}
          placeholder={intl.formatMessage(messages.customIdPlaceholder)}
          size="small"
          sx={{ fontFamily: 'monospace' }}
          value={draft.id}
        />
        <FormHelperText id={`${id}-id-helper`}>
          {idError ? (
            <FormattedMessage {...idError.message} values={idError.values} />
          ) : (
            <FormattedMessage {...messages.customIdHint} />
          )}
        </FormHelperText>
      </FormControl>

      <Stack spacing={1}>
        {draft.pairs.map((pair) => {
          const nameKey = fieldKeys.pairName(draft.key, pair.key);
          const valueKey = `${nameKey}.value`;
          const nameError = errorOf(nameKey, [nameKey, valueKey]);
          const setPair = (patch: Partial<typeof pair>) =>
            onChange({
              pairs: draft.pairs.map((other) =>
                other.key === pair.key ? { ...other, ...patch } : other,
              ),
            });
          return (
            <Box key={pair.key}>
              <Box
                sx={{
                  alignItems: 'center',
                  display: 'grid',
                  gap: 1,
                  gridTemplateColumns: 'minmax(0, 1fr) auto minmax(0, 1.4fr) auto',
                }}
              >
                <OutlinedInput
                  disabled={disabled}
                  error={Boolean(nameError)}
                  inputProps={{ 'aria-label': intl.formatMessage(messages.keyLabel) }}
                  onBlur={() => touch(nameKey)}
                  onChange={(event) => setPair({ name: event.target.value })}
                  placeholder={intl.formatMessage(messages.keyPlaceholder)}
                  size="small"
                  sx={{ fontFamily: 'monospace' }}
                  value={pair.name}
                />
                <Typography color="text.secondary" variant="body2">
                  <FormattedMessage {...messages.pairEquals} />
                </Typography>
                <OutlinedInput
                  disabled={disabled}
                  inputProps={{ 'aria-label': intl.formatMessage(messages.valueLabel) }}
                  onBlur={() => touch(valueKey)}
                  onChange={(event) => setPair({ value: event.target.value })}
                  placeholder={intl.formatMessage(messages.valuePlaceholder)}
                  size="small"
                  sx={{ fontFamily: 'monospace' }}
                  value={pair.value}
                />
                {readOnly ? (
                  <Box sx={{ width: 34 }} />
                ) : (
                  <IconButton
                    aria-label={intl.formatMessage(messages.removeValue)}
                    color="error"
                    disabled={disabled}
                    onClick={() =>
                      onChange({ pairs: draft.pairs.filter((other) => other.key !== pair.key) })
                    }
                    size="small"
                  >
                    <X size={16} />
                  </IconButton>
                )}
              </Box>
              {nameError && (
                <FormHelperText error>
                  <FormattedMessage {...nameError.message} values={nameError.values} />
                </FormHelperText>
              )}
            </Box>
          );
        })}
      </Stack>
      <ErrorText error={paramsError} />
      {!readOnly && (
        <Box>
          <Button
            disabled={disabled}
            onClick={() => onChange({ pairs: [...draft.pairs, newPair()] })}
            size="small"
            startIcon={<Plus size={16} />}
            variant="text"
          >
            <FormattedMessage {...messages.addValue} />
          </Button>
        </Box>
      )}
    </Stack>
  );
}
