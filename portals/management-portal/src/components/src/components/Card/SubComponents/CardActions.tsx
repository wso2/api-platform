import React from 'react';
import {
  CardActions as MuiCardActions,
  styled,
  type SxProps,
  type Theme,
} from '@mui/material';

interface CardActionsProps {
  children: React.ReactNode;
  testId: string;
  sx?: SxProps<Theme>;
}

const StyledCardActions = styled(MuiCardActions)(({ theme }) => ({
  padding: theme.spacing(1),
  '&:last-child': {
    paddingBottom: theme.spacing(1),
  },
  display: 'flex',
  gap: theme.spacing(1),
  paddingTop: theme.spacing(3),
  borderTop: `1px solid ${theme.palette.grey[100]}`,
}));

export const CardActions = ({
  children,
  testId,
  sx,
  ...rest
}: CardActionsProps) => (
  <StyledCardActions data-cyid={`${testId}-card-actions`} sx={sx} {...rest}>
    {children}
  </StyledCardActions>
);
