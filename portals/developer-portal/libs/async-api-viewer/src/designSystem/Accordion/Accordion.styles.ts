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
import { createStyles, makeStyles, Theme } from '@material-ui/core/styles';

const useStyles = makeStyles((theme: Theme) =>
  createStyles({
    accordion: {
      '&$accordionBordered': {
        border: `1px solid ${theme.palette.grey[100]}`,
        marginBottom: theme.spacing(1),
        borderRadius: 8,
        '&.Mui-expanded': {
          marginBottom: theme.spacing(1),
        },
        '&:last-child': {
          borderBottomColor: theme.palette.grey[100],
          marginBottom: 0,
        },
      },
    },
    accordionBordered: {
      '& $accordionSummaryBackgroundFilled': {
        borderRadius: 8,
      },
    },
    nestedAccordionContent: {
      width: '100%',
      padding: theme.spacing(1, 3),
    },
    accordionSummaryRoot: {
      minHeight: 'initial',
      padding: theme.spacing(0.5, 2),
      borderBottom: '1px solid transparent',
      '&$accordionSummaryBackgroundFilled': {
        background: "#E5E4E2",
      },
      '&.Mui-expanded': {
        minHeight: 'initial',
        borderBottomColor: theme.palette.grey[100],
        borderBottomLeftRadius: 0,
        borderBottomRightRadius: 0,
      },
      '& .MuiIconButton-label': {
        fontSize: theme.spacing(1.5),
        color: theme.palette.common.black,
      },
      '& .MuiAccordionSummary-content': {
        margin: 0,
        flexGrow: 1,
        overflow: 'hidden',
        '&.Mui-expanded': {
          margin: 0,
        },
      },
      '&$accordionSummaryNoPaddingRoot': {
        padding: theme.spacing(0.5, 0),
      },
    },
    accordionSummaryNoPaddingRoot: {},
    accordionSummaryBackgroundFilled: {},
  })
);
export default useStyles;
