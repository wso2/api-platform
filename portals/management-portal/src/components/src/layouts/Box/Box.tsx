import React from 'react';
import { type BoxProps, StyledBox } from './Box.styled';

/**
 * Box component
 * @component
 */
export const Box = React.forwardRef<HTMLDivElement, BoxProps>(
  ({ children, className, onMouseEnter, onMouseLeave, ...rest }) => {
    return (
      <StyledBox
        className={className}
        onMouseEnter={onMouseEnter}
        onMouseLeave={onMouseLeave}
        {...rest}
      >
        {children}
      </StyledBox>
    );
  }
);

Box.displayName = 'Box';
