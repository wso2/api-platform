/*
 * Copyright (c) 2022, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you’ve
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

import { alpha, makeStyles } from '@material-ui/core';
import { ITextInputProps } from './TextInput.types';

export const useTextFiledStyles = makeStyles((theme) => ({
  root: {
    padding: theme.spacing(0.5, 1.5),
    width: '100%',
    minHeight: theme.spacing(5),
    backgroundColor: theme.palette.common.white,
    border: `1px solid ${theme.palette.grey[100]}`,
    boxShadow: `0 1px 2px -1px ${alpha(
      theme.palette.common.black,
      0.08
    )}, 0 -3px 9px 0 ${alpha(theme.palette.common.black, 0.04)} inset`,
    borderRadius: 5,
    '&$multiline': {
      height: 'auto',
      resize: 'auto',
    },
    '&$multilineReadonly': {
      height: 'auto',
      resize: 'none',
      '& $textarea': {
        height: 'auto',
        resize: 'none',
      },
    },
    '&$multilineResizeIndicator': {
      height: 'auto',
      resize: 'none',
      '& $textarea': {
        height: 'auto',
        resize: 'none',
      },
    },
    '&$rounded': {
      paddingLeft: theme.spacing(2),
    },
    '&:hover': {
      borderColor: '#ccd1f2',
    },
  },
  rootSmall: {
    minHeight: theme.spacing(4),
  },
  rootLarge: {
    minHeight: theme.spacing(7),
    borderRadius: 12,
    padding: theme.spacing(1, 1, 1, 3),
  },
  textInput: (props: ITextInputProps) => ({
    minHeight: theme.spacing(2.5),
    padding: theme.spacing(0.125, 0),
    fontSize: theme.typography[props.typography || 'body1'].fontSize,
    fontWeight: theme.typography[props.typography || 'body1'].fontWeight,
    lineHeight: theme.typography[props.typography || 'body1'].lineHeight,
  }),
  textInputLarge: (props: ITextInputProps) => ({
    fontSize: theme.typography[props.typography || 'overline'].fontSize,
    fontWeight: theme.typography[props.typography || 'overline'].fontWeight,
    lineHeight: theme.typography[props.typography || 'overline'].lineHeight,
  }),
  textInputDisabled: {
    '&:hover': {
      borderColor: theme.palette.grey[100],
    },
  },
  inputAdornedEnd: {
    '& .MuiInputAdornment-root': {
      marginRight: theme.spacing(-1),
    },
  },
  inputAdornedEndAlignTop: {
    '& .MuiInputAdornment-root': {
      alignSelf: 'flex-end',
      height: 'auto',
      marginBottom: theme.spacing(0.25),
    },
  },
  multiline: {},
  multilineReadonly: {},
  multilineResizeIndicator: {},
  rounded: {
    borderRadius: theme.spacing(2.5),
  },
  focused: {
    borderColor: theme.palette.primary.light,
    borderWidth: 1,
    boxShadow: `0 -3px 9px 0 ${alpha(
      theme.palette.common.black,
      0.04
    )} inset, 0 0 0 2px #D8E1E8`,
    '&:hover': {
      borderColor: theme.palette.primary.light,
    },
  },
  error: {
    background: theme.palette.error.light,
    borderColor: theme.palette.error.main,
    boxShadow: `0 0 0 1px ${theme.palette.error.light}, inset 0 2px 2px ${alpha(
      theme.palette.error.light,
      0.07
    )}`,
    '&:hover': {
      borderColor: theme.palette.error.main,
    },
  },
  readOnlyDefault: {
    boxShadow: `0 0 0 1px ${alpha(
      theme.palette.common.black,
      0.05
    )}, inset 0 2px 2px ${alpha(theme.palette.common.black, 0.05)}`,
    border: 'none',
    backgroundColor: "#E5E4E2",
  },
  readOnlyPlain: {
    boxShadow: 'none',
    border: 'none',
    backgroundColor: theme.palette.common.white,
    paddingLeft: 0,
    paddingRight: 0,
  },
  formLabel: {
    display: 'flex',
    width: '100%',
    alignItems: 'center',
    marginBottom: theme.spacing(0.5),
  },
  formLabelAction: {
    marginLeft: 'auto',
    display: 'flex',
    alignItems: 'center',
  },
  formLabelInfo: {
    marginLeft: theme.spacing(1),
    display: 'flex',
    alignItems: 'center',
  },
  formOptional: {
    color: theme.palette.grey[200],
    fontSize: theme.spacing(1.4),
    marginLeft: theme.spacing(1),
  },
  formLabelTooltip: {
    marginLeft: theme.spacing(1),
    display: 'flex',
    alignItems: 'center',
  },
  inputGroup: {
    position: 'relative',
  },
  tooltipIcon: {
    display: 'flex',
    alignItems: 'center',
    color: theme.palette.secondary.main,
    cursor: 'help',
    fontSize: theme.spacing(1.75),
  },
  textarea: {
    resize: 'both',
  },
  copyToClipboardInput: {
    backgroundColor: 'transparent',
    border: 'none',
    boxShadow: 'none',
    paddingRight: theme.spacing(5),
    minHeight: theme.spacing(4.5),
    fontSize: theme.typography.body2.fontSize,
    fontFamily: '"Roboto Mono", "Monaco", "Consolas", monospace',
    '&:hover': {
      borderColor: 'transparent',
    },
  },
  textInputInfoIcon: {
    display: 'flex',
    alignItems: 'center',
    fontSize: theme.spacing(1.75),
  },
}));

export default useTextFiledStyles;
