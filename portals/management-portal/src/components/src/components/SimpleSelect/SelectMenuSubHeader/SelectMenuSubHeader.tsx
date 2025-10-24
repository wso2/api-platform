import React from 'react';
import { StyledSelectMenuSubHeader } from './SelectMenuSubHeader.styled';

export interface SelectMenuSubHeaderProps {
  testId: string;
  children?: React.ReactNode;
}
/**
 * SelectMenuSubHeader component
 * @component
 */
export const SelectMenuSubHeader: React.FC<SelectMenuSubHeaderProps> = ({
  testId,
  children,
}) => {
  return (
    <StyledSelectMenuSubHeader
      className="selectMenuSubHeader"
      data-testid={testId}
      testId={testId}
    >
      {children}
    </StyledSelectMenuSubHeader>
  );
};
