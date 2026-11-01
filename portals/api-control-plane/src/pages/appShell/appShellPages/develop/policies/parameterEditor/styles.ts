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

import { useMemo } from 'react';
import { useTheme } from '@wso2/oxygen-ui';

/** Lines of a description shown before "Show more". */
export const MAX_DESCRIPTION_LINES = 5;

/**
 * Style recipes for the policy parameter editor. Colour, border and radius come
 * from theme tokens; everything else is layout and type scale.
 */
export const useStyles = () => {
  const theme = useTheme();

  return useMemo(() => {
    const clamp = {
      display: '-webkit-box',
      WebkitBoxOrient: 'vertical',
      WebkitLineClamp: MAX_DESCRIPTION_LINES,
      overflow: 'hidden',
    } as const;
    // Muted so the label and the input stay the focus; `maxWidth: none` because the
    // theme caps Typography blocks, which wrapped these well short of the field.
    const description = {
      color: 'text.secondary',
      fontSize: '0.75rem',
      lineHeight: 1.6,
      marginBottom: 0,
      marginTop: theme.spacing(0.75),
      maxWidth: 'none',
      opacity: 0.7,
      whiteSpace: 'pre-wrap',
      width: '100%',
    } as const;
    const deleteButton = {
      color: 'error.main',
      flexShrink: 0,
      padding: theme.spacing(0.25),
      '&:hover': { color: 'error.dark' },
    } as const;

    return {
      root: {
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        minHeight: 0,
        width: '100%',
      },
      body: { flex: 1, minHeight: 0, overflowY: 'auto', padding: theme.spacing(3) },
      problemsPanel: {
        borderTop: '1px solid',
        borderColor: 'divider',
        maxHeight: 240,
        overflowY: 'auto',
        padding: theme.spacing(2, 3),
      },
      problemsHeader: { alignItems: 'center', display: 'flex', marginBottom: theme.spacing(1) },
      problemsTitle: {
        color: 'text.secondary',
        fontSize: '0.75rem',
        letterSpacing: '0.04em',
        textTransform: 'uppercase',
      },
      problemsList: { listStyle: 'none', margin: 0, padding: 0 },
      problemRow: {
        alignItems: 'center',
        color: 'error.main',
        display: 'flex',
        gap: theme.spacing(1),
        minHeight: theme.spacing(4.5),
      },
      problemPath: {
        color: 'text.primary',
        fontFamily: 'monospace',
        fontSize: '0.8125rem',
        overflowWrap: 'anywhere',
      },
      problemMessage: { color: 'error.main', fontSize: '0.8125rem' },
      goToFieldButton: {
        color: 'text.secondary',
        fontSize: '0.8125rem',
        marginLeft: 'auto',
        textTransform: 'none',
      },
      footer: {
        alignItems: 'center',
        borderTop: '1px solid',
        borderColor: 'divider',
        display: 'flex',
        gap: theme.spacing(2),
        justifyContent: 'space-between',
        padding: theme.spacing(2, 3),
      },
      footerActions: { display: 'flex', gap: theme.spacing(1) },
      readyStatus: {
        alignItems: 'center',
        color: 'success.main',
        display: 'flex',
        gap: theme.spacing(1),
      },
      header: {
        display: 'flex',
        flexDirection: 'column',
        gap: theme.spacing(1),
        marginBottom: theme.spacing(2),
      },
      headerTitle: { fontSize: '1.25rem', fontWeight: 600 },
      headerDescription: {
        ...description,
        fontSize: '0.8125rem',
        margin: 0,
        marginTop: theme.spacing(0.5),
      },
      headerDescriptionTruncated: {
        ...description,
        ...clamp,
        fontSize: '0.8125rem',
        margin: 0,
        marginTop: theme.spacing(0.5),
      },
      treeNode: { marginBottom: theme.spacing(2) },
      nodeRow: { alignItems: 'flex-start', display: 'flex', padding: 0 },
      nestedSection: {
        borderLeft: '2px solid',
        borderColor: 'divider',
        paddingLeft: theme.spacing(1.5),
      },
      nestedChildren: {
        borderLeft: '2px solid',
        borderColor: 'divider',
        marginTop: theme.spacing(1),
        paddingLeft: theme.spacing(1.5),
        '& > .MuiBox-root > .MuiBox-root:last-child': { marginBottom: 0 },
      },
      expandIcon: {
        alignItems: 'center',
        color: 'text.primary',
        cursor: 'pointer',
        display: 'flex',
        flexShrink: 0,
        height: 16,
        justifyContent: 'center',
        padding: 0,
        width: 16,
        '&:hover': { color: 'primary.main' },
      },
      nodeContent: { flex: 1, minWidth: 0 },
      nodeLabelRow: {
        alignItems: 'center',
        display: 'flex',
        gap: theme.spacing(1),
        marginBottom: theme.spacing(0.5),
      },
      nodeLabel: { fontSize: '0.8125rem', fontWeight: 600 },
      requiredBadge: { color: 'error.main', fontSize: '0.75rem' },
      deprecatedBadge: { fontSize: '0.7rem', height: theme.spacing(2.5) },
      nodeDescription: description,
      nodeDescriptionTruncated: { ...description, ...clamp },
      showMoreButton: {
        color: 'primary.main',
        cursor: 'pointer',
        fontSize: '0.7rem',
        marginBottom: theme.spacing(1),
        '&:hover': { textDecoration: 'underline' },
      },
      nodeChildren: { marginLeft: 0, marginTop: theme.spacing(0.5), paddingLeft: 0 },
      fieldContainer: { marginTop: theme.spacing(0.5) },
      fieldInput: { '& .MuiInputBase-root': { fontSize: '0.875rem' } },
      enumSelect: { fontSize: '0.875rem', minWidth: 0 },
      tagInputContainer: {
        display: 'flex',
        flexDirection: 'column',
        gap: theme.spacing(0.5),
        marginTop: theme.spacing(0.5),
      },
      tagInputChips: { display: 'flex', flexWrap: 'wrap', gap: theme.spacing(0.5) },
      tagInputField: { '& .MuiInputBase-root': { fontSize: '0.875rem' } },
      tagInputHelperText: { marginLeft: theme.spacing(0.5) },
      arrayContainer: { marginTop: theme.spacing(1) },
      arrayAddButton: { fontSize: '0.75rem' },
      arrayItem: {
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: `${theme.shape.borderRadius}px`,
        marginBottom: theme.spacing(0.75),
        padding: theme.spacing(0.5, 1),
        position: 'relative',
      },
      arrayItemDeleteButton: { ...deleteButton, marginLeft: 'auto' },
      toggleContainer: { alignItems: 'center', display: 'flex' },
      errorText: {
        color: 'error.main',
        fontSize: '0.75rem',
        marginTop: theme.spacing(0.5),
      },
      keyValueRow: {
        alignItems: 'start',
        display: 'grid',
        gap: theme.spacing(1),
        gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1fr) auto',
        marginBottom: theme.spacing(1),
      },
      keyValueKey: { minWidth: 0, '& .MuiInputBase-root': { fontSize: '0.875rem' } },
      keyValueValue: {
        minWidth: 0,
        '& > .MuiBox-root': { marginTop: 0 },
        '& .MuiInputBase-root': { fontSize: '0.875rem' },
      },
      keyValueDeleteButton: { ...deleteButton, marginTop: theme.spacing(1) },
      groupSelector: {
        border: '1px solid',
        borderColor: 'primary.main',
        borderRadius: `${theme.shape.borderRadius}px`,
        margin: theme.spacing(0, 0, 2.5),
        minWidth: 0,
        padding: theme.spacing(1, 2),
      },
      groupSelectorLabel: {
        color: 'primary.main',
        fontSize: '0.75rem',
        fontWeight: 600,
        letterSpacing: '0.04em',
        padding: theme.spacing(0, 0.75),
        textTransform: 'uppercase',
      },
      choiceLabel: { fontSize: '0.8rem' },
      multiSelectRow: { columnGap: theme.spacing(1), display: 'flex', flexWrap: 'wrap' },
      monospaceInput: { '& textarea': { fontFamily: 'monospace', fontSize: '0.8rem' } },
      gatewayDefaultHint: {
        color: 'text.secondary',
        fontSize: '0.7rem',
        fontStyle: 'italic',
        marginBottom: theme.spacing(0.5),
      },
      sectionClearButton: {
        color: 'text.secondary',
        marginLeft: 'auto',
        padding: theme.spacing(0.25),
      },
    };
  }, [theme]);
};
