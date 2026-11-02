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

import { Box, IconButton, Tooltip, Typography } from '@wso2/oxygen-ui';
import type { BoxProps } from '@wso2/oxygen-ui';
import { GripVertical, Pencil, X } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, useIntl } from 'react-intl';

import { usePolicyVersions } from '@/api/resources/policyHub';
import type { Policy } from '@/api/resources/restApis';
import { inheritedPillSx, tintedPillSx } from '@/theme/receipes';

const messages = defineMessages({
  edit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyChip.edit',
    defaultMessage: 'Edit',
    description: 'Tooltip on the button that reconfigures an attached policy. Verb.',
  },
  editLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyChip.editLabel',
    defaultMessage: 'Edit policy',
    description: 'Accessible label for the edit button on one attached policy.',
  },
  removeLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.policies.PolicyChip.removeLabel',
    defaultMessage: 'Remove policy',
    description: 'Accessible label for the remove button on one attached policy.',
  },
});

/**
 * The name to show for an attached policy: the catalog's display name ("API Key
 * Auth") rather than its handle ("api-key-auth"). The handle is shown until the
 * catalog answers, and stays if the Policy Hub is not configured or has no entry.
 */
export function usePolicyDisplayName(name: string): string {
  const versions = usePolicyVersions(name);
  const entries = versions.data ?? [];
  const entry = entries.find((candidate) => candidate.isLatest) ?? entries[0];
  return entry?.displayName || name;
}

/** An attached policy's display name, for places that only need the text. */
export function PolicyName({ name }: { name: string }) {
  return <>{usePolicyDisplayName(name)}</>;
}

/** Muted until hovered, so the policy name is the one thing that reads at full strength. */
const actionSx = {
  color: 'text.secondary',
  opacity: 0.55,
  p: 0.375,
  '&:hover': { opacity: 1 },
} as const;

type DragHandlers = Pick<BoxProps, 'onDragEnd' | 'onDragOver' | 'onDragStart' | 'onDrop'>;

export type PolicyChipProps = {
  policy: Policy;
  /** Zero-based position among the scope's own policies. */
  index: number;
  /** The number shown on the chip, when it differs from `index + 1` (e.g. after inherited ones). */
  position?: number;
  /** A policy this scope inherits from the one above: dashed, muted and read-only. */
  inherited?: boolean;
  /** False lists the policy only: no handle, edit or remove, and not draggable. */
  editable: boolean;
  /** Whether the config drawer can open (the Policy Hub is configured). */
  canEdit: boolean;
  /** This chip is the one being dragged. */
  dragging?: boolean;
  /** Another chip is being dragged over this one. */
  dropTarget?: boolean;
  onEdit: () => void;
  onRemove: () => void;
} & DragHandlers;

/**
 * One attached policy as a chip: drag handle, position, display name, and edit /
 * remove actions. Dragging a chip onto another reorders them.
 */
export function PolicyChip({
  policy,
  index,
  position,
  inherited = false,
  editable: editableProp,
  canEdit,
  dragging = false,
  dropTarget = false,
  onEdit,
  onRemove,
  ...dragHandlers
}: PolicyChipProps) {
  const intl = useIntl();
  const displayName = usePolicyDisplayName(policy.name);
  const editable = editableProp && !inherited;

  return (
    <Box
      draggable={editable}
      {...dragHandlers}
      sx={(theme) => ({
        ...(inherited ? inheritedPillSx(theme) : tintedPillSx(theme, dropTarget)),
        gap: 0.75,
        opacity: dragging ? 0.5 : 1,
        pl: 1.25,
        pr: editable ? 0.5 : 1.5,
        py: 0.75,
      })}
    >
      {editable && (
        <Box sx={{ color: 'text.secondary', cursor: 'grab', display: 'flex', opacity: 0.5 }}>
          <GripVertical size={14} />
        </Box>
      )}
      <Typography color="text.secondary" sx={{ opacity: 0.6 }} variant="body1">
        {position ?? index + 1}
      </Typography>
      <Typography color={inherited ? 'text.secondary' : undefined} noWrap variant="body1">
        {displayName}
      </Typography>
      {editable && (
        <Box sx={{ alignItems: 'center', display: 'flex', ml: 0.25 }}>
          <Tooltip title={intl.formatMessage(messages.edit)}>
            <span>
              <IconButton
                aria-label={intl.formatMessage(messages.editLabel)}
                disabled={!canEdit}
                onClick={onEdit}
                size="small"
                sx={actionSx}
              >
                <Pencil size={16} />
              </IconButton>
            </span>
          </Tooltip>
          <IconButton
            aria-label={intl.formatMessage(messages.removeLabel)}
            onClick={onRemove}
            size="small"
            sx={actionSx}
          >
            <X size={16} />
          </IconButton>
        </Box>
      )}
    </Box>
  );
}
