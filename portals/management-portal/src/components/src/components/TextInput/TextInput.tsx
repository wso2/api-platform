import React from 'react';
import {
  Box,
  CircularProgress,
  FormHelperText,
  Tooltip,
  type TooltipProps,
  Typography,
} from '@mui/material';
import {
  StyledTextField,
  StyledFormControl,
  HeadingWrapper,
} from './TextInput.styled';
import { QuestionIcon, InfoIcon } from '../../Icons';
import Question from '../../Icons/generated/Question';

type InputPropsJson = Pick<
  React.InputHTMLAttributes<HTMLInputElement>,
  'inputMode' | 'pattern'
>;

export interface TextInputProps {
  label?: string;
  value: string;
  optional?: boolean;
  loading?: boolean;
  tooltip?: React.ReactNode;
  info?: React.ReactNode;
  tooltipPlacement?: TooltipProps['placement'];
  inputTooltip?: React.ReactNode;
  typography?: React.ComponentProps<typeof Typography>['variant'];
  helperText?: string;
  error?: boolean;
  errorMessage?: string;
  testId: string;
  onChange: (text: string) => void;
  disabled?: boolean;
  type?: string;
  readonly?: boolean;
  actions?: React.ReactNode;
  multiline?: boolean;
  rows?: number;
  rounded?: boolean;
  className?: string;
  size?: 'small' | 'medium' | 'large';
  fullWidth?: boolean;
  placeholder?: string;
  endAdornment?: React.ReactNode;
  InputProps?: React.ComponentProps<typeof StyledTextField>['InputProps'];
  inputPropsJson?: InputPropsJson;
  onBlur?: React.FocusEventHandler<HTMLInputElement | HTMLTextAreaElement>;
}

export const TextInput = React.forwardRef<HTMLDivElement, TextInputProps>(
  (
    {
      label,
      tooltip,
      value,
      error,
      errorMessage,
      testId,
      onChange,
      disabled,
      readonly,
      multiline = false,
      className,
      rows,
      optional,
      loading,
      info,
      actions,
      helperText,
      rounded = true,
      size = 'small',
      fullWidth = false,
      type,
      endAdornment,
      onBlur,
      inputPropsJson,
      InputProps,
      ...props
    },
    ref
  ) => {
    const computedError = !!errorMessage || !!error;

    const toolTip = tooltip && (
      <Tooltip title={tooltip} placement={props.tooltipPlacement}>
        <Box className="tooltipIcon">
          <Box className="textInputInfoIcon">
            <Question fontSize="inherit" />
          </Box>
        </Box>
      </Tooltip>
    );

    return (
      <StyledFormControl ref={ref} className={className}>
        {(label || toolTip || info || optional || actions) && (
          <HeadingWrapper>
            <Typography>{label}</Typography>
            {tooltip && (
              <Tooltip title={tooltip} className="formLabelTooltip">
                <QuestionIcon fontSize="inherit" className="tooltipIcon" />
              </Tooltip>
            )}
            {info && <Box className="formLabelInfo">{info}</Box>}
            {optional && (
              <Typography variant="body2" className="formOptional">
                (Optional)
              </Typography>
            )}
            {actions && <Box className="formLabelAction">{actions}</Box>}
          </HeadingWrapper>
        )}
        <StyledTextField
          customSize={size}
          data-cyid={testId}
          variant="outlined"
          multiline={multiline}
          rows={rows}
          type={type}
          value={value}
          onChange={(evt: React.ChangeEvent<HTMLInputElement>) =>
            onChange(evt.target.value)
          }
          disabled={disabled}
          onBlur={onBlur}
          slotProps={{
            htmlInput: inputPropsJson,
            input: {
              readOnly: readonly,
            },
            inputLabel: {
              shrink: false,
            },
          }}
          InputProps={{
            ...InputProps,
            endAdornment: endAdornment ?? InputProps?.endAdornment,
          }}
          error={computedError}
          helperText={
            computedError && errorMessage ? (
              <Box display="flex" alignItems="center" gap={0.5}>
                <InfoIcon fontSize="inherit" />
                {errorMessage}
              </Box>
            ) : (
              helperText
            )
          }
          fullWidth={fullWidth}
          {...props}
        />
        {loading && helperText && (
          <FormHelperText>
            <Box display="flex" alignItems="center">
              <CircularProgress size={12} />
              <Box ml={1}>{helperText}</Box>
            </Box>
          </FormHelperText>
        )}
      </StyledFormControl>
    );
  }
);

TextInput.displayName = 'TextInput';
