/* eslint-disable react/jsx-props-no-spreading */
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

import { forwardRef, ForwardRefRenderFunction } from 'react';
import {
  Box,
  InputBase,
  FormHelperText,
  Typography,
  CircularProgress,
} from '@material-ui/core';
import clsx from 'clsx';
import Tooltip from '../Tooltip/Tooltip';
import useTextFiledStyles from './TextInput.styles';
import { ITextInputProps } from './TextInput.types';
import { Info, QuestionAnswer } from '@material-ui/icons';

const TextInput: ForwardRefRenderFunction<unknown, ITextInputProps> = (
  props,
  ref
) => {
  const classes = useTextFiledStyles(props);
  const {
    label,
    width,
    readOnly,
    error,
    helperText,
    rows,
    multiline,
    optional,
    loading,
    tooltip,
    tooltipPlacement = 'top',
    inputTooltip,
    size = 'medium',
    info,
    actions,
    testId,
    variant = 'default',
    rounded = false,
    resizeIndicator = true,
    ...rest
  } = props;

  const toolTip = tooltip && (
    <Tooltip
      darken
      title={tooltip}
      disabled={!tooltip}
      placement={tooltipPlacement}
    >
      <Box className={classes.tooltipIcon}>
        <Box className={classes.textInputInfoIcon}>
          <QuestionAnswer fontSize="inherit" />
        </Box>
      </Box>
    </Tooltip>
  );
  return (
    <Box width={width}>
      {(label || toolTip || info || optional || actions) && (
        <Box className={classes.formLabel}>
          {label && (
            <Typography component="h6" variant="body1">
              {label}
            </Typography>
          )}
          {info && <Box className={classes.formLabelInfo}>{info}</Box>}
          {toolTip && <Box className={classes.formLabelTooltip}>{toolTip}</Box>}
          {optional && (
            <Typography variant="body2" className={classes.formOptional}>
              (Optional)
            </Typography>
          )}
          {actions && <Box className={classes.formLabelAction}>{actions}</Box>}
        </Box>
      )}
      <Tooltip
        darken
        heading={inputTooltip}
        disabled={!inputTooltip}
        placement={tooltipPlacement}
      >
        <Box className={classes.inputGroup}>
          <InputBase
            ref={ref}
            classes={{
              root: clsx({
                [classes.root]: true,
                [classes.rootSmall]: size === 'small',
                [classes.rootLarge]: size === 'large',
                [classes.readOnlyDefault]: readOnly && variant === 'default',
                [classes.readOnlyPlain]: readOnly && variant === 'plain',
                [classes.multiline]: multiline,
                [classes.multilineReadonly]: multiline && readOnly,
                [classes.multilineResizeIndicator]:
                  multiline && !resizeIndicator,
                [classes.rounded]: rounded,
              }),
              focused: classes.focused,
              error: classes.error,
              inputMultiline: classes.textarea,
              input: clsx({
                [classes.textInput]: true,
                [classes.textInputLarge]: size === 'large',
              }),
              disabled: classes.textInputDisabled,
              adornedEnd: clsx({
                [classes.inputAdornedEnd]: true,
                [classes.inputAdornedEndAlignTop]: multiline,
              }),
            }}
            readOnly={readOnly}
            {...rest}
            error={error}
            rows={rows}
            multiline={multiline}
            data-cyid={testId}
          />
        </Box>
      </Tooltip>
      {error && helperText && (
        <FormHelperText error={error}>
          <Box display="flex" alignItems="center">
            <Box className={classes.textInputInfoIcon}>
              <Info fontSize="inherit" />
            </Box>
            <Box ml={1}>{helperText}</Box>
          </Box>
        </FormHelperText>
      )}
      {loading && helperText && (
        <FormHelperText>
          <Box display="flex" alignItems="center">
            <CircularProgress size={12} />
            <Box ml={1}>{helperText}</Box>
          </Box>
        </FormHelperText>
      )}
    </Box>
  );
};

export default forwardRef(TextInput);
