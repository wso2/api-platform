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

import React, {
  useState,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useSyncExternalStore,
} from 'react';
import {
  Alert,
  Box,
  Button,
  Chip,
  Typography,
  Card,
  CardContent,
  Checkbox,
  FormControlLabel,
} from '@wso2/oxygen-ui';
import { Check, ChevronDown, ChevronUp, Circle } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import type { ParameterValues } from './types';
import {
  createPolicyForm,
  type PolicyDefinition as EngineDefinition,
  type Resolution,
  type ViewNode,
} from './engine';
import SchemaTree from './SchemaTree';
import { MAX_DESCRIPTION_LINES, useStyles } from './styles';

const messages = defineMessages({
  showMore: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.PolicyParameterEditor.showMore',
    defaultMessage: 'Show more',
    description: 'Expands a truncated description. Verb phrase.',
  },
  showLess: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.PolicyParameterEditor.showLess',
    defaultMessage: 'Show less',
    description: 'Collapses an expanded description. Verb phrase.',
  },
  showAllErrors: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.PolicyParameterEditor.showAllErrors',
    defaultMessage: 'Show all errors',
    description: 'Checkbox that reveals every validation error in the form at once.',
  },
  fixFields: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.PolicyParameterEditor.fixFields',
    defaultMessage:
      '{count, plural, one {Fix the highlighted field to continue.} other {Fix the # highlighted fields to continue.}}',
    description:
      'Banner above the form after a submit attempt with validation problems. {count} is how many fields have one.',
  },
  statusToFix: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.PolicyParameterEditor.statusToFix',
    defaultMessage: '{count} to fix',
    description: 'Chip showing how many validation problems remain. {count} is a number.',
  },
  needsAttention: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.PolicyParameterEditor.needsAttention',
    defaultMessage: 'Needs attention',
    description: 'Heading of the panel listing the validation problems still to fix.',
  },
  goToField: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.PolicyParameterEditor.goToField',
    defaultMessage: 'Go to field',
    description: 'Scrolls to and focuses the field a validation problem is about. Verb phrase.',
  },
  toggleProblems: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.PolicyParameterEditor.toggleProblems',
    defaultMessage: 'Show or hide the list of problems',
    description:
      'Accessible label for the chip that expands or collapses the validation problems panel.',
  },
  ready: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.PolicyParameterEditor.ready',
    defaultMessage: 'Ready',
    description: 'Footer status shown when the form has no validation problems. Adjective.',
  },
  rootPath: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.PolicyParameterEditor.rootPath',
    defaultMessage: 'Parameters',
    description: 'Names the whole form in a validation message that is not about one field. Noun.',
  },
  close: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.PolicyParameterEditor.close',
    defaultMessage: 'Close',
    description: 'Dismisses a read-only view of the policy. Verb.',
  },
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.PolicyParameterEditor.cancel',
    defaultMessage: 'Cancel',
    description: 'Dismisses the form without saving. Verb.',
  },
  add: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.PolicyParameterEditor.add',
    defaultMessage: 'Add',
    description: 'Confirms a new policy configuration. Verb.',
  },
  update: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.PolicyParameterEditor.update',
    defaultMessage: 'Update',
    description: 'Confirms edits to an existing policy configuration. Verb.',
  },
});

/**
 * Format description text:
 * - Collapse single newlines within paragraphs to spaces
 * - Preserve paragraph breaks (double newlines)
 * - Preserve bullet points (lines starting with - or *)
 */
