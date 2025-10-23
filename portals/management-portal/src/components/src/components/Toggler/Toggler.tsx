import React from 'react';
import { StyledToggler } from './Toggler.styled';

export type colorVariant = 'primary' | 'default'; // Updated to match styled component
export type sizeVariant = 'small' | 'medium';

export interface TogglerProps {
  className?: string;
  onClick?: (event: React.MouseEvent) => void;
  disabled?: boolean;
  size?: sizeVariant;
  checked?: boolean;
  color?: colorVariant;
  sx?: React.CSSProperties;
  testId?: string; // Added missing testId prop
  [key: string]: any;
}

/**
 * Toggler component
 * @component
 */
export const Toggler = React.forwardRef<HTMLButtonElement, TogglerProps>(
  (
    {
      children,
      className,
      onClick,
      disabled = false,
      color = 'default', // Set default to 'default'
      testId,
      ...props
    },
    ref
  ) => {
    const handleChange = (event: React.ChangeEvent<HTMLInputElement>) => {
      if (disabled) return;
      // Convert ChangeEvent to MouseEvent for onClick handler
      const mouseEvent =
        event as unknown as React.MouseEvent<HTMLButtonElement>;
      onClick?.(mouseEvent);
    };

    return (
      <StyledToggler
        ref={ref}
        size={props.size || 'medium'}
        className={className}
        onChange={handleChange} // Use onChange instead of onClick for Switch
        disabled={disabled}
        checked={props.checked}
        color={color}
        disableRipple={true}
        disableTouchRipple={true}
        disableFocusRipple={true}
        data-testid={testId}
        {...props}
      />
    );
  }
);

Toggler.displayName = 'Toggler';
