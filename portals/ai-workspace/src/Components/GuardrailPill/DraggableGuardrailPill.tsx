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
import { Box, IconButton } from '@wso2/oxygen-ui';
import { ChevronLeft, ChevronRight } from '@wso2/oxygen-ui-icons-react';
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
  onKeyboardMove?: (direction: -1 | 1) => void;
  onClick?: () => void;
  onRemove?: () => void;
};

export const reorderItem = <T,>(
  items: T[],
  sourceIndex: number,
  targetIndex: number
) => {
  if (
    !Number.isInteger(sourceIndex) ||
    !Number.isInteger(targetIndex) ||
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

export const reorderItemsWithinIndexes = <T,>(
  items: T[],
  visibleIndexes: number[],
  sourceIndex: number,
  targetIndex: number
) => {
  if (
    visibleIndexes.some((index) => !Number.isInteger(index)) ||
    new Set(visibleIndexes).size !== visibleIndexes.length ||
    visibleIndexes.some((index) => index < 0 || index >= items.length)
  ) {
    return null;
  }
  const sourcePosition = visibleIndexes.indexOf(sourceIndex);
  const targetPosition = visibleIndexes.indexOf(targetIndex);
  const visibleItems = visibleIndexes.map((index) => items[index]);
  const reorderedVisibleItems = reorderItem(
    visibleItems,
    sourcePosition,
    targetPosition
  );
  if (!reorderedVisibleItems) return null;

  const reordered = [...items];
  visibleIndexes.forEach((index, position) => {
    reordered[index] = reorderedVisibleItems[position];
  });
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
  onKeyboardMove,
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
      role={reorderable ? 'group' : undefined}
      tabIndex={reorderable ? 0 : undefined}
      onKeyDown={(event) => {
        if (!reorderable || !onKeyboardMove) return;
        if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
          event.preventDefault();
          event.stopPropagation();
          onKeyboardMove(-1);
        } else if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
          event.preventDefault();
          event.stopPropagation();
          onKeyboardMove(1);
        }
      }}
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
      {reorderable && onKeyboardMove ? (
        <IconButton
          size="small"
          aria-label={`Move ${label} earlier`}
          onClick={(event) => {
            event.stopPropagation();
            onKeyboardMove(-1);
          }}
          sx={{
            display: 'none',
            '@media (hover: none), (pointer: coarse)': { display: 'inline-flex' },
          }}
        >
          <ChevronLeft size={14} />
        </IconButton>
      ) : null}
      <GuardrailPill label={label} onClick={onClick} onRemove={onRemove} />
      {reorderable && onKeyboardMove ? (
        <IconButton
          size="small"
          aria-label={`Move ${label} later`}
          onClick={(event) => {
            event.stopPropagation();
            onKeyboardMove(1);
          }}
          sx={{
            display: 'none',
            '@media (hover: none), (pointer: coarse)': { display: 'inline-flex' },
          }}
        >
          <ChevronRight size={14} />
        </IconButton>
      ) : null}
    </Box>
  );
}
