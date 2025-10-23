import { StyledTableBody } from './TableBody.styled';

export interface TableBodyProps {
  children?: React.ReactNode;
}

export const TableBody: React.FC<TableBodyProps> = (props) => {
  return <StyledTableBody {...props}>{props.children}</StyledTableBody>;
};

TableBody.displayName = 'TableBody';
