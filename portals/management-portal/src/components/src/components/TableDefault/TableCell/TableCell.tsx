import { StyledTableCell } from './TableCell.styled';
import { type TableCellProps as MUITableCellProps } from '@mui/material';

export interface TableCellProps extends MUITableCellProps {
  children?: React.ReactNode;
}

export const TableCell: React.FC<TableCellProps> = (props) => {
  return <StyledTableCell {...props}>{props.children}</StyledTableCell>;
};

TableCell.displayName = 'TableCell';
