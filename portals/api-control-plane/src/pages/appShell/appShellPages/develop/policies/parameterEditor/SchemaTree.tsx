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

import React, { useState, useMemo, useRef, useEffect } from 'react';
import {
  Box,
  Accordion,
  AccordionSummary,
  AccordionDetails,
  Typography,
  Collapse,
  IconButton,
  Button,
  Chip,
  Checkbox,
  FormControlLabel,
  Radio,
  RadioGroup,
} from '@wso2/oxygen-ui';
import { ChevronRight, ChevronDown, Plus, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import type { GroupView, PolicyForm, ViewNode } from './engine';
import { fieldError, getFieldRenderer, testPath } from './FieldRenderers';
import { MAX_DESCRIPTION_LINES, useStyles } from './styles';

const messages = defineMessages({
  showMore: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.SchemaTree.showMore',
    defaultMessage: 'Show more',
    description: 'Expands a truncated description. Verb phrase.',
  },
  showLess: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.SchemaTree.showLess',
    defaultMessage: 'Show less',
    description: 'Collapses an expanded description. Verb phrase.',
  },
  chooseAtLeastOne: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.SchemaTree.chooseAtLeastOne',
    defaultMessage: 'Configure at least one',
    description: 'Legend of a group of options where any combination may be selected.',
  },
  chooseOne: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.SchemaTree.chooseOne',
    defaultMessage: 'Choose one',
    description: 'Legend of a group of options where exactly one is selected.',
  },
  advancedSettings: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.SchemaTree.advancedSettings',
    defaultMessage: 'Advanced Settings',
    description:
      'Accessible label for the section holding the parameters the policy marks as advanced.',
  },
  advancedSettingsCount: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.SchemaTree.advancedSettingsCount',
    defaultMessage: 'Advanced settings ({count})',
    description: 'Heading of the advanced parameters section. {count} is how many it holds.',
  },
  collapseField: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.SchemaTree.collapseField',
    defaultMessage: 'Collapse {label}',
    description:
      'Accessible label for the button hiding a field\u2019s contents. {label} is the field name from the policy definition; do not translate it.',
  },
  expandField: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.SchemaTree.expandField',
    defaultMessage: 'Expand {label}',
    description:
      'Accessible label for the button revealing a field\u2019s contents. {label} is the field name from the policy definition; do not translate it.',
  },
  itemLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.SchemaTree.itemLabel',
    defaultMessage: 'Item {number}',
    description: 'Label of one entry in a list. {number} is its 1-based position.',
  },
  deleteItem: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.SchemaTree.deleteItem',
    defaultMessage: 'Delete Item {number}',
    description:
      'Accessible label for the button removing one list entry. {number} is its 1-based position.',
  },
  clearField: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.SchemaTree.clearField',
    defaultMessage: 'Clear {label}',
    description:
      'Accessible label for the button resetting an optional section. {label} is the section name from the policy definition; do not translate it.',
  },
  requiredMarker: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.SchemaTree.requiredMarker',
    defaultMessage: '*',
    description: 'Marker beside the label of a required parameter.',
  },
  deprecated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.SchemaTree.deprecated',
    defaultMessage: 'Deprecated',
    description: 'Badge on a parameter the policy marks as deprecated.',
  },
  gatewayDefaultHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.SchemaTree.gatewayDefaultHint',
    defaultMessage: 'Leave empty to use the gateway\u2019s configured value.',
  },
  addItem: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.parameterEditor.SchemaTree.addItem',
    defaultMessage: 'Add Item',
    description: 'Adds another entry to a list of objects. Verb phrase.',
  },
});

/**
 * Component to render description with show more/less
 */
const TruncatedDescription: React.FC<{
  description: string;
  classes: ReturnType<typeof useStyles>;
}> = ({ description, classes }) => {
  const [isExpanded, setIsExpanded] = useState(false);
  const trimmed = description.trim();
  const lineCount = trimmed.split('\n').length;
  const needsTruncation = lineCount > MAX_DESCRIPTION_LINES;

  if (!needsTruncation) {
    return <Typography sx={classes.nodeDescription}>{trimmed}</Typography>;
  }

  return (
    <>
      <Typography sx={isExpanded ? classes.nodeDescription : classes.nodeDescriptionTruncated}>
        {trimmed}
      </Typography>
      <Box component="span" sx={classes.showMoreButton} onClick={() => setIsExpanded(!isExpanded)}>
        <FormattedMessage {...(isExpanded ? messages.showLess : messages.showMore)} />
      </Box>
    </>
  );
};

