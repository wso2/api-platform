import React from 'react';
import { StyledButtonContainer } from './ButtonContainer.styled';

export interface ButtonContainerProps {
  children?: React.ReactNode;
  className?: string;
  onClick?: (event: React.MouseEvent) => void;
  disabled?: boolean;
  align?: 'left' | 'center' | 'right' | 'space-between';
  marginTop?: 'sm' | 'md' | 'lg';
  testId: string;
}

export const ButtonContainer = React.forwardRef<
  HTMLDivElement,
  ButtonContainerProps
>(({ children, className, onClick, disabled = false, ...props }, ref) => {
  return (
    <StyledButtonContainer
      ref={ref}
      className={className}
      onClick={disabled ? undefined : onClick}
      disabled={disabled}
      {...props}
    >
      {children}
    </StyledButtonContainer>
  );
});

ButtonContainer.displayName = 'ButtonContainer';
