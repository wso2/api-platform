import React from 'react';
import { StyledCheckbox } from './Checkbox.styled';
import { Checkbox as MUICheckbox } from '@mui/material';

export type CheckboxSize = 'small' | 'medium';

export type CheckboxColor =
  | 'default'
  | 'primary'
  | 'secondary'
  | 'error'
  | 'warning'
  | 'info'
  | 'success';

export interface CheckboxProps {
  children?: React.ReactNode;
  className?: string;
  onClick?: (event: React.MouseEvent) => void;
  disabled?: boolean;
  checked?: boolean;
  indeterminate?: boolean;
  name?: string;
  value?: string;
  size?: CheckboxSize;
  color?: CheckboxColor;
  disableRipple?: boolean;
  sx?: React.CSSProperties;
  [key: string]: any;
}

/**
 * Checkbox component
 * @component
 */
export const Checkbox = React.forwardRef<HTMLDivElement, CheckboxProps>(
  (
    {
      children,
      className,
      onClick,
      disabled = false,
      disableRipple = true,
      ...props
    },
    ref
  ) => {
    return (
      <StyledCheckbox
        ref={ref}
        className={className}
        disabled={disabled}
        {...props}
      >
        <MUICheckbox
          {...props}
          className={className}
          checked={props.checked}
          indeterminate={props.indeterminate}
          disableRipple={disableRipple}
          name={props.name}
          value={props.value}
          size={props.size}
          disabled={disabled}
          onClick={onClick}
          data-cyid={`${props.testId}-check-box`}
          color={props.color}
          sx={props.sx}
        />
        <span>{children}</span>
      </StyledCheckbox>
    );
  }
);

Checkbox.displayName = 'Checkbox';