function formatDescriptionText(text: string): string {
  if (!text) return '';

  const trimmed = text.trim();

  // Split by double newlines (paragraph breaks)
  const paragraphs = trimmed.split(/\n\n+/);

  return paragraphs
    .map((paragraph) => {
      // Check if paragraph contains bullet points
      const lines = paragraph.split('\n');
      const hasBullets = lines.some((line) => /^\s*[-*•]/.test(line));

      if (hasBullets) {
        // Keep bullet point formatting, but collapse non-bullet continuation lines
        const result: string[] = [];
        let currentLine = '';

        for (const line of lines) {
          if (/^\s*[-*•]/.test(line)) {
            // This is a bullet point line
            if (currentLine) {
              result.push(currentLine);
            }
            currentLine = line;
          } else if (line.trim() === '') {
            // Empty line
            if (currentLine) {
              result.push(currentLine);
              currentLine = '';
            }
          } else {
            // Continuation of previous line
            currentLine = currentLine ? `${currentLine} ${line.trim()}` : line;
          }
        }
        if (currentLine) {
          result.push(currentLine);
        }
        return result.join('\n');
      } else {
        // Regular paragraph - collapse all newlines to spaces
        return paragraph.replace(/\n/g, ' ').replace(/\s+/g, ' ').trim();
      }
    })
    .join('\n\n');
}

/**
 * Component to render header description with show more/less
 */
const TruncatedHeaderDescription: React.FC<{
  description: string;
  classes: ReturnType<typeof useStyles>;
}> = ({ description, classes }) => {
  const [isExpanded, setIsExpanded] = useState(false);
  const formatted = formatDescriptionText(description);
  const lineCount = formatted.split('\n').length;
  const needsTruncation = lineCount > MAX_DESCRIPTION_LINES;

  if (!needsTruncation) {
    return <Typography sx={classes.headerDescription}>{formatted}</Typography>;
  }

  return (
    <>
      <Typography
        variant="body1"
        component="div"
        sx={isExpanded ? classes.headerDescription : classes.headerDescriptionTruncated}
      >
        {formatted}
      </Typography>
      <Box component="span" sx={classes.showMoreButton} onClick={() => setIsExpanded(!isExpanded)}>
        <FormattedMessage {...(isExpanded ? messages.showLess : messages.showMore)} />
      </Box>
    </>
  );
};

interface PolicyParameterEditorProps {
  policyDefinition: EngineDefinition;
  policyDisplayName?: string;
  /** Leave the policy name out of the header when the host already shows it. */
  hideTitle?: boolean;
  /** Overrides the default Add / Update label of the submit button. */
  submitLabel?: string;
  /** Overrides the default Cancel / Close label of the dismiss button. */
  cancelLabel?: string;
  /** Overrides the footer text shown when the form is valid (default "Ready"). */
  readyLabel?: string;
  existingValues?: ParameterValues;
  onCancel: () => void;
  onSubmit: (values: ParameterValues) => void;
  disabled?: boolean;
  // readOnly renders the parameters for viewing only: every field is
  // non-editable and the submit action is hidden, but the values remain visible
  // and the close/cancel action stays enabled.
  readOnly?: boolean;
}

/** The view node with this `pathKey`, searched through fields, list items and map values. */
function findNode(node: ViewNode, key: string): ViewNode | undefined {
  if (node.pathKey === key) return node;
  const below = [
    ...(node.children ?? []),
    ...(node.items ?? []),
    ...(node.entries ?? []).map((entry) => entry.node),
  ];
  for (const child of below) {
    const found = findNode(child, key);
    if (found) return found;
  }
  return undefined;
}

/**
 * Mirrors the old "level one" check that keeps Add/Update disabled: every
 * required top-level field has a value (a required list has at least one item)
 * and every top-level "configure at least one" group has a selection.
 */
function isLevelOneValid(resolution: Resolution): boolean {
  const topLevel = resolution.errors.some(
    (e) => e.path.length <= 1 && /^(Required|Select at least)/.test(e.message),
  );
  if (topLevel) return false;
  return !(resolution.view.children ?? []).some(
    (c) =>
      c.visible &&
      c.required &&
      (c.field.kind === 'list' ||
        c.field.kind === 'objectList' ||
        c.field.kind === 'multiselect') &&
      (c.items?.length ?? 0) === 0,
  );
}

