/* eslint-disable @typescript-eslint/no-unused-vars */
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
import React, { useState } from 'react';
import { Box, IconButton, Tooltip } from '@material-ui/core';
import copy from 'clipboard-copy';
import ContentCopyOutlined from '@mui/icons-material/ContentCopyOutlined';

interface CopyIconButtonProps {
  testId: string;
  value: string;
  size?: 'small' | 'tiny';
  stopPropagation?: boolean;
}

const CopyIconButton = (props: CopyIconButtonProps) => {
  const { value, testId, size, stopPropagation } = props;
  const fieldValue = value as string;

  const copyText = 'Copy to Clipboard';
  const copiedText = 'Copied';

  const [copyBtnText, setCopyBtnText] = useState(copyText);

  const handleTooltipClose = () => {
    setCopyBtnText(copyText);
  };

  const onClick = (e: React.MouseEvent<HTMLButtonElement, MouseEvent>) => {
    setCopyBtnText(copiedText);
    copy(fieldValue);
    if (stopPropagation) {
      e.stopPropagation();
    }
  };
  return (
    <Tooltip
      title={copyBtnText}
      onClose={handleTooltipClose}
      placement="top-end"
    >
      <Box component="span">
        <IconButton size="small" color="primary" onClick={(e) => onClick(e)}>
          <ContentCopyOutlined />
        </IconButton>
      </Box>
    </Tooltip>
  );
};

export default CopyIconButton;
