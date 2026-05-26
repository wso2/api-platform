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

import { createStyles, makeStyles, Theme } from '@material-ui/core/styles';

const useStyles = makeStyles((theme: Theme) =>
  createStyles({
    topicContainer: {
      width: "95%",
      maxWidth: "95%",
      marginTop: theme.spacing(2),
      marginBottom: theme.spacing(2),
      marginLeft: theme.spacing(1),
      borderBottom: 0,
      boxSizing: 'border-box',
    },
    topicTypeContainer: {
      height: theme.spacing(3),
      display: 'flex',
    },
    typeChipContainer: {
      display: 'flex',
      height: theme.spacing(3),
      marginRight: theme.spacing(1),
      alignItems: 'center',
    },
    endpointContainer: {
      display: 'flex',
      width: "100%",
      maxWidth: "100%",
      padding: theme.spacing(2),
      border: 1,
      borderBottom: `1px solid ${theme.palette.grey[200]}`,
      gap: theme.spacing(1),
      alignItems: 'center',
      boxSizing: 'border-box',
      overflow: 'hidden',
    },
    textInput: {
      flex: 1,
      minWidth: 200,
      backgroundColor: '#ffffff',
      height: 40,
      display: 'flex',
      alignItems: 'center',
      borderRadius: theme.spacing(0.5),
      border: `1px solid ${theme.palette.grey[200]}`,
      boxShadow: '0 1px 3px rgba(0, 0, 0, 0.08)',
      padding: theme.spacing(0, 1),
      '&:hover': {
        borderColor: theme.palette.primary.light,
      },
    },
    payloadText: {
      borderTop: 1,
      height: theme.spacing(5),
      marginLeft: theme.spacing(1),
    },
    tabs: {
      padding: theme.spacing(1),
      paddingLeft: theme.spacing(1),
      width: "100%",
      maxWidth: "100%",
      boxSizing: 'border-box',
    },
    tabPanelContent: {
      width: "100%",
      maxWidth: "100%",
      boxSizing: 'border-box',
    },
    payloadContainer: {
      display: 'flex',
      flexDirection: 'column',
      minHeight: 170,
      height: 'auto',
      borderRadius: 'none',
      marginBottom: theme.spacing(2),
      width: "100%",
      maxWidth: "100%",
      boxSizing: 'border-box',
      position: 'relative',
    },
    sendIcon: {
      marginLeft: theme.spacing(0),
      marginBottom: theme.spacing(2),
      width: '100%',
      maxWidth: '100%',
      boxSizing: 'border-box',
    },
    parameterContainerWrapper: {
      width: '100%',
      maxWidth: '100%',
      height: theme.spacing(27.5),
      overflow: 'scroll',
      boxSizing: 'border-box',
    },
    label: {
      color: theme.palette.secondary.dark,
      fontWeight: 400,
      minWidth: theme.spacing(12.5),
      marginTop: '10px',
    },
    textArea: {
      borderRadius: 5,
      '& .MuiOutlinedInput-root': {
        borderRadius: 5,
        padding: 0,
        '& .MuiOutlinedInput-input': {
          backgroundColor: theme.palette.common.white,
          borderRadius: 5,
          boxShadow:
            'inset 0 0 0 1px rgba(85,103,213,0.4), inset 0 1px 1px 0 rgba(0,0,0,0.07), 0 0 0 0 rgba(50,50,77,0.07)',
          padding: theme.spacing(2.0375),
          '&:active, &:focus': {
            borderRadius: 5,
            borderColor: 'transparent',
            boxShadow:
              'inset 0 0 0 1px rgba(85,103,213,0.4), inset 0 1px 1px 0 rgba(0,0,0,0.07), 0 0 0 0 rgba(50,50,77,0.07)',
          },
          overflowY: 'auto',
          '&::-webkit-scrollbar': {
            width: 4,
          },
          '&::-webkit-scrollbar-thumb': {
            background: '#e6e7ec',
            borderRadius: 2,
          },
          '&::-webkit-scrollbar-track': {
            backgroundColor: 'transparent',
          },
          '&::-webkit-scrollbar-thumb:hover': {
            background: '#eee',
          },
        },
      },
    },
    parameterContainer: {
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
      gap: theme.spacing(1),
      width: '100%',
      maxWidth: '100%',
      height: theme.spacing(22),
      borderRadius: 'none',
      overflow: 'scroll',
      boxSizing: 'border-box',
    },
    paramTextInput: {
      size: 'small',
      width: '100%',
      maxWidth: '100%',
    },
    apiTokenTextInput: {
      size: 'small',
      width: "100%",
      maxWidth: "100%",
    },
    callbackURLTextInput: {
      size: 'small',
      width: "100%",
      maxWidth: "100%",
    },
    executeButton: {
      marginLeft: theme.spacing(0),
      marginBottom: theme.spacing(1),
      width: '100%',
      maxWidth: '100%',
      boxSizing: 'border-box',
    },
    outputConsoleContainer: {
      display: 'flex',
      justifyContent: 'space-between',
      padding: theme.spacing(1),
      paddingLeft: theme.spacing(2),
      paddingRight: theme.spacing(2),
      alignItems: 'center',
      borderTop: `1px solid ${theme.palette.grey[200]}`,
      width: '100%',
      maxWidth: '100%',
      boxSizing: 'border-box',
    },
    outputContainer: {
      height: theme.spacing(20),
      overflow: 'scroll',
      borderTop: `1px solid ${theme.palette.grey[200]}`,
      borderBottomRightRadius: theme.spacing(1),
      borderBottomLeftRadius: theme.spacing(1),
      width: '100%',
      maxWidth: '100%',
      boxSizing: 'border-box',
      paddingLeft: theme.spacing(1),
      paddingRight: theme.spacing(1),
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
    logColor5: {
      color: '#f8c2c2',
    },
    editorContainer: {
      display: 'flex',
      width: '100%',
      height: 170,
      position: 'relative',
      '& .react-monaco-editor-container': {
        width: '100%',
        height: '100%',
        '& .monaco-editor': {
          '& .monaco-scrollable-element': {
            left: '0 !important',
          },
        },
      },
    },
    copyButton: {
      marginRight: theme.spacing(1),
    },
  })
);

export default useStyles;
