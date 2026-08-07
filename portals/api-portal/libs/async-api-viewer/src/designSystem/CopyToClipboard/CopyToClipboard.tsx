/* eslint-disable react/jsx-props-no-spreading */
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
import React from 'react';
import { Box, InputAdornment } from '@material-ui/core';
import clsx from 'clsx';
import CopyIconButton from '../CopyIconButton/CopyIconButton';
import TextInput from '../TextInput/TextInput';
import useTextFiledStyles from '../TextInput/TextInput.styles';
import { ITextInputProps } from '../TextInput/TextInput.types';
import useStyles from './CopyToClipboard.styles';

interface CopyToClipboardProps extends ITextInputProps {
  testId: string;
  component?: 'textInput' | 'card';
  isMultiline?: boolean;
  endAdornment?: React.ReactNode;
}

const CopyToClipboard = (props: CopyToClipboardProps) => {
  const classes = useStyles();
  const textFiledClasses = useTextFiledStyles(props);
  const {
    size,
    value,
    component = 'textInput',
    endAdornment,
    testId,
    isMultiline,
    ...rest
  } = props;
  const fieldValue = value as string;

  return (
    <Box
      className={classes.copyToClipboard}
      data-cyid={`${testId}-copy-to-clipboard`}
    >
      {component === 'textInput' && (
        <Box className={classes.copyToClipboardInputWrap}>
          <TextInput
            value={value}
            readOnly
            className={textFiledClasses.copyToClipboardInput}
            multiline={isMultiline}
            {...rest}
            size={size}
            testId={testId}
            endAdornment={
              <InputAdornment position="end">{endAdornment}</InputAdornment>
            }
          />
        </Box>
      )}
      {component === 'card' && (
        <Box className={classes.copyToClipboardCard}>{value}</Box>
      )}
      <Box
        className={clsx({
          [classes.copyIconWrap]: true,
          [classes.copyIconWrapCard]: component === 'card',
        })}
      >
        <CopyIconButton
          value={fieldValue}
          size={size === 'small' ? 'tiny' : 'small'}
          testId="copy-to-clipboard"
        />
      </Box>
    </Box>
  );
};

export default CopyToClipboard;
