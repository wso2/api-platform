import { Box, styled, type Theme } from '@mui/material';

export interface StyledPaginationProps {
  theme?: Theme;
  children?: React.ReactNode;
  className?: string;
  testId?: string;
  ref?: React.Ref<HTMLDivElement>;
  onClick?: (event: React.MouseEvent<HTMLDivElement>) => void;
}

export const StyledPagination: React.ComponentType<StyledPaginationProps> =
  styled(Box)(({ theme }) => ({
    dropdown: {
      width: theme.spacing(8),
      marginLeft: theme.spacing(1),
    },
    color: theme.palette.text.primary,
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.0),
  }));

export interface StyledDivProps {
  theme?: Theme;
  className?: string;
  children?: React.ReactNode;
}

export const StyledDiv: React.ComponentType<StyledDivProps> = styled('div')(
  ({ theme }) => ({
    flexShrink: 0,
    marginLeft: theme.spacing(2.5),
    marginRight: theme.spacing(6),
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'flex-end',
    flexGrow: 1,
    gridGap: theme.spacing(0.5),
    color: theme.palette.text.primary,
  })
);
