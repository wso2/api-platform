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

import { lazy, Suspense } from 'react';
import { Box, CircularProgress } from '@material-ui/core';
import useStyles from './style';
import React from 'react';

const MonacoEditor = lazy(() => import('react-monaco-editor'));

const MONACO_REQUEST_OPTIONS: any = {
  autoIndent: 'full',
  automaticLayout: true,
  contextmenu: true,
  fontFamily: '"Droid Sans Mono", Monaco, monospace',
  hideCursorInOverviewRuler: true,
  matchBrackets: 'always',
  minimap: {
    enabled: false,
  },
  overviewRulerLanes: 0,
  scrollbar: {
    vertical: 'hidden',
    horizontal: 'hidden',
    handleMouseWheel: false,
  },
  lineNumbers: false,
  readOnly: false,
  renderLineHighlight: 'none',
  padding: {
    top: 8,
  },
};

interface PayloadEditorProps {
  fileContent: string;
  language?: string;
  width?: string;
  height?: string;
  setFileContent: (value: string) => void;
}

export function PayloadEditor({
  fileContent,
  language,
  width = '100%',
  height = '70vh',
  setFileContent,
}: PayloadEditorProps) {
  const classes = useStyles();
  return (
    <Box className={classes.editorContainer}>
      <Suspense fallback={<CircularProgress />}>
        <MonacoEditor
          width={width}
          height={height}
          theme="choreoRequestLightTheme"
          language={language}
          options={MONACO_REQUEST_OPTIONS}
          onChange={(content: string) => setFileContent(content)}
          value={fileContent}
        />
      </Suspense>
    </Box>
  );
}
