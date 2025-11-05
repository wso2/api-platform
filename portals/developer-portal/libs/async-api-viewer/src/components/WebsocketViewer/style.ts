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
      width: "90%",
      marginTop: theme.spacing(2),
      marginBottom: theme.spacing(2),
      marginLeft: theme.spacing(1),
      borderBottom: 0,
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
      padding: theme.spacing(1, 1),
      border: 1,
      borderBottom: `1px solid ${theme.palette.grey[200]}`,
      gap: theme.spacing(1),
      marginRight: theme.spacing(1),
    },
    textInput: {
      width: "88%",
      backgroundColor: '#f5f5f5',
    },
    payloadText: {
      borderTop: 1,
      height: theme.spacing(5),
      marginLeft: theme.spacing(1),
    },
    tabs: {
      padding: theme.spacing(1, 1.5),
      marginLeft: theme.spacing(1),
    },
    payloadContainer: {
      display: 'flex',
      flexDirection: 'column',
      height: theme.spacing(15),
      borderRadius: 'none',
      marginBottom: theme.spacing(2),
      marginLeft: theme.spacing(1),
    },
    sendIcon: {
      marginLeft: theme.spacing(0),
      marginBottom: theme.spacing(2),
    },
    parameterContainerWrapper: {
      width: '100%',
      height: theme.spacing(27.5),
      overflow: 'scroll',
    },
    parameterContainer: {
      display: 'grid',
      gridTemplateColumns: 'repeat(4, 1fr)',
      gap: theme.spacing(1),
      width: '100%',
      height: theme.spacing(22),
      borderRadius: 'none',
      overflow: 'scroll',
    },
    paramTextInput: {
      size: 'small',
      width: theme.spacing(30),
    },
    apiTokenTextInput: {
      size: 'small',
      width: "80%",
    },
    executeButton: {
      marginLeft: theme.spacing(0),
      marginBottom: theme.spacing(1),
    },
    outputConsoleContainer: {
      display: 'flex',
      justifyContent: 'space-between',
      padding: theme.spacing(1, 2),
      alignItems: 'center',
      borderTop: `1px solid ${theme.palette.grey[200]}`,
    },
    outputContainer: {
      height: theme.spacing(20),
      overflow: 'scroll',
      borderTop: `1px solid ${theme.palette.grey[200]}`,
      borderBottomRightRadius: theme.spacing(1),
      borderBottomLeftRadius: theme.spacing(1),
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
      overflowY: 'auto',
      position: 'absolute',
      '& .react-monaco-editor-container': {
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
