/*
 * Copyright (c) 2023, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
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

import { createStyles, makeStyles, Theme } from '@material-ui/core/';

export const useToolTipBaseStyles = makeStyles((theme: Theme) =>
  createStyles({
    infoTooltipDark: {
      color: theme.palette.grey[100],
      backgroundColor: theme.palette.secondary.dark,
      borderRadius: 5,
    },
    infoArrowDark: {
      color: theme.palette.secondary.dark,
    },
    infoTooltipLight: {
      color: theme.palette.secondary.dark,
      backgroundColor: theme.palette.common.white,
      borderRadius: 5,
      maxWidth: theme.spacing(53),
    },
    infoArrowLight: {
      color: theme.palette.common.white,
    },
  })
);

export const useStyles = makeStyles((theme: Theme) =>
  createStyles({
    divider: {
      marginTop: theme.spacing(1),
      marginBottom: theme.spacing(1),
      backgroundColor: theme.palette.grey[100],
    },
    buttonLink: {
      color: theme.palette.primary.main,
      cursor: 'pointer',
      marginTop: theme.spacing(1.5),
      textDecoration: 'none',
    },
    dividerDark: {
      backgroundColor: theme.palette.grey[100],
    },
    exampleContent: {
      fontWeight: 100,
      marginTop: theme.spacing(1),
      marginBottom: theme.spacing(1),
    },
    exampleContentDark: {
      color: theme.palette.grey[100],
    },
    exampleContentLight: {
      color: theme.palette.secondary.dark,
    },
  })
);