/**
 * PolicyParameterEditor - A dynamic form editor for policy parameters
 * based on JSON Schema-like definitions from YAML policy files.
 *
 * The form logic (which fields show, which are required, defaults, validation,
 * and the values to submit) comes from ./engine. This component renders it.
 */
const PolicyParameterEditor: React.FC<PolicyParameterEditorProps> = ({
  policyDefinition,
  policyDisplayName,
  hideTitle = false,
  submitLabel,
  cancelLabel,
  readyLabel,
  existingValues,
  onCancel,
  onSubmit,
  disabled = false,
  readOnly = false,
}) => {
  const classes = useStyles();
  const intl = useIntl();
  const { name, description } = policyDefinition;

  // Use policyDisplayName if provided, then the definition's displayName, then its name
  const displayName = policyDisplayName || policyDefinition.displayName || name;

  // One form per policy definition. existingValues seeds it; defaults fill the rest.
  const form = useMemo(
    () =>
      createPolicyForm(policyDefinition, {
        initialConfig: existingValues,
      }),
    // existingValues changes are applied by the effect below
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [policyDefinition],
  );
  const resolution = useSyncExternalStore(form.subscribe, form.getResolution, form.getResolution);

  // Errors are shown after the first submit attempt, as before
  const [showErrors, setShowErrors] = useState(false);

  const canSubmit = useMemo(() => isLevelOneValid(resolution), [resolution]);

  useEffect(() => {
    if (existingValues) {
      form.reset(existingValues);
    }
  }, [existingValues, form]);

  // Handle form submission
  const handleSubmit = useCallback(() => {
    const current = form.getResolution();
    if (!current.valid) {
      setShowErrors(true);
      return;
    }
    setShowErrors(false);
    // `config` holds typed values (numbers, booleans, objects), leaves out fields
    // hidden by a choice or rule, and omits optional values that were never set.
    onSubmit(current.config as ParameterValues);
  }, [form, onSubmit]);

  const errorCount = resolution.errors.length;
  const [panelOpen, setPanelOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  // Scroll to the field a problem is about and put the cursor in it. The error's
  // `pathKey` names the node it was attached to; its config path can differ from where
  // the input is drawn (a named map value is drawn under its row index).
  const goToField = useCallback(
    (error: Resolution['errors'][number]) => {
      setShowErrors(true);
      const owner = findNode(resolution.view, error.pathKey);
      const id = (owner?.path ?? error.path).join('.');
      // Wait a frame so a collapsed section (Advanced settings) has opened first.
      window.requestAnimationFrame(() => {
        const root = rootRef.current;
        // Matched by attribute value, not through a selector string: a property name
        // from the policy definition may contain quotes or brackets.
        const byTestId = (testId: string) =>
          Array.from(root?.querySelectorAll<HTMLElement>('[data-testid]') ?? []).find(
            (element) => element.dataset.testid === testId,
          );
        const target = byTestId(`field-${id}`) ?? byTestId(`node-${id}`);
        target?.scrollIntoView?.({ behavior: 'smooth', block: 'center' });
        target?.focus?.();
      });
    },
    [resolution.view],
  );

  return (
    <Box
      ref={rootRef}
      sx={[classes.root, { containerName: 'policy-editor', containerType: 'inline-size' }]}
    >
      <Box sx={classes.body}>
        {/* Header */}
        {(!hideTitle || description) && (
          <Box sx={classes.header}>
            {!hideTitle && (
              <Box sx={{ alignItems: 'center', display: 'flex', flexWrap: 'wrap', gap: 1 }}>
                <Typography variant="h5" sx={classes.headerTitle}>
                  {displayName}
                </Typography>
                <Chip
                  label={`v${policyDefinition.version.replace(/^v+/i, '')}`}
                  size="small"
                  variant="outlined"
                  sx={{ color: 'text.secondary', fontFamily: 'monospace', fontSize: '0.7rem' }}
                />
              </Box>
            )}
            {description && (
              <TruncatedHeaderDescription description={description} classes={classes} />
            )}
          </Box>
        )}

        {/* Shown after a submit attempt that found problems */}
        {showErrors && errorCount > 0 && (
          <Alert severity="error" sx={{ mb: 1 }} data-testid="policy-param-errors">
            <FormattedMessage {...messages.fixFields} values={{ count: errorCount }} />
          </Alert>
        )}

        <Card>
          <CardContent>
            <SchemaTree
              view={resolution.view}
              form={form}
              showErrors={showErrors}
              disabled={disabled || readOnly}
            />
          </CardContent>
        </Card>
      </Box>

      {/* Problems panel: collapsed by default, opened from the footer chip */}
      {panelOpen && errorCount > 0 && (
        <Box sx={classes.problemsPanel} data-testid="policy-problems">
          <Box sx={classes.problemsHeader}>
            <Typography sx={classes.problemsTitle}>
              <FormattedMessage {...messages.needsAttention} />
            </Typography>
            <FormControlLabel
              label={intl.formatMessage(messages.showAllErrors)}
              sx={{ ml: 'auto', mr: 0, '& .MuiFormControlLabel-label': { fontSize: '0.75rem' } }}
              control={
                <Checkbox
                  size="small"
                  checked={showErrors}
                  onChange={(_, checked) => setShowErrors(checked)}
                />
              }
            />
          </Box>
          <Box aria-live="polite" aria-atomic="true" component="ul" sx={classes.problemsList}>
            {resolution.errors.map((error, index) => (
              <Box component="li" key={`${error.pathKey}-${index}`} sx={classes.problemRow}>
                <Circle fill="currentColor" size={8} style={{ flexShrink: 0 }} />
                <Typography component="span" sx={classes.problemPath}>
                  {error.path.join('.') || intl.formatMessage(messages.rootPath)}
                </Typography>
                <Typography component="span" sx={classes.problemMessage}>
                  {error.message}
                </Typography>
                <Button
                  color="inherit"
                  onClick={() => goToField(error)}
                  size="small"
                  sx={classes.goToFieldButton}
                  variant="text"
                >
                  <FormattedMessage {...messages.goToField} />
                </Button>
              </Box>
            ))}
          </Box>
        </Box>
      )}

      {/* Footer */}
      <Box sx={classes.footer}>
        <Box data-testid="policy-status">
          {errorCount > 0 ? (
            <Button
              aria-expanded={panelOpen}
              aria-label={intl.formatMessage(messages.toggleProblems)}
              color="error"
              endIcon={panelOpen ? <ChevronDown size={16} /> : <ChevronUp size={16} />}
              onClick={() => setPanelOpen((open) => !open)}
              size="small"
              startIcon={<Circle fill="currentColor" size={8} />}
              variant="outlined"
            >
              {intl.formatMessage(messages.statusToFix, { count: errorCount })}
            </Button>
          ) : (
            <Box sx={classes.readyStatus}>
              <Check size={16} />
              <Typography component="span" variant="body2">
                {readyLabel ?? intl.formatMessage(messages.ready)}
              </Typography>
            </Box>
          )}
        </Box>
        <Box sx={classes.footerActions}>
          <Button
            variant="outlined"
            color="secondary"
            onClick={onCancel}
            data-testid="policy-param-cancel"
            disabled={disabled}
          >
            {cancelLabel ?? intl.formatMessage(readOnly ? messages.close : messages.cancel)}
          </Button>
          {!readOnly && (
            <Button
              variant="contained"
              color="primary"
              onClick={handleSubmit}
              data-testid="policy-param-submit"
              disabled={disabled || !canSubmit}
            >
              {submitLabel ?? intl.formatMessage(existingValues ? messages.update : messages.add)}
            </Button>
          )}
        </Box>
      </Box>
    </Box>
  );
};

export default PolicyParameterEditor;
