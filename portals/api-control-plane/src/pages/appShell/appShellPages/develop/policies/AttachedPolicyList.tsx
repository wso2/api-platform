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
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { GripVertical, Pencil, Plus, Shield, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { Policy } from '@/api/resources/restApis';
import { hairline } from '@/theme/receipes';
import { versionLabel } from '@/utils/versionLabel';
import { PolicyChip, PolicyName } from './PolicyChip';

/** Bullet between the legend and the hint; punctuation, not translatable text. */
const SEPARATOR = '\u00b7';

const messages = defineMessages({
  inheritedFromGlobal: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AttachedPolicyList.inheritedFromGlobal',
    defaultMessage: 'Inherited from global',
    description:
      'Legend for the dashed chips: policies this resource gets from the API-level section.',
  },
  reorderHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AttachedPolicyList.reorderHint',
    defaultMessage: 'Drag to reorder this resource\u2019s policies',
    description: 'Hint under a resource\u2019s policy chips.',
  },
  heading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AttachedPolicyList.heading',
    defaultMessage: 'Policies',
    description: 'Heading over the list of policies attached at this scope. Noun.',
  },
  addPolicy: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AttachedPolicyList.addPolicy',
    defaultMessage: 'Add Policy',
    description: 'Button that attaches a policy from the catalog. Verb phrase.',
  },
  empty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AttachedPolicyList.empty',
    defaultMessage: 'No policies attached.',
    description: 'Default placeholder when nothing is attached at this scope.',
  },
  edit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AttachedPolicyList.edit',
    defaultMessage: 'Edit',
    description: 'Tooltip on the button that reconfigures an attached policy. Verb.',
  },
  editLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AttachedPolicyList.editLabel',
    defaultMessage: 'Edit policy',
    description: 'Accessible label for the edit button on one attached policy.',
  },
  remove: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AttachedPolicyList.remove',
    defaultMessage: 'Remove',
    description: 'Tooltip on the button that detaches a policy. Verb.',
  },
  removeLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.AttachedPolicyList.removeLabel',
    defaultMessage: 'Remove policy',
    description: 'Accessible label for the remove button on one attached policy.',
  },
});

/**
 * Renders a flat, ordered list of attached policies (the Hybrid-style policy
 * list) with edit / delete and drag-to-reorder, plus an Add button + empty
 * state. Reordering uses native HTML5 drag within this list only.
 *
 * `readOnly` lists the policies without any of that — no add, edit, remove or
 * reorder — for an API the console must not change (one discovered from a
 * data-plane gateway).
 */
