import React from 'react';
import { StyledIconButton } from './IconButton.styled';
import { useTheme } from '@mui/material/styles';

export type iconButtonVariant = 'circular' | 'rounded' | 'square'; // not anymore in mui v7
export type iconButtonColorVariant =
  | 'primary'
  | 'secondary'
  | 'error'
  | 'warning'
  | 'info'
  | 'success';
export type iconButtonSizeVariant = 'tiny' | 'small' | 'medium';
export type edgeVariant = 'start' | 'end' | false;

export interface IconButtonProps {
  /**
   * The content of the component
   */
  children?: React.ReactNode;
  /**
   * Additional className for the component
   */
  className?: string;
  /**
   * Optional click handler
   */
  onClick?: (event: React.MouseEvent) => void;
  /**
   * If true, the component will be disabled
   */
  disabled?: boolean;
  /**
   * edge variant of the icon button
   */
  edge?: edgeVariant;
  /**
   * The color variant of the component
   */
  color?: iconButtonColorVariant;
  /**
   * The size variant of the component
   */
  size?: iconButtonSizeVariant;
  /**
   * disable ripple effect
   */
  disableRipple?: boolean;
  /**
   * disable focus ripple effect
   */
  disableFocusRipple?: boolean;
  /**
   * disable touch ripple effect
   */
  disableTouchRipple?: boolean;
  /**
   * The sx prop for custom styles
   */
  sx?: React.CSSProperties;
  /**
   * Additional props for MUI IconButton
   */
  [key: string]: any;
}

export const IconButton = React.forwardRef<HTMLButtonElement, IconButtonProps>(
  ({ children, ...props }, ref) => (
    <StyledIconButton
      ref={ref}
      theme={useTheme()}
      onClick={props.disabled ? undefined : props.onClick}
      disabled={props.disabled}
      {...props}
    >
      {children}
    </StyledIconButton>
  )
);

IconButton.displayName = 'IconButton';
