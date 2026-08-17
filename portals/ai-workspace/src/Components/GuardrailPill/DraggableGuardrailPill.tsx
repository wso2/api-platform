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

import React from 'react';
import { Box } from '@wso2/oxygen-ui';
import GuardrailPill from './GuardrailPill';

export type DraggableGuardrailPillProps = {
  id: string;
  label: string;
  reorderable: boolean;
  isDragging: boolean;
  isDragOver: boolean;
  onDragStart: () => void;
  onDragOver: () => void;
  onDrop: () => void;
  onDragEnd: () => void;
  onClick?: () => void;
  onRemove?: () => void;
};

export const reorderItem = <T,>(
  items: T[],
  sourceIndex: number,
  targetIndex: number
) => {
  if (
    sourceIndex < 0 ||
    sourceIndex >= items.length ||
    targetIndex < 0 ||
    targetIndex >= items.length
  ) {
    return null;
  }
  const reordered = [...items];
  const [movedItem] = reordered.splice(sourceIndex, 1);
  reordered.splice(targetIndex, 0, movedItem);
  return reordered;
};

export default function DraggableGuardrailPill({
  id,
  label,
  reorderable,
  isDragging,
  isDragOver,
  onDragStart,
  onDragOver,
  onDrop,
  onDragEnd,
  onClick,
  onRemove,
}: DraggableGuardrailPillProps) {
  return (
    <Box
      draggable={reorderable}
      onDragStart={(event) => {
        if (!reorderable) return;
        event.dataTransfer.effectAllowed = 'move';
        event.dataTransfer.setData('text/plain', id);
        onDragStart();
      }}
      onDragEnd={reorderable ? onDragEnd : undefined}
      onDragOver={(event) => {
        if (!reorderable) return;
        event.preventDefault();
        event.dataTransfer.dropEffect = 'move';
        onDragOver();
      }}
      onDrop={(event) => {
        if (!reorderable) return;
        event.preventDefault();
        onDrop();
      }}
      aria-label={reorderable ? `Drag to reorder ${label}` : undefined}
      sx={{
        display: 'inline-flex',
        borderRadius: 0.5,
        opacity: isDragging ? 0.55 : 1,
        outline: isDragOver ? '2px solid' : 'none',
        outlineColor: 'primary.main',
        outlineOffset: 2,
        '& > div': {
          cursor: reorderable ? 'grab' : undefined,
        },
        '&:active > div': {
          cursor: reorderable ? 'grabbing' : undefined,
        },
      }}
    >
      <GuardrailPill label={label} onClick={onClick} onRemove={onRemove} />
    </Box>
  );
}
