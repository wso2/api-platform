import { StyledTableContainer } from './TableContainer.styled';

export interface TableContainerProps {
  children?: React.ReactNode;
}

export const TableContainer: React.FC<TableContainerProps> = (props) => {
  return (
    <StyledTableContainer {...props}>{props.children}</StyledTableContainer>
  );
};

TableContainer.displayName = 'TableContainer';
