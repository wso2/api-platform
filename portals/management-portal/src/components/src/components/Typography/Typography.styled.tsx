import { Typography, type TypographyProps, styled } from '@mui/material';
import type { ComponentType } from 'react';

export interface StyledTypographyProps {
  monospace?: boolean;
}

export const StyledTypography: ComponentType<
  StyledTypographyProps & TypographyProps
> = styled(Typography)<TypographyProps & StyledTypographyProps>(
  ({ monospace }) => ({
    fontFamily: monospace ? 'monospace' : 'inherit',
  })
);
