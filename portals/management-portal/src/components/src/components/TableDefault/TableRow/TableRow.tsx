import { StyledTableRow } from './TableRow.styled';

export interface TableRowProps {
  children?: React.ReactNode;
  deletable?: boolean;
  disabled?: boolean;
  noBorderBottom?: boolean;
  onClick?: (event: React.MouseEvent<HTMLTableRowElement>) => void;
}

export const TableRow: React.FC<TableRowProps> = (props) => {
  return <StyledTableRow {...props}>{props.children}</StyledTableRow>;
};

TableRow.displayName = 'TableRow';
