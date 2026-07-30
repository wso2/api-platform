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

import { createStyles, makeStyles, Theme } from '@material-ui/core';

const useStyles = makeStyles((theme: Theme) =>
  createStyles({
    copyToClipboard: {
      display: 'flex',
      width: '100%',
      position: 'relative',
    },
    copyToClipboardInputWrap: {
      flexGrow: 1,
    },
    copyToClipboardCard: {
      padding: theme.spacing(2),
      borderRadius: theme.spacing(1),
      flexGrow: 1,
    },
    copyIconWrap: {
      display: 'flex',
      alignItems: 'center',
      position: 'absolute',
      bottom: theme.spacing(0.625),
      right: theme.spacing(0.5),
    },
    copyIconWrapCard: {
      top: theme.spacing(1.25),
      right: theme.spacing(1),
    },
  })
);

export default useStyles;
