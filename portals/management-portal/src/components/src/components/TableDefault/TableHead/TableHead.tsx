import { StyledTableHead } from './TableHead.styled';

export interface TableHeadProps {
  children?: React.ReactNode;
}

export const TableHead: React.FC<TableHeadProps> = (props) => {
  return <StyledTableHead {...props}>{props.children}</StyledTableHead>;
};

TableHead.displayName = 'TableHead';
