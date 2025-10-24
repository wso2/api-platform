import React from 'react';
import { StyledSimpleSelect } from './SimpleSelect.styled';
import {
  Box,
  CircularProgress,
  FormHelperText,
  type PopoverOrigin,
  Select as MUISelect,
  type SelectChangeEvent,
} from '@mui/material';
import clsx from 'clsx';
import ChevronDown from '../../Icons/generated/ChevronDown';
import Info from '../../Icons/generated/Info';

export type sizeVariant = 'small' | 'medium';

export interface SimpleSelectProps {
  children?: React.ReactNode;
  className?: string;
  onClick?: (event: React.MouseEvent<HTMLDivElement>) => void;
  disabled?: boolean;
  testId: string;
  onChange: (event: SelectChangeEvent<unknown>) => void;
  value: unknown;
  error?: boolean;
  size?: sizeVariant;
  helperText?: React.ReactNode;
  renderValue?: (value: unknown) => React.ReactNode;
  resetStyles?: boolean;
  anchorOrigin?: PopoverOrigin;
  transformOrigin?: PopoverOrigin;
  isLoading?: boolean;
  isScrollable?: boolean;
  startAdornment?: React.ReactNode;
  isSearchBarItem?: boolean;
}

/**
 * SimpleSelect component
 * @component
 */
export const SimpleSelect = React.forwardRef<HTMLDivElement, SimpleSelectProps>(
  (
    {
      children,
      className,
      onClick,
      disabled = false,
      startAdornment,
      isLoading,
      testId,
      value,
      onChange,
      size,
      anchorOrigin,
      transformOrigin,
      renderValue,
      error,
      helperText,
      isScrollable,
      isSearchBarItem = false,
      ...props
    },
    ref
  ) => {
    const handleClick = React.useCallback(
      (event: React.MouseEvent<HTMLDivElement>) => {
        if (!disabled && onClick) {
          onClick(event);
        }
      },
      [disabled, onClick]
    );

    const handleChange = React.useCallback(
      (event: SelectChangeEvent<unknown>) => {
        onChange(event);
      },
      [onChange]
    );

    const CircularLoader = () => (
      <Box className="loadingIcon">
        <CircularProgress size={14} />
      </Box>
    );

    return (
      <StyledSimpleSelect
        ref={ref}
        onClick={handleClick}
        disabled={disabled}
        className={clsx({
          simpleSelect: true,
          resetSimpleSelectStyles: props.resetStyles,
        })}
        isSearchBarItem={isSearchBarItem}
        size={size}
        {...props}
      >
        <MUISelect
          startAdornment={startAdornment}
          disabled={disabled || isLoading}
          data-cyid={testId}
          data-testid={testId}
          value={value}
          onChange={handleChange}
          disableUnderline
          IconComponent={isLoading ? CircularLoader : ChevronDown}
          variant="outlined"
          size={size}
          MenuProps={{
            PopoverClasses: {
              paper: `listPaper ${
                isScrollable ? 'scrollableList' : ''
              } ${startAdornment ? 'startAdornmentAlignLeft' : ''}`,
            },
            anchorOrigin,
            transformOrigin,
          }}
          renderValue={renderValue}
          error={error}
          fullWidth
          className={clsx({
            root: true,
            rootSmall: size === 'small',
            rootMedium: size === 'medium',
            icon: true,
            iconSmall: size === 'small',
            iconMedium: size === 'medium',
            outlined: true,
            outlinedSmall: size === 'small',
            outlinedMedium: size === 'medium',
          })}
        >
          {children}
        </MUISelect>
        {helperText && (
          <FormHelperText error={error}>
            <Box display="flex" alignItems="center">
              {error && (
                <Box className="selectInfoIcon">
                  <Info fontSize="inherit" />
                </Box>
              )}
              <Box ml={1}>{helperText}</Box>
            </Box>
          </FormHelperText>
        )}
      </StyledSimpleSelect>
    );
  }
);

SimpleSelect.displayName = 'SimpleSelect';
