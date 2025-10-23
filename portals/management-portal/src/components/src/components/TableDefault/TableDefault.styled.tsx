import {
  styled,
  Table as MUITable,
  type TableProps as MUITableProps,
  alpha,
} from '@mui/material';
import type { ComponentType } from 'react';

export interface StyledTableDefaultProps {
  variant: string;
}

export const StyledTable: ComponentType<
  MUITableProps & StyledTableDefaultProps
> = styled(MUITable, {
  shouldForwardProp: (prop) => prop !== 'disabled' && prop !== 'variant',
})<StyledTableDefaultProps>(({ theme, variant }) => ({
  ...(variant === 'dark' && {
    borderCollapse: 'separate',
    borderSpacing: theme.spacing(0, 1),
    '& .MuiTableBody-root': {
      '& .MuiTableRow-root': {
        boxShadow: `0px 2px 2px ${alpha(theme.palette.secondary.main, 0.2)} `,
        borderRadius: theme.spacing(1),
      },
    },
    '& .MuiTableCell-body': {
      backgroundColor: theme.palette.secondary.light,
      borderBottom: 'none',
      padding: theme.spacing(1, 2),
      '&:first-child': {
        borderLeft: '1px solid transparent',
        borderTopLeftRadius: theme.spacing(1),
        borderBottomLeftRadius: theme.spacing(1),
      },
      '&:last-child': {
        borderRight: '1px solid transparent',
        borderTopRightRadius: theme.spacing(1),
        borderBottomRightRadius: theme.spacing(1),
      },
      '&[data-padding="checkbox"]': {
        backgroundColor: 'transparent',
      },
    },
  }),
}));
