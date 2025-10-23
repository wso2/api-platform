import React from 'react';
import { StyledTable } from './TableDefault.styled';

export interface TableDefaultProps {
  /** The content to be rendered within the component */
  children?: React.ReactNode;
  /** Additional CSS class names */
  className?: string;
  /** The variant style for the table */
  variant: 'dark' | 'default';
  testId?: string;
}

export const TableDefault = React.forwardRef<
  HTMLTableElement,
  TableDefaultProps
>(({ children, className, variant = 'default', testId = undefined }, ref) => {
  return (
    <StyledTable
      ref={ref}
      className={className}
      variant={variant}
      data-testid={testId}
    >
      {children}
    </StyledTable>
  );
});

TableDefault.displayName = 'TableDefault';
