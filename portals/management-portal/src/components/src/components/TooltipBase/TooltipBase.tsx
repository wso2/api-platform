import React from 'react';
import { StyledTooltipBase } from './TooltipBase.styled';

export interface TooltipBaseProps {
  /** The content to be rendered within the component */
  children?: React.ReactNode;
  /** The tooltip content */
  title?: React.ReactNode;
  /** Additional CSS class names */
  className?: string;
  /** Click event handler */
  onClick?: (event: React.MouseEvent<HTMLDivElement>) => void;
}

/**
 * TooltipBase component
 * @component
 */
export const TooltipBase = React.forwardRef<HTMLDivElement, TooltipBaseProps>(
  ({ children, title, className, onClick, ...props }, ref) => {
    const child = React.isValidElement(children) ? (
      React.cloneElement(children, {
        ...(onClick && { onClick }),
        ...(className && { className }),
        ref,
        ...props,
      } as any)
    ) : (
      <span ref={ref} onClick={onClick} className={className} {...props}>
        {children}
      </span>
    );

    return (
      <StyledTooltipBase title={title || 'Tooltip content'}>
        {child}
      </StyledTooltipBase>
    );
  }
);

TooltipBase.displayName = 'TooltipBase';
