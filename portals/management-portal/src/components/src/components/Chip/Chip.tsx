import React from 'react';
import { StyledChip } from './Chip.styled';
import type { SxProps, Theme } from '@mui/material';

export type colorVariant =
  | 'default'
  | 'primary'
  | 'secondary'
  | 'error'
  | 'warning'
  | 'info'
  | 'success';
export type chipVariant = 'filled' | 'outlined';
export type sizeVariant = 'small' | 'medium' | 'large';

export interface ChipProps {
  /** The content to be rendered within the component */
  children?: React.ReactNode;
  /** Additional CSS class names */
  className?: string;
  /** Whether the component is disabled */
  disabled?: boolean;
  //**
  // the label of the component */
  label?: string;
  /**
   * The color of the component
   */
  color?: colorVariant;
  /**
   * The variant of the component
   */
  variant?: chipVariant;
  /**
   * The size of the component
   */
  size?: sizeVariant;
  testId: string;
  /**
   * The sx prop for custom styles
   */
  sx?: SxProps<Theme>;
  /**
   * Additional props for MUI Chip
   */
  [key: string]: any;
}

/**
 * Chip component
 * @component
 */
export const Chip = React.forwardRef<HTMLDivElement, ChipProps>(
  (
    {
      children,
      className,
      disabled = false,
      size = 'medium',
      variant = 'filled',
      color = 'default',
      ...props
    },
    ref
  ) => {
    return (
      <StyledChip
        ref={ref}
        {...props}
        size={size}
        variant={variant === 'filled' ? 'filled' : 'outlined'}
        color={color}
        label={props.label}
        className={className}
        disabled={disabled}
        data-cyid={`${props.testId}-chip`}
      >
        {children}
      </StyledChip>
    );
  }
);

Chip.displayName = 'Chip';
