import {
  styled,
  TableRow as MUITableRow,
  type TableRowProps as MUITableRowProps,
} from '@mui/material';

interface TableRowProps extends MUITableRowProps {
  deletable?: boolean;
  disabled?: boolean;
  noBorderBottom?: boolean;
}

export const StyledTableRow: React.ComponentType<TableRowProps> = styled(
  MUITableRow
)<TableRowProps>(({ theme, disabled, noBorderBottom }) => ({
  opacity: disabled ? 0.7 : 1,
  color: disabled ? theme.palette.text.disabled : theme.palette.text.primary,
  cursor: disabled ? 'not-allowed' : 'pointer',
  pointerEvents: disabled ? 'none' : 'auto',
  ...(noBorderBottom && {
    '& .MuiTableCell-root': {
      borderBottom: 'none',
    },
  }),
  '&:hover': {
    backgroundColor: disabled ? 'transparent' : theme.palette.action.hover,
  },
}));