/** Advanced fields go under "Advanced Settings", unless they are currently required. */
const isAdvancedNode = (node: ViewNode): boolean => node.field.advanced && !node.required;

/** Whether a node or anything under it has an error. */
const hasErrorsDeep = (node: ViewNode): boolean =>
  node.errors.length > 0 ||
  (node.groups ?? []).some((g) => g.errors.length > 0) ||
  (node.children ?? []).some((c) => c.visible && hasErrorsDeep(c)) ||
  (node.items ?? []).some(hasErrorsDeep) ||
  (node.entries ?? []).some((e) => e.keyErrors.length > 0 || hasErrorsDeep(e.node));

interface TreeContext {
  form: PolicyForm;
  showErrors: boolean;
  disabled?: boolean;
}

/**
 * Selector for a oneOf / anyOf group: radio buttons when exactly one option
 * applies, checkboxes when any combination does. Only the chosen fields show.
 */
const GroupSelector: React.FC<{ group: GroupView } & TreeContext> = ({
  group,
  form,
  showErrors,
  disabled,
}) => {
  const classes = useStyles();
  const g = group.group;
  const testId = `group-${group.instanceKey.replace(/#/g, '-') || 'root'}`;
  const toggle = (id: string, on: boolean) =>
    form.setChoice(
      group.instanceKey,
      on ? [...group.selected, id] : group.selected.filter((x) => x !== id),
    );
  return (
    <Box component="fieldset" sx={classes.groupSelector} data-testid={testId}>
      <Typography component="legend" sx={classes.groupSelectorLabel}>
        <FormattedMessage {...(g.multi ? messages.chooseAtLeastOne : messages.chooseOne)} />
      </Typography>
      {g.multi ? (
        <Box sx={classes.multiSelectRow}>
          {g.options.map((o) => (
            <FormControlLabel
              key={o.id}
              label={<Typography sx={classes.choiceLabel}>{o.label}</Typography>}
              control={
                <Checkbox
                  size="small"
                  checked={group.selected.includes(o.id)}
                  onChange={(e) => toggle(o.id, e.target.checked)}
                  disabled={disabled}
                  inputProps={{ 'aria-label': o.label }}
                />
              }
            />
          ))}
        </Box>
      ) : (
        <RadioGroup
          row
          value={group.selected[0] ?? ''}
          onChange={(e) => form.setChoice(group.instanceKey, [e.target.value])}
        >
          {g.options.map((o) => (
            <FormControlLabel
              key={o.id}
              value={o.id}
              label={<Typography sx={classes.choiceLabel}>{o.label}</Typography>}
              control={<Radio size="small" disabled={disabled} />}
            />
          ))}
        </RadioGroup>
      )}
      {showErrors &&
        group.errors.map((e) => (
          <Box key={e} sx={classes.errorText}>
            {e}
          </Box>
        ))}
    </Box>
  );
};

interface ChildrenWithAdvancedProps extends TreeContext {
  parent: ViewNode;
  depth: number;
}

/**
 * Splits children into normal and advanced groups based on x-wso2-policy-advanced-param.
 * A choice-group selector is placed just before the group's first field.
 */
const ChildrenWithAdvanced: React.FC<ChildrenWithAdvancedProps> = ({ parent, depth, ...ctx }) => {
  const classes = useStyles();
  const intl = useIntl();
  const [showAdvanced, setShowAdvanced] = useState<boolean | null>(null);

  const { normalItems, advancedChildren } = useMemo(() => {
    const normal: Array<{ kind: 'group'; group: GroupView } | { kind: 'node'; node: ViewNode }> =
      [];
    const advanced: ViewNode[] = [];
    const placed = new Set<GroupView>();
    (parent.children ?? []).forEach((child) => {
      (parent.groups ?? []).forEach((g) => {
        if (!placed.has(g) && g.group.members.includes(child.field.key)) {
          placed.add(g);
          normal.push({ kind: 'group', group: g });
        }
      });
      if (!child.visible) return;
      if (isAdvancedNode(child)) advanced.push(child);
      else normal.push({ kind: 'node', node: child });
    });
    return { normalItems: normal, advancedChildren: advanced };
  }, [parent]);

  const hasAdvanced = advancedChildren.length > 0;
  const advancedOpen = showAdvanced ?? (ctx.showErrors && advancedChildren.some(hasErrorsDeep));

  return (
    <Box sx={classes.nodeChildren}>
      {normalItems.map((item) =>
        item.kind === 'group' ? (
          <GroupSelector key={`group-${item.group.instanceKey}`} group={item.group} {...ctx} />
        ) : (
          <SchemaTreeNodeComponent
            key={item.node.pathKey}
            node={item.node}
            depth={depth}
            {...ctx}
          />
        ),
      )}
      {hasAdvanced && (
        <Accordion
          expanded={advancedOpen}
          onChange={(_, expanded) => setShowAdvanced(expanded)}
          variant="outlined"
          sx={{
            borderRadius: 1,
            '&:first-of-type, &:last-of-type': { borderRadius: 1 },
            '&::before': { display: 'none' },
            overflow: 'hidden',
          }}
          disableGutters
        >
          <AccordionSummary
            expandIcon={<ChevronDown size={16} />}
            aria-label={intl.formatMessage(messages.advancedSettings)}
          >
            <Typography variant="subtitle2" sx={{ fontSize: '0.75rem' }}>
              <FormattedMessage
                {...messages.advancedSettingsCount}
                values={{ count: advancedChildren.length }}
              />
            </Typography>
          </AccordionSummary>
          <AccordionDetails>
            {advancedChildren.map((child) => (
              <SchemaTreeNodeComponent key={child.pathKey} node={child} depth={depth} {...ctx} />
            ))}
          </AccordionDetails>
        </Accordion>
      )}
    </Box>
  );
};

