import React from 'react';
import {
  CardContent as MuiCardContent,
  styled,
  type SxProps,
  type Theme,
} from '@mui/material';

interface CardContentProps {
  children: React.ReactNode;
  paddingSize?: 'md' | 'lg';
  fullHeight?: boolean;
  testId?: string;
  sx?: SxProps<Theme>;
}

const StyledCardContent = styled(MuiCardContent, {
  shouldForwardProp: (prop) => prop !== 'paddingSize' && prop !== 'fullHeight',
})<{ paddingSize?: 'md' | 'lg'; fullHeight?: boolean }>(
  ({ theme, paddingSize = 'lg', fullHeight = false }) => ({
    padding: paddingSize === 'lg' ? theme.spacing(3) : theme.spacing(2),
    '&:last-child': {
      paddingBottom: paddingSize === 'lg' ? theme.spacing(3) : theme.spacing(2),
    },
    ...(fullHeight && {
      height: '100%',
    }),
  })
);

export const CardContent = ({
  children,
  paddingSize = 'lg',
  fullHeight = false,
  testId,
  sx,
}: CardContentProps) => (
  <StyledCardContent
    paddingSize={paddingSize}
    fullHeight={fullHeight}
    data-cyid={testId ? `${testId}-card-content` : undefined}
    sx={sx}
  >
    {children}
  </StyledCardContent>
);
