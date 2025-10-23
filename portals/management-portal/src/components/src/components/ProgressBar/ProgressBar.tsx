import React from 'react';
import { StyledProgressBar } from './ProgressBar.styled';

export type ProgressBarVariant =
  | 'determinate'
  | 'indeterminate'
  | 'buffer'
  | 'query';

export type ProgressBarColor = 'primary' | 'secondary' | 'inherit';

export type ProgressBarSize = 'small' | 'medium' | 'large';

export interface ProgressBarProps {
  children?: React.ReactNode;
  className?: string;
  onClick?: (event: React.MouseEvent) => void;
  disabled?: boolean;
  variant?: ProgressBarVariant;
  color?: ProgressBarColor;
  value?: number;
  valueBuffer?: number;
  size?: ProgressBarSize;
  sx?: React.CSSProperties;
  [key: string]: any;
}

/**
 * ProgressBar component
 * @component
 */
export const ProgressBar = React.forwardRef<HTMLDivElement, ProgressBarProps>(
  (
    {
      children,
      className,
      onClick,
      size = 'small',
      disabled = false,
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

    return (
      <StyledProgressBar
        ref={ref}
        className={className}
        color={props.color || 'primary'}
        variant={props.variant || 'indeterminate'}
        onClick={handleClick}
        disabled={disabled}
        size={size}
        {...props}
      />
    );
  }
);

ProgressBar.displayName = 'ProgressBar';
