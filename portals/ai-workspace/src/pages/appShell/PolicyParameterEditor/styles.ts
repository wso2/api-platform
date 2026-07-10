/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you've
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

import { useMemo } from 'react';
import { useTheme } from '@wso2/oxygen-ui';

export const useStyles = () => {
  const theme = useTheme();

  return useMemo(
    () => ({
      root: {
        width: '100%',
      },
      header: {
        marginBottom: theme.spacing(2),
        gap: theme.spacing(1),
        display: 'flex',
        flexDirection: 'column',
      },
      headerTitle: {
        fontWeight: 600,
        fontSize: '1rem',
        // color: theme.palette.text.primary,
      },
      headerDescription: {
        color: theme.palette.text.secondary,
        marginTop: theme.spacing(0.5),
        // fontSize: '13px',
        lineHeight: 1.5,
        whiteSpace: 'pre-wrap',
        fontFamily: 'inherit',
        margin: 0,
      },
      headerDescriptionTruncated: {
        color: theme.palette.text.secondary,
        marginTop: theme.spacing(0.5),
        // fontSize: '13px',
        lineHeight: 1.5,
        whiteSpace: 'pre-wrap',
        fontFamily: 'inherit',
        margin: 0,
        display: '-webkit-box',
        WebkitLineClamp: 5,
        WebkitBoxOrient: 'vertical',
        overflow: 'hidden',
      },
      treeContainer: {
        border: `1px solid ${theme.palette.divider}`,
        borderRadius: theme.spacing(1),
        padding: theme.spacing(1),
        // backgroundColor:
        //   (theme as { palette?: { mode?: string } }).palette?.mode === 'dark'
        //     ? '#585858'
        //     : 'transparent',
      },
      // Tree node styles
      treeNode: {
        marginBottom: theme.spacing(0.5),
      },
      nodeRow: {
        display: 'flex',
        alignItems: 'flex-start',
        padding: theme.spacing(0.75, 1, 0.75, 0),
        borderRadius: theme.spacing(0.8),
        '&:hover': {
          backgroundColor: theme.palette.action.hover,
        },
      },
      nodeRowExpanded: {
        // backgroundColor: '#c2c2c2',
        border: `1px solid #909090`,
      },
      expandIcon: {
        cursor: 'pointer',
        marginRight: theme.spacing(0.5),
        marginTop: 2, // Align with label text
        color: theme.palette.text.secondary,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        width: 16,
        height: 16,
        flexShrink: 0,
        '& svg': {
          fontSize: 14,
        },
        '&:hover': {
          color: theme.palette.primary.main,
        },
      },
      expandIconPlaceholder: {
        width: 16,
        marginRight: theme.spacing(0.5),
        marginTop: 3,
        flexShrink: 0,
      },
      nodeIcon: {
        marginRight: theme.spacing(1),
        color: theme.palette.text.secondary,
        fontSize: 18,
      },
      nodeContent: {
        flex: 1,
        minWidth: 0,
      },
      nodeLabelRow: {
        display: 'flex',
        alignItems: 'center',
        gap: theme.spacing(1),
        marginBottom: theme.spacing(0.5),
      },
      nodeLabel: {
        fontWeight: 500,
        fontSize: '0.875rem',
        // color: theme.palette.text.primary,
      },
      requiredBadge: {
        color: theme.palette.error.main,
        fontSize: '0.75rem',
      },
      typeBadge: {
        fontSize: '0.7rem',
        padding: theme.spacing(0.25, 0.75),
        borderRadius: theme.spacing(0.5),
        backgroundColor: theme.palette.grey[100],
        // color: theme.palette.text.secondary,
        fontFamily: 'monospace',
      },
      nodeDescription: {
        fontSize: '0.75rem',
        // color: theme.palette.text.secondary,
        lineHeight: 1.4,
        marginBottom: theme.spacing(1),
        whiteSpace: 'pre-wrap',
      },
      nodeDescriptionTruncated: {
        fontSize: '0.75rem',
        color: theme.palette.text.secondary,
        lineHeight: 1.4,
        marginBottom: theme.spacing(0.5),
        whiteSpace: 'pre-wrap',
        display: '-webkit-box',
        WebkitLineClamp: 5,
        WebkitBoxOrient: 'vertical',
        overflow: 'hidden',
      },
      showMoreButton: {
        fontSize: '0.7rem',
        padding: 0,
        minWidth: 'auto',
        marginBottom: theme.spacing(1),
        textTransform: 'none',
        color: theme.palette.primary.main,
        '&:hover': {
          backgroundColor: 'transparent',
          textDecoration: 'underline',
        },
      },
      nodeChildren: {
        marginTop: theme.spacing(0.5),
        marginLeft: 0,
        paddingLeft: 0,
      },
      // Field input styles
      fieldContainer: {
        marginTop: theme.spacing(0.5),
      },
      fieldInput: {
        '& .MuiInputBase-root': {
          fontSize: '0.875rem',
        },
      },
      enumSelect: {
        minWidth: 200,
      },
      // Tag input styles
      tagInputContainer: {
        display: 'flex',
        flexDirection: 'column',
        gap: theme.spacing(0.5),
        marginTop: theme.spacing(0.5),
      },
      tagInputChips: {
        display: 'flex',
        flexWrap: 'wrap',
        gap: theme.spacing(0.5),
      },
      tagInputField: {
        '& .MuiInputBase-root': {
          fontSize: '0.875rem',
        },
      },
      tagInputHelperText: {
        marginLeft: theme.spacing(0.5),
      },
      // Array styles
      arrayContainer: {
        marginTop: theme.spacing(1),
      },
      arrayHeader: {
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        marginBottom: theme.spacing(1),
      },
      arrayAddButton: {
        fontSize: '0.75rem',
      },
      arrayItem: {
        position: 'relative',
        border: `1px solid ${theme.palette.divider}`,
        borderRadius: theme.spacing(0.5),
        padding: theme.spacing(0.5, 1),
        marginBottom: theme.spacing(0.75),
        // backgroundColor: theme.palette.background.default,
      },
      arrayItemHeader: {
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        marginBottom: theme.spacing(1),
      },
      arrayItemTitle: {
        fontWeight: 500,
        fontSize: '0.8rem',
        color: theme.palette.text.secondary,
      },
      arrayItemDeleteButton: {
        padding: theme.spacing(0.25),
        marginLeft: 'auto',
        color: theme.palette.error.main,
        '& svg': {
          fontSize: 16,
        },
        '&:hover': {
          color: theme.palette.error.dark,
          backgroundColor: theme.palette.error.light + '20',
        },
      },
      // Toggle styles
      toggleContainer: {
        display: 'flex',
        alignItems: 'center',
      },
      // Buttons
      buttonContainer: {
        display: 'flex',
        justifyContent: 'flex-end',
        gap: theme.spacing(1),
        marginTop: theme.spacing(3),
        paddingTop: theme.spacing(2),
        borderTop: `1px solid ${theme.palette.divider}`,
      },
      // Error styles
      errorText: {
        color: theme.palette.error.main,
        fontSize: '0.75rem',
        marginTop: theme.spacing(0.5),
      },
      // Key-value (dynamic object) styles
      keyValueRow: {
        display: 'flex',
        alignItems: 'center',
        gap: theme.spacing(1),
        marginBottom: theme.spacing(1),
      },
      keyValueKey: {
        flex: '1 1 40%',
        '& .MuiInputBase-root': {
          fontSize: '0.875rem',
        },
      },
      keyValueValue: {
        flex: '1 1 50%',
        '& .MuiInputBase-root': {
          fontSize: '0.875rem',
        },
      },
      keyValueDeleteButton: {
        padding: theme.spacing(0.25),
        flexShrink: 0,
        color: theme.palette.error.main,
        '& svg': {
          fontSize: 16,
        },
        '&:hover': {
          color: theme.palette.error.dark,
          backgroundColor: theme.palette.error.light + '20',
        },
      },
      // Advanced settings section
      advancedSettingsToggle: {
        marginTop: theme.spacing(2),
        marginBottom: theme.spacing(1),
        paddingTop: theme.spacing(1.5),
        borderTop: `1px solid ${theme.palette.divider}`,
      },
      advancedSettingsToggleNoBorder: {
        marginTop: 0,
        paddingTop: 0,
        borderTop: 'none',
      },
      optionalFieldsSection: {
        marginTop: theme.spacing(1),
        paddingLeft: theme.spacing(3), // Indent to align with Advanced Settings text
      },
    }),
    [theme]
  );
};