interface SchemaTreeNodeProps extends TreeContext {
  node: ViewNode;
  depth: number;
  defaultExpanded?: boolean;
}

/**
 * Renders a single tree node with its children
 */
const SchemaTreeNodeComponent: React.FC<SchemaTreeNodeProps> = ({
  node,
  depth,
  defaultExpanded = false,
  ...ctx
}) => {
  const { form, showErrors, disabled } = ctx;
  const classes = useStyles();
  const intl = useIntl();
  const [isExpanded, setIsExpanded] = useState(defaultExpanded);
  const { field } = node;

  const last = node.path[node.path.length - 1];
  const isArrayItem = typeof last === 'number';
  const parentPath = node.path.slice(0, -1);
  const isObject = field.kind === 'object';
  const isComplexArray = field.kind === 'objectList';
  const items = node.items ?? [];
  // An optional section that isn't configured yet (no value) shows only its header.
  const isOffSection = isObject && field.toggleable === true && node.enabled === false;
  const visibleChildren = (node.children ?? []).filter((c) => c.visible);

  // Determine if this node has expandable content
  const hasExpandableContent =
    (isObject && (isOffSection || visibleChildren.length > 0 || (node.groups ?? []).length > 0)) ||
    (isComplexArray && items.length > 0);
  const expanded = !isOffSection && (isExpanded || (showErrors && hasErrorsDeep(node)));

  const toggleExpand = () => {
    if (isOffSection) {
      // Expanding an unconfigured section starts configuring it, with its defaults.
      form.setEnabled(node.path, true);
      setIsExpanded(true);
      return;
    }
    setIsExpanded(!expanded);
  };

  // New array items start expanded
  const prevArrayLengthRef = useRef(items.length);
  const newItemIndex = items.length > prevArrayLengthRef.current ? items.length - 1 : -1;
  useEffect(() => {
    prevArrayLengthRef.current = items.length;
  });

  const FieldRenderer = getFieldRenderer(node);
  const ownError = showErrors && !FieldRenderer ? node.errors[0] : undefined;
  const maxItems = field.constraints.maxItems;

  return (
    <Box
      sx={[classes.treeNode, isObject ? classes.nestedSection : null]}
      data-testid={`node-${testPath(node.path)}`}
    >
      {/* Node Row */}
      <Box sx={[classes.nodeRow, { pl: 0 }]}>
        {/* Node Content */}
        <Box sx={classes.nodeContent}>
          {/* Label Row */}
          <Box sx={classes.nodeLabelRow}>
            {hasExpandableContent && (
              <IconButton
                size="small"
                sx={classes.expandIcon}
                onClick={toggleExpand}
                aria-label={intl.formatMessage(
                  expanded ? messages.collapseField : messages.expandField,
                  {
                    label: field.label,
                  },
                )}
                aria-expanded={expanded}
                data-testid={`expand-${testPath(node.path)}`}
              >
                {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
              </IconButton>
            )}
            <Typography sx={classes.nodeLabel}>
              {isArrayItem
                ? intl.formatMessage(messages.itemLabel, { number: (last as number) + 1 })
                : field.label}
            </Typography>
            {node.required && !isArrayItem && (
              <Typography sx={classes.requiredBadge}>
                {intl.formatMessage(messages.requiredMarker)}
              </Typography>
            )}
            {field.deprecated && (
              <Chip
                label={intl.formatMessage(messages.deprecated)}
                size="small"
                variant="outlined"
                color="warning"
                sx={classes.deprecatedBadge}
              />
            )}
            {/* Delete button for array items */}
            {isArrayItem && (
              <IconButton
                size="small"
                sx={classes.arrayItemDeleteButton}
                onClick={() => form.removeItem(parentPath, last as number)}
                disabled={disabled}
                aria-label={intl.formatMessage(messages.deleteItem, {
                  number: (last as number) + 1,
                })}
              >
                <Trash2 size={14} />
              </IconButton>
            )}
            {/* Clear button for a configured optional section */}
            {isObject && field.toggleable && node.enabled && (
              <IconButton
                size="small"
                sx={classes.sectionClearButton}
                onClick={() => {
                  form.setEnabled(node.path, false);
                  setIsExpanded(false);
                }}
                disabled={disabled}
                aria-label={intl.formatMessage(messages.clearField, { label: field.label })}
                data-testid={`clear-${testPath(node.path)}`}
              >
                <Trash2 size={14} />
              </IconButton>
            )}
          </Box>

          {/* Field Renderer (for leaf nodes) */}
          {FieldRenderer && (
            <FieldRenderer
              node={node}
              form={form}
              error={showErrors ? fieldError(node) : undefined}
              disabled={disabled}
            />
          )}
          {/* Description */}
          {field.description && !isArrayItem && !isObject && (
            <TruncatedDescription description={field.description} classes={classes} />
          )}
          {field.gatewayDefault && (
            <Typography sx={classes.gatewayDefaultHint}>
              <FormattedMessage {...messages.gatewayDefaultHint} />
            </Typography>
          )}

          {ownError && <Box sx={classes.errorText}>{ownError}</Box>}

          {/* Add button for complex arrays */}
          {isComplexArray && (maxItems === undefined || items.length < maxItems) && (
            <Box sx={classes.arrayContainer}>
              <Button
                variant="outlined"
                size="small"
                startIcon={<Plus size={14} />}
                onClick={() => {
                  form.addItem(node.path);
                  setIsExpanded(true);
                }}
                disabled={disabled}
                data-testid={`add-${testPath(node.path)}-item`}
                sx={classes.arrayAddButton}
              >
                <FormattedMessage {...messages.addItem} />
              </Button>
            </Box>
          )}
        </Box>
      </Box>

      {/* Children (for objects) - split into normal and advanced */}
      {isObject && !isOffSection && (
        <Collapse in={expanded}>
          <Box sx={classes.nestedChildren}>
            <ChildrenWithAdvanced parent={node} depth={depth + 1} {...ctx} />
          </Box>
        </Collapse>
      )}

      {isObject && field.description && !isArrayItem && (
        <TruncatedDescription description={field.description} classes={classes} />
      )}

      {/* Array Items (for complex arrays) */}
      {isComplexArray && (
        <Collapse in={expanded}>
          <Box sx={classes.nodeChildren}>
            {items.map((item, index) => (
              <Box key={item.pathKey} sx={classes.arrayItem}>
                <SchemaTreeNodeComponent
                  node={item}
                  depth={depth + 1}
                  defaultExpanded={index === newItemIndex}
                  {...ctx}
                />
              </Box>
            ))}
          </Box>
        </Collapse>
      )}
    </Box>
  );
};

interface SchemaTreeProps {
  /** The engine's resolved view of the whole form (resolution.view) */
  view: ViewNode;
  form: PolicyForm;
  /** Show validation errors (after the first submit attempt) */
  showErrors: boolean;
  disabled?: boolean;
}

/**
 * Main Schema Tree component that renders the full parameter tree
 * Splits first-level parameters into required and optional (advanced) sections
 */
const SchemaTree: React.FC<SchemaTreeProps> = ({ view, form, showErrors, disabled }) => {
  const classes = useStyles();
  // Rules that span several top-level fields (e.g. "this combination isn't allowed")
  const rootErrors = showErrors ? view.errors : [];

  return (
    <Box>
      {rootErrors.map((e) => (
        <Box key={e} sx={classes.errorText} role="alert">
          {e}
        </Box>
      ))}
      <ChildrenWithAdvanced
        parent={view}
        depth={0}
        form={form}
        showErrors={showErrors}
        disabled={disabled}
      />
    </Box>
  );
};

export default SchemaTree;
