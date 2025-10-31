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
import { createStyles, makeStyles, Theme } from '@material-ui/core/styles';

const useStyles = makeStyles((theme: Theme) =>
  createStyles({
    tabCard: {
      padding: 0,
    },
    fullHeight: {
      display: 'flex',
      flexDirection: 'column',
      height: '100%',
      width: '100%',
    },
    tabHeading: {
      borderBottom: `1px solid ${theme.palette.grey[100]}`,
    },
    tabBody: {
      flexGrow: 1,
      overflow: 'auto',
      display: 'flex',
      flexDirection: 'column',
    },
    tabs: {
      minHeight: 'initial',
      '& .MuiTabs-flexContainer': {
        gap: theme.spacing(3),
      },
    },
    tab: {
      padding: 0,
      minWidth: 'auto',
      textTransform: 'initial',
      minHeight: 'initial',
      paddingTop: theme.spacing(1),
      paddingBottom: theme.spacing(1),
      color: theme.palette.secondary.dark,
      fontWeight: 400,
      '&.Mui-selected': {
        color: theme.palette.primary.main,
        fontWeight: 600,
      },
      '&.Mui-disabled': {
        color: theme.palette.grey[200],
      },
    },
    tabSmall: {
      fontSize: theme.spacing(1.5),
      paddingTop: theme.spacing(0.5),
      paddingBottom: theme.spacing(0.5),
    },
    tabMedium: {
      fontSize: theme.spacing(1.75),
      paddingTop: theme.spacing(1),
      paddingBottom: theme.spacing(1),
    },
    tabLarge: {
      fontSize: theme.spacing(2),
      paddingBottom: theme.spacing(1.25),
    },
    iconLeft: {
      '& .MuiTab-wrapper': {
        flexDirection: 'row',
        '& .MuiSvgIcon-root': {
          marginBottom: 0,
          marginRight: theme.spacing(1),
        },
      },
    },
    iconRight: {
      '& .MuiTab-wrapper': {
        flexDirection: 'row-reverse',
        '& .MuiSvgIcon-root': {
          marginBottom: 0,
          marginLeft: theme.spacing(1),
        },
      },
    },
    iconBottom: {
      '& .MuiTab-wrapper': {
        flexDirection: 'column-reverse',
        '& .MuiSvgIcon-root': {
          marginBottom: 0,
          marginTop: theme.spacing(1),
        },
      },
    },
    iconTop: {},
    tabBox: {
      display: 'flex',
      alignItems: 'center',
    },
    tabAction: {
      marginLeft: theme.spacing(1),
    },
    tabActionsWrap: {
      display: 'flex',
      width: '100%',
      justifyContent: 'space-between',
    },
    tabActionsWrapCenter: {
      justifyContent: 'center',
    },
    tabActions: {
      marginLeft: theme.spacing(2),
      paddingBottom: theme.spacing(1),
    },
    tabPanel: {
      paddingTop: theme.spacing(2),
      overflow: 'hidden',
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
    },
    tabPanelHide: {
      display: 'none',
    },
    menuPaper: {
      border: `1px solid ${theme.palette.grey[100]}`,
      borderRadius: 5,
    },
    menuList: {
      paddingTop: 0,
      paddingBottom: 0,
    },
    menuItemRoot: {
      '&:hover': {
        backgroundColor: "#E5E4E2",
      },
    },
    menuItemContent: {
      display: 'flex',
      alignItems: 'center',
      gridGap: theme.spacing(1),
    },
    menuItemIcon: {
      display: 'flex',
      alignItems: 'center',
    },
    menuItemText: {},
  })
);
export default useStyles;
