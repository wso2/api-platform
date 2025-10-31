/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
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

import {
  createStyles,
  makeStyles,
  Theme,
  alpha,
} from '@material-ui/core/styles';

export default makeStyles<Theme>((theme: Theme) =>
  createStyles({
    disableUserSelect: {
      userSelect: 'none',
    },
    '@global': {
      '@keyframes refreshRotation': {
        from: {
          transform: 'rotate(0deg)',
        },
        to: {
          transform: 'rotate(360deg)',
        },
      },
    },
    highlightedLogLine: {
      backgroundColor: '#D8E1E8',
    },
    selectedLogLine: {
      backgroundColor: alpha('#ccd1f2', 1),
    },
    traceIdAvailableLogLine: {
      backgroundColor: "#E5E4E2",
      cursor: 'point',
    },
    whiteBackground: {
      backgroundColor: theme.palette.common.white,
    },
    logLineWrapper: { display: 'flex' },
    expandedLogKey: {
      width: theme.spacing(35),
      paddingLeft: theme.spacing(0.5),
    },
    defaultLogLine: {
      fontFamily: 'Droid Sans Mono',
      minWidth: '100%',
      paddingLeft: theme.spacing(0.4),
      fontSize: theme.spacing(1.375),
      transitionDuration: '0.25s',
      transitionProperty: 'background-color',
      overflow: 'visible',
      minHeight: theme.spacing(3),
      display: 'inline-flex',
      diplayDirection: 'row',
      '& $actionContainer': {
        visibility: 'hidden',
      },
      '&:hover': {
        background: alpha("#E5E4E2", 1),
      },
    },
    '@keyframes animFade': {
      from: {
        opacity: 0,
      },
      to: { opacity: 1 },
    },
    toolTipParent: {
      display: 'inline-flex',
      lineHeight: theme.spacing(0.25),
      '& $toolTipChild': {
        visibility: 'hidden',
      },
      '&:hover': {
        '& $toolTipChild': {
          visibility: 'visible',
          animation: '$animFade 250ms 1',
        },
      },
    },
    toolTipChild: {
      top: theme.spacing(-4),
      padding: theme.spacing(0.5),
      color: theme.palette.primary.main,
      display: 'flex',
      background: theme.palette.common.white,
      boxShadow: `0 1px 3px rgba(0, 0, 0, 0.19)`,
      userSelect: 'none',
      borderRadius: theme.spacing(0.625),
      position: 'absolute',
      cursor: 'pointer',
      '&:hover': {
        backgroundColor: theme.palette.common.white,
      },
    },
    logEntryActions: {
      position: 'relative',
      display: 'flex',
      paddingRight: theme.spacing(1.25),
    },
    actionContainer: {
      top: theme.spacing(-6),
      padding: theme.spacing(0.5),
      display: 'flex',
      width: 'max-content',
      background: theme.palette.common.white,
      position: 'absolute',
      cursor: 'pointer',
      border: `1px solid ${theme.palette.grey[100]}`,
      borderColor: theme.palette.grey[100],
      borderRadius: theme.spacing(0.625),
      backgroundColor: theme.palette.common.white,
      boxShadow: `0 1px 3px rgba(0, 0, 0, 0.19)`,
    },
    actionButton: {
      display: 'flex',
      alignItems: 'center',
      padding: `${theme.spacing(0.5)}px ${theme.spacing(1)}px`,
      gap: theme.spacing(0.5),
      height: theme.spacing(3),
      backgroundColor: alpha(theme.palette.common.white, 0),
      flex: 'none',
      border: theme.palette.grey[100],
      cursor: 'pointer',
      '&:hover': {
        '& $actionIcon': {
          color: theme.palette.primary.light,
        },
        '& $actionLabel': {
          color: theme.palette.primary.light,
        },
      },
    },
    actionIcon: {
      width: theme.spacing(1.625),
      height: theme.spacing(1.625),
      color: theme.palette.primary.main,
    },
    actionLabel: {
      fontFamily: theme.typography.fontFamily,
      fontWeight: 400,
      fontSize: theme.spacing(1.625),
      lineHeight: theme.spacing(0.23),
      color: theme.palette.primary.main,
    },
    errorLogs: {
      color: '#EA4C4D',
      lineHeight: theme.spacing(0.25),
    },
    infoLogs: {
      color: '#53C08A',
      lineHeight: theme.spacing(0.25),
    },
    warningLogs: {
      color: '#FF9D52',
      lineHeight: theme.spacing(0.25),
    },
    logOther: {
      color: theme.palette.secondary.dark,
      lineHeight: theme.spacing(0.25),
    },
    logContextKeyValue: {
      color: '#0095ff',
    },
    logMsg: {
      lineHeight: theme.spacing(0.25),
      display: 'inline',
      overflow: 'visible',
      whiteSpace: 'nowrap',
      '&:hover': {
        '& $actionContainer': {
          visibility: 'visible',
          animation: '$animFade 250ms 1',
          backgroundColor: theme.palette.common.white,
        },
      },
    },
    infoSection: {
      display: 'inline',
      whiteSpace: 'nowrap',
    },
    arrow: {
      color: theme.palette.primary.main,
      fontSize: theme.spacing(1),
      lineHeight: theme.spacing(0.25),
      padding: theme.spacing(0.5),
      cursor: 'pointer',
    },
    logContextDivider: {
      color: theme.palette.divider,
    },
    lastLogTimeStamp: {
      borderBottom: `1px dashed ${theme.palette.primary.main}`,
    },
    // Note: Added important to avoid overriding by virtualize lib
    // Note: Added type casting to 'auto !important' as 'auto' to avoid type error
    logsList: {
      overflowX: 'auto !important' as 'auto',
    },
    expandedLogRow: {
      width: 'fit-content',
      borderBottom: `1px solid ${"#E5E4E2"}`,
      padding: theme.spacing(0.5, 0),
      display: 'flex',
      backgroundColor: theme.palette.common.white,
    },
    tableContainer: {
      display: 'flex',
      flexDirection: 'column',
      paddingLeft: theme.spacing(8),
      overflowX: 'visible',
      whiteSpace: 'nowrap',
    },
    logsListContainer: {
      width: '100%',
      height: '100%',
      paddingLeft: theme.spacing(2),
      backgroundColor: theme.palette.common.white,
    },
    logColor1: {
      color: '#745BCC',
    },
    logColor2: {
      color: '#2366CC',
    },
    logColor3: {
      color: '#2366CC',
    },
    logColor4: {
      color: '#0095ff',
    },
    alertMetric: {
      filter: 'hue-rotate(45deg)',
    },
  })
);
