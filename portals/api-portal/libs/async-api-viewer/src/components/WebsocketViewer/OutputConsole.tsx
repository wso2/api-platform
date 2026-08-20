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

import React from 'react';
import { Box, Typography } from '@material-ui/core';
import copy from 'clipboard-copy';

import useStyles from './style';
import { Button } from '../../designSystem/Button';
import { CopyrightOutlined, Delete } from '@material-ui/icons';

interface OutputConsoleProps {
  messages: Record<any, string>[];
  clearLogs: () => void;
  isDisabled: boolean;
}

const OutputConsole = (props: OutputConsoleProps) => {
  const classes = useStyles();
  const { messages, clearLogs, isDisabled } = props;

  const handleCopyClick = () => {
    const allMessagesString = messages
      .map((msg) => `${msg.timestamp} ${msg.message}`)
      .join('\n');
    copy(allMessagesString);
  };

  return (
    <Box className={classes.outputConsoleContainer}>
      <Typography variant="body1">
        Output
      </Typography>
      <Box>
        <Button
          testId="copy-output"
          variant="link"
          disabled={isDisabled}
          onClick={handleCopyClick}
          startIcon={<CopyrightOutlined />}
          color="primary"
          size="small"
          className={classes.copyButton}
        >
          Copy Output
        </Button>
        <Button
          testId="clear-output"
          variant="link"
          onClick={clearLogs}
          disabled={isDisabled}
          startIcon={<Delete />}
          color="error"
          size="small"
        >
          Clear
        </Button>
      </Box>
    </Box>
  );
};
export default OutputConsole;
