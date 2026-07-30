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
import { createStyles, makeStyles, Theme, alpha } from '@material-ui/core';

const useStyles = makeStyles((theme: Theme) =>
  createStyles({
    root: {
      '& .MuiChip-icon': {
        color: 'inherit',
      },
      '&$small': {
        fontSize: theme.spacing(1.25),
        borderRadius: theme.spacing(0.375),
        lineHeight: 1.2,
        height: theme.spacing(2),
        '& .MuiChip-label': {
          padding: theme.spacing(0, 0.5, 0.125, 0.5),
        },
        '& .MuiChip-icon': {
          marginLeft: theme.spacing(0.5),
          marginRight: 0,
        },
      },
      '&$medium': {
        fontSize: theme.spacing(1.625),
        borderRadius: theme.spacing(0.625),
        lineHeight: 1.23,
        height: theme.spacing(3),
        '& .MuiChip-label': {
          padding: theme.spacing(0.1, 1),
        },
        '& .MuiChip-icon': {
          marginLeft: theme.spacing(1),
          marginRight: 0,
        },
      },
      '&$large': {
        fontSize: theme.spacing(1.625),
        borderRadius: theme.spacing(0.625),
        lineHeight: 1.23,
        '& .MuiChip-label': {
          padding: theme.spacing(1, 1.5),
        },
        '& .MuiChip-icon': {
          marginLeft: theme.spacing(1.5),
          marginRight: 0,
        },
      },
    },
    small: {},
    medium: {},
    large: {},
    contained: {
      '&$info': {
        backgroundColor: '#0095ff',
        color: theme.palette.common.white,
      },
      '&$primary': {
        backgroundColor: theme.palette.primary.main,
        color: theme.palette.common.white,
      },
      '&$secondary': {
        background: "#E5E4E2",
      },
      '&$success': {
        backgroundColor: theme.palette.success.main,
        color: theme.palette.common.white,
      },
      '&$default': {
        backgroundColor: theme.palette.grey[200],
      },
      '&$warning': {
        backgroundColor: theme.palette.warning.dark,
        color: theme.palette.common.white,
      },
      '&$error': {
        backgroundColor: theme.palette.error.main,
        color: theme.palette.common.white,
      },
    },
    outlined: {
      '&$info': {
        color: '#0095ff',
        border: `1px solid ${'0095ff'}`,
        backgroundColor: alpha('#0095ff', 0.1),
      },
      '&$primary': {
        border: `1px solid ${theme.palette.primary.main}`,
        color: theme.palette.primary.main,
        backgroundColor: '#D8E1E8',
      },
      '&$secondary': {
        backgroundColor: theme.palette.common.white,
        border: `1px solid ${theme.palette.grey[200]}`,
      },
      '&$success': {
        border: `1px solid ${theme.palette.success.main}`,
        color: theme.palette.success.main,
        backgroundColor: theme.palette.success.light,
      },
      '&$default': {
        backgroundColor: theme.palette.grey[100],
        border: `1px solid ${theme.palette.grey[200]}`,
      },
      '&$warning': {
        border: `1px solid ${theme.palette.warning.dark}`,
        color: theme.palette.warning.dark,
        backgroundColor: theme.palette.warning.light,
      },
      '&$error': {
        border: `1px solid ${theme.palette.error.main}`,
        color: theme.palette.error.main,
        backgroundColor: theme.palette.error.light,
      },
    },
    info: {},
    primary: {},
    secondary: {},
    success: {},
    default: {},
    warning: {},
    error: {},
  })
);
export default useStyles;