export function AttachedPolicyList({
  policies,
  canAdd,
  onAdd,
  onEdit,
  onRemove,
  onReorder,
  emptyText,
  readOnly = false,
  showHeader = true,
  variant = 'list',
  inheritedPolicies = [],
}: {
  policies: Policy[];
  canAdd: boolean;
  onAdd: () => void;
  onEdit: (index: number) => void;
  onRemove: (index: number) => void;
  onReorder: (from: number, to: number) => void;
  /** Overrides the default placeholder; already-translated text. */
  emptyText?: string;
  /** Shows the policies only, with no way to change them. */
  readOnly?: boolean;
  showHeader?: boolean;
  /**
   * `chips` lays the policies out as numbered pills in an inset well, for a scope
   * with few policies (the API-level one). `resource` is the same pills in a row
   * that follows the inherited ones, with no well. `list` is one full-width row each.
   */
  variant?: 'list' | 'chips' | 'resource';
  /** For `resource`: the parent scope's policies, shown first as dashed, read-only chips. */
  inheritedPolicies?: Policy[];
}) {
  const intl = useIntl();
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [overIndex, setOverIndex] = useState<number | null>(null);
  // Defaulted here rather than in the signature: a default parameter is
  // evaluated before `useIntl` exists, so the fallback could not be translated.
  const emptyLabel = emptyText ?? intl.formatMessage(messages.empty);
  const canModify = canAdd && !readOnly;
  const editable = !readOnly;

  /** Native drag-to-reorder within this list, shared by every layout. */
  const reorderHandlers = (index: number) => ({
    onDragEnd: () => {
      setDragIndex(null);
      setOverIndex(null);
    },
    onDragOver: (event: React.DragEvent) => {
      if (dragIndex === null) return;
      event.preventDefault();
      setOverIndex(index);
    },
    onDragStart: (event: React.DragEvent) => {
      if (!editable) return;
      setDragIndex(index);
      event.dataTransfer.effectAllowed = 'move';
      // Mark as an internal reorder so external policy drops ignore it.
      event.dataTransfer.setData('application/x-policy-reorder', String(index));
    },
    onDrop: (event: React.DragEvent) => {
      event.preventDefault();
      event.stopPropagation();
      if (dragIndex !== null && dragIndex !== index) onReorder(dragIndex, index);
      setDragIndex(null);
      setOverIndex(null);
    },
  });

  const ownChips = (offset = 0) =>
    policies.map((policy, index) => {
      const isOver = overIndex === index && dragIndex !== null && dragIndex !== index;
      return (
        <PolicyChip
          canEdit={canAdd}
          dragging={dragIndex === index}
          dropTarget={isOver}
          editable={editable}
          index={index}
          position={offset + index + 1}
          key={`${policy.name}-${index}`}
          {...reorderHandlers(index)}
          onEdit={() => onEdit(index)}
          onRemove={() => onRemove(index)}
          policy={policy}
        />
      );
    });

  if (variant === 'resource') {
    // Nothing inherited and nothing of its own: just the drop hint, centred in the card.
    if (inheritedPolicies.length === 0 && policies.length === 0) {
      return (
        <Box
          sx={{
            alignItems: 'center',
            color: 'text.secondary',
            display: 'flex',
            justifyContent: 'center',
            minHeight: 56,
          }}
        >
          <Typography sx={{ opacity: 0.7 }} variant="body2">
            {emptyLabel}
          </Typography>
        </Box>
      );
    }
    return (
      <Box>
        <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1 }}>
          {inheritedPolicies.map((policy, index) => (
            <PolicyChip
              canEdit={false}
              editable={false}
              inherited
              index={index}
              key={`inherited-${policy.name}-${index}`}
              onEdit={() => undefined}
              onRemove={() => undefined}
              policy={policy}
            />
          ))}
          {ownChips(inheritedPolicies.length)}
        </Box>
        <Box
          sx={{ alignItems: 'center', color: 'text.secondary', display: 'flex', gap: 1, mt: 1.25 }}
        >
          {inheritedPolicies.length > 0 && (
            <>
              <Box
                sx={(theme) => ({
                  border: hairline(theme),
                  borderColor: 'divider',
                  borderRadius: 0.5,
                  borderStyle: 'dashed',
                  height: 14,
                  width: 14,
                })}
              />
              <Typography sx={{ opacity: 0.7 }} variant="caption">
                <FormattedMessage {...messages.inheritedFromGlobal} />
              </Typography>
              <Typography sx={{ opacity: 0.7 }} variant="caption">
                {SEPARATOR}
              </Typography>
            </>
          )}
          <Typography sx={{ opacity: 0.7 }} variant="caption">
            {policies.length === 0 && editable ? (
              emptyLabel
            ) : (
              <FormattedMessage {...messages.reorderHint} />
            )}
          </Typography>
        </Box>
      </Box>
    );
  }

  if (variant === 'chips') {
    return (
      // A dashed, recessed well inside the section card reads as the drop area: it is
      // the one place policies are dropped, reordered and edited.
      <Card
        sx={(theme) => ({
          bgcolor: 'background.default',
          border: hairline(theme),
          borderColor: 'divider',
          borderStyle: 'dashed',
          boxShadow: 'none',
        })}
      >
        <CardContent sx={{ '&:last-child': { pb: 1.5 }, p: 1.5 }}>
          {policies.length === 0 ? (
            <Box
              sx={{
                alignItems: 'center',
                color: 'text.secondary',
                display: 'flex',
                justifyContent: 'center',
                minHeight: 56,
              }}
            >
              <Typography variant="body2">{emptyLabel}</Typography>
            </Box>
          ) : (
            <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1 }}>{ownChips()}</Box>
          )}
        </CardContent>
      </Card>
    );
  }

  return (
    <Box>
      {showHeader && (
        <Box
          sx={{
            alignItems: 'center',
            display: 'flex',
            justifyContent: 'space-between',
            mb: 1,
          }}
        >
          <Typography sx={{ fontWeight: 600 }} variant="body2">
            <FormattedMessage {...messages.heading} />
          </Typography>
          {canModify && (
            <Button onClick={onAdd} size="small" startIcon={<Plus size={14} />} variant="outlined">
              <FormattedMessage {...messages.addPolicy} />
            </Button>
          )}
        </Box>
      )}

      {policies.length === 0 ? (
        <Box
          sx={{
            alignItems: 'center',
            border: '2px dashed',
            borderColor: 'divider',
            borderRadius: 1,
            color: 'text.secondary',
            display: 'flex',
            justifyContent: 'center',
            minHeight: 120,
            px: 2,
            py: 1.5,
          }}
        >
          <Typography variant="body2">{emptyLabel}</Typography>
        </Box>
      ) : (
        <Stack spacing={0.75}>
          {policies.map((policy, index) => {
            const isOver = overIndex === index && dragIndex !== null && dragIndex !== index;
            return (
              <Box
                draggable={editable}
                key={`${policy.name}-${index}`}
                {...reorderHandlers(index)}
                sx={{
                  alignItems: 'center',
                  bgcolor: 'background.paper',
                  border: '1px solid',
                  borderColor: isOver ? 'primary.main' : 'divider',
                  borderRadius: 1,
                  borderTopWidth: isOver ? 3 : 1,
                  display: 'flex',
                  gap: 1,
                  opacity: dragIndex === index ? 0.5 : 1,
                  px: 1.25,
                  py: 1,
                }}
              >
                {editable && (
                  <Box sx={{ color: 'text.disabled', cursor: 'grab', display: 'flex' }}>
                    <GripVertical size={16} />
                  </Box>
                )}
                <Shield size={16} />
                <Typography noWrap sx={{ flex: 1, fontWeight: 500 }} variant="body2">
                  <PolicyName name={policy.name} />
                </Typography>
                <Chip label={versionLabel(policy.version)} size="small" variant="outlined" />
                {editable && (
                  <Stack direction="row">
                    <Tooltip title={intl.formatMessage(messages.edit)}>
                      <IconButton
                        aria-label={intl.formatMessage(messages.editLabel)}
                        disabled={!canAdd}
                        onClick={() => onEdit(index)}
                        size="small"
                      >
                        <Pencil size={14} />
                      </IconButton>
                    </Tooltip>
                    <Tooltip title={intl.formatMessage(messages.remove)}>
                      <IconButton
                        aria-label={intl.formatMessage(messages.removeLabel)}
                        color="error"
                        onClick={() => onRemove(index)}
                        size="small"
                      >
                        <Trash2 size={14} />
                      </IconButton>
                    </Tooltip>
                  </Stack>
                )}
              </Box>
            );
          })}
        </Stack>
      )}

      {canAdd && editable && policies.length > 0 && (
        <Box
          sx={{
            border: '1px dashed',
            borderColor: 'divider',
            borderRadius: 1,
            color: 'text.secondary',
            mt: 0.75,
            px: 2,
            py: 1,
            textAlign: 'center',
          }}
        >
          <Typography variant="caption">{emptyLabel}</Typography>
        </Box>
      )}
    </Box>
  );
}
