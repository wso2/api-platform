import React from 'react';
import { StyledRadioGroup } from './RadioGroup.styled';

export interface RadioGroupProps {
  /** The content to be rendered within the component */
  children?: React.ReactNode;
  /** Additional CSS class names */
  className?: string;
  /** Click event handler */
  onClick?: (event: React.MouseEvent<HTMLDivElement>) => void;
  /** Whether the component is disabled */
  disabled?: boolean;
  /**
   * If true, the component will be displayed in a horizontal layout
   */
  row?: boolean;
  /**
   * The sx prop for custom styles
   */
  sx?: React.CSSProperties;
  /**
   * Additional props for MUI RadioGroup
   */
  [key: string]: any;
}

/**
 * RadioGroup component
 * @component
 */
export const RadioGroup = React.forwardRef<HTMLDivElement, RadioGroupProps>(
  ({ children, className, onClick, disabled = false, ...props }) => {
    return (
      <StyledRadioGroup
        className={className}
        onClick={disabled ? undefined : onClick}
        disabled={disabled}
        row={props.row}
        {...props}
      >
        {disabled
          ? React.Children.map(children, (child) => {
              if (React.isValidElement(child)) {
                return React.cloneElement(child as React.ReactElement<any>, {
                  disabled: true,
                });
              }
              return child;
            })
          : children}
      </StyledRadioGroup>
    );
  }
);

RadioGroup.displayName = 'RadioGroup';
