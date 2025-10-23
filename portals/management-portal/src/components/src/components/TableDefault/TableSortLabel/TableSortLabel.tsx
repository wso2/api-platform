import { StyledTableSortLabel } from './TableSortLabel.styled';

export interface TableSortLabelProps {
  children?: React.ReactNode;
  direction?: 'asc' | 'desc';
  active?: boolean;
  onClick?: (event: React.MouseEvent<unknown>) => void;
}

export const TableSortLabel: React.FC<TableSortLabelProps> = (props) => {
  return (
    <StyledTableSortLabel {...props} onClick={props.onClick}>
      {props.children}
    </StyledTableSortLabel>
  );
};

TableSortLabel.displayName = 'TableSortLabel';
