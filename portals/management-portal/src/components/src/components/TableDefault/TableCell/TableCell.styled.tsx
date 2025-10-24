import {
  styled,
  type TableCellProps,
  TableCell as MUITableCell,
} from '@mui/material';

export const StyledTableCell: React.ComponentType<TableCellProps> = styled(
  MUITableCell
)(({ theme }) => ({
  padding: theme.spacing(1.5, 2),
}));
