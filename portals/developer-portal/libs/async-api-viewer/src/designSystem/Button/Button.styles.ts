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

import { alpha, createStyles, makeStyles, Theme } from '@material-ui/core';

export const useButtonStyles = makeStyles((theme: Theme) =>
  createStyles({
    commons: {
      boxShadow: `0 1px 2px  ${alpha(theme.palette.common.black, 0.15)}`,
      borderRadius: 5,
      color: theme.palette.common.white,
      padding: theme.spacing(0.875, 2),
      gap: theme.spacing(1),
      fontWeight: 400,
      fontSize: theme.spacing(1.625),
      lineHeight: `${theme.spacing(3)}px`,
      '&$disabled': {
        color: theme.palette.common.white,
      },
      '& $startIcon': {
        '& *:first-child': {
          fontSize: theme.spacing(2),
        },
        '&$startIconSmall': {
          '& *:first-child': {
            fontSize: theme.spacing(1.75),
          },
        },
        '&$startIconTiny': {
          '& *:first-child': {
            fontSize: theme.spacing(1.5),
          },
        },
      },

      '& $endIcon': {
        '& *:first-child': {
          fontSize: theme.spacing(2),
        },
        '&$endIconSmall': {
          '& *:first-child': {
            fontSize: theme.spacing(1.75),
          },
        },
        '&$endIconTiny': {
          '& *:first-child': {
            fontSize: theme.spacing(1.5),
          },
        },
      },
    },
    contained: {
      border: `1px solid transparent`,
      '&:hover': {
        boxShadow: `0 1px 3px ${alpha(theme.palette.common.black, 0.1)}`,
      },
      '&:focus': {
        boxShadow: `0 1px 6px 2px ${alpha(theme.palette.common.black, 0.1)}`,
      },
    },
    outlined: {
      backgroundColor: 'transparent',
      boxShadow: `0 1px 2px ${alpha(theme.palette.common.black, 0.05)}`,
      '&:hover, &:focus': {
        backgroundColor: 'transparent',
        boxShadow: `0 1px 6px 2px ${alpha(theme.palette.common.black, 0.1)}`,
      },
    },
    fullWidth: { width: '100%' },
    text: {
      backgroundColor: 'transparent',
      border: 'none',
      boxShadow: 'none',
    },
    subtle: {
      border: `1px solid ${theme.palette.grey[100]}`,
      boxShadow: `0 1px 3px ${alpha(theme.palette.common.black, 0.05)}`,
      backgroundColor: "#E5E4E2",
      '&:hover': {
        backgroundColor: "#E5E4E2",
        boxShadow: `0 1px 3px ${alpha(theme.palette.common.black, 0.1)}`,
      },
      '&:focus': {
        backgroundColor: "#E5E4E2",
        boxShadow: 'none',
      },
    },
    link: {
      borderColor: 'transparent',
      boxShadow: 'none',
      paddingLeft: 0,
      paddingRight: 0,
      minWidth: 'initial',
      backgroundColor: 'transparent',
      '& $startIcon': {
        marginLeft: 0,
      },
      '& $endIcon': {
        marginRight: 0,
      },
      '&:hover': {
        opacity: 0.6,
      },
      '&:hover, &:focus': {
        backgroundColor: 'transparent',
        boxShadow: 'none',
      },
    },
    primaryContained: {
      backgroundColor: theme.palette.primary.main,
      borderColor: theme.palette.primary.main,
      '&:hover': {
        backgroundColor: theme.palette.primary.dark,
        borderColor: theme.palette.primary.dark,
      },
    },
    primaryText: {
      color: theme.palette.primary.main,
      '&$disabled': {
        color: theme.palette.primary.main,
      },
    },
    primaryOutlined: {
      color: theme.palette.primary.main,
      border: `1px solid ${theme.palette.primary.main}`,
      boxShadow: `0 1px 2px  ${alpha(theme.palette.common.black, 0.05)}`,
      '&$disabled': {
        color: theme.palette.primary.main,
      },
    },
    primarySubtle: {
      color: theme.palette.primary.main,
      '&$disabled': {
        color: theme.palette.primary.main,
      },
    },
    primaryLink: {
      color: theme.palette.primary.main,
      borderColor: 'transparent',
      boxShadow: 'none',
      '&$disabled': {
        color: theme.palette.primary.main,
        borderColor: 'transparent',
        boxShadow: 'none',
      },
    },
    secondaryContained: {
      backgroundColor: "#E5E4E2",
      color: theme.palette.common.black,
      border: `1px solid ${theme.palette.grey[100]}`,
      boxShadow: `0 1px 2px ${alpha(theme.palette.common.black, 0.05)}`,
      '&:hover': {
        backgroundColor: "#E5E4E2",
        color: theme.palette.common.black,
      },
      '&$disabled': {
        color: theme.palette.common.black,
      },
    },
    secondaryText: {
      color: theme.palette.common.black,
      '&$disabled': {
        color: theme.palette.common.black,
      },
    },
    secondaryOutlined: {
      color: theme.palette.secondary.main,
      border: `1px solid ${theme.palette.secondary.main}`,
      '&:hover': {
        borderColor: theme.palette.secondary.dark,
      },
      '&$disabled': {
        color: theme.palette.secondary.main,
      },
    },
    secondarySubtle: {
      color: theme.palette.common.black,
      '&$disabled': {
        color: theme.palette.common.black,
      },
    },
    secondaryLink: {
      color: theme.palette.common.black,
      borderColor: 'transparent',
      boxShadow: 'none',
      '&$disabled': {
        color: theme.palette.common.black,
        borderColor: 'transparent',
        boxShadow: 'none',
      },
    },
    errorContained: {
      backgroundColor: theme.palette.error.main,
      borderColor: theme.palette.error.main,
      '&:hover': {
        backgroundColor: theme.palette.error.dark,
        borderColor: theme.palette.error.dark,
      },
    },
    errorOutlined: {
      color: theme.palette.error.main,
      border: `1px solid ${theme.palette.error.main}`,
      '&:hover': {
        borderColor: theme.palette.error.dark,
      },
      '&$disabled': {
        color: theme.palette.error.main,
      },
    },
    errorText: {
      color: theme.palette.error.main,
      '&$disabled': {
        color: theme.palette.error.main,
      },
    },
    errorSubtle: {
      color: theme.palette.error.main,
      '&$disabled': {
        color: theme.palette.error.main,
      },
    },
    errorLink: {
      color: theme.palette.error.main,
      borderColor: 'transparent',
      boxShadow: 'none',
      '&$disabled': {
        color: theme.palette.error.main,
        borderColor: 'transparent',
        boxShadow: 'none',
      },
    },
    successContained: {
      backgroundColor: theme.palette.success.main,
      borderColor: theme.palette.success.main,
      '&:hover': {
        backgroundColor: theme.palette.success.dark,
        borderColor: theme.palette.success.dark,
      },
    },
    successOutlined: {
      color: theme.palette.success.main,
      border: `1px solid ${theme.palette.success.main}`,
      '&:hover': {
        borderColor: theme.palette.success.dark,
      },
      '&$disabled': {
        color: theme.palette.success.main,
      },
    },
    successText: {
      color: theme.palette.success.main,
      '&$disabled': {
        color: theme.palette.success.main,
      },
    },
    successSubtle: {
      color: theme.palette.success.main,
      '&$disabled': {
        color: theme.palette.success.main,
      },
    },
    successLink: {
      color: theme.palette.success.main,
      borderColor: 'transparent',
      boxShadow: 'none',
      '&$disabled': {
        color: theme.palette.success.main,
        borderColor: 'transparent',
        boxShadow: 'none',
      },
    },
    warningContained: {
      backgroundColor: theme.palette.warning.main,
      borderColor: theme.palette.warning.main,
      '&:hover': {
        backgroundColor: theme.palette.warning.dark,
        borderColor: theme.palette.warning.dark,
      },
    },
    warningOutlined: {
      color: theme.palette.warning.main,
      border: `1px solid ${theme.palette.warning.main}`,
      '&:hover': {
        borderColor: theme.palette.warning.dark,
      },
      '&$disabled': {
        color: theme.palette.warning.main,
      },
    },
    warningText: {
      color: theme.palette.warning.main,
      '&$disabled': {
        color: theme.palette.warning.main,
      },
    },
    warningSubtle: {
      color: theme.palette.warning.main,
      '&$disabled': {
        color: theme.palette.warning.main,
      },
    },
    warningLink: {
      color: theme.palette.warning.main,
      borderColor: 'transparent',
      boxShadow: 'none',
      '&$disabled': {
        color: theme.palette.warning.main,
        borderColor: 'transparent',
        boxShadow: 'none',
      },
    },
    pill: {
      borderRadius: theme.spacing(3.125),
    },
    small: {
      padding: theme.spacing(0.375, 2),
      gap: theme.spacing(0.75),
    },
    smallPill: {},
    smallLink: {
      padding: theme.spacing(0.375, 0),
    },
    tiny: {
      padding: theme.spacing(0, 1.5),
      gap: theme.spacing(0.5),
    },
    tinyLink: {
      padding: theme.spacing(0),
    },
    tinyPill: {},
    disabled: {
      opacity: 0.5,
      cursor: 'default',
      pointerEvents: 'none',
      color: theme.palette.common.white,
      '&:hover': {
        textDecoration: 'none',
      },
    },
    startIcon: {},
    startIconSmall: {},
    startIconTiny: {},
    endIcon: {},
    endIconSmall: {},
    endIconTiny: {},
  })
);

export default useButtonStyles;
