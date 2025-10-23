import styled from '@emotion/styled';
import { Box, type BoxProps } from '@mui/material';
import type { Theme } from '@mui/material/styles';
import React from 'react';

interface StyledButtonContainerProps {
  disabled?: boolean;
  align?: 'left' | 'center' | 'right' | 'space-between';
  marginTop?: 'sm' | 'md' | 'lg';
  theme?: Theme;
}

export const StyledButtonContainer: React.ComponentType<
  BoxProps & StyledButtonContainerProps
> = styled(Box, {
  shouldForwardProp: (prop) =>
    !['disabled', 'align', 'marginTop'].includes(prop as string),
})<StyledButtonContainerProps>(({
  theme,
  disabled,
  align = 'left',
  marginTop,
}) => {
  let justifyContent = 'flex-start';
  if (align === 'center') justifyContent = 'center';
  else if (align === 'right') justifyContent = 'flex-end';
  else if (align === 'space-between') justifyContent = 'space-between';

  let marginTopValue = ''; // Initialize as empty string
  if (marginTop === 'sm') marginTopValue = theme?.spacing(1);
  else if (marginTop === 'md') marginTopValue = theme?.spacing(2);
  else if (marginTop === 'lg') marginTopValue = theme?.spacing(3);

  return {
    display: 'flex',
    justifyContent,
    opacity: disabled ? 0.5 : 1,
    cursor: disabled ? 'not-allowed' : 'default',
    marginTop: marginTopValue || 0, // Fallback to 0 if empty string
    gap: theme?.spacing(1),
    '&:hover': {
      backgroundColor: 'inherit',
      color: 'inherit',
    },
    pointerEvents: disabled ? 'none' : 'auto',
  };
});
