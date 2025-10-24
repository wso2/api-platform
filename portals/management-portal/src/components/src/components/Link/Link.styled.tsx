import { type LinkProps, styled, Link as MuiLink } from '@mui/material';
import type { ComponentType } from 'react';

export interface StyledLinkProps extends LinkProps {
  disabled?: boolean;
  testId: string;
}

export const StyledLink: ComponentType<StyledLinkProps> = styled(
  MuiLink
)<StyledLinkProps>(({ disabled }) => ({
  opacity: disabled ? 0.4 : 1,
  cursor: disabled ? 'default' : 'pointer',
  backgroundColor: 'transparent',
  pointerEvents: disabled ? 'none' : 'auto',
}));
