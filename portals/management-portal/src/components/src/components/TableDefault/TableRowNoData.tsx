import { TableCell } from './TableCell/TableCell';
import { TableRow } from './TableRow/TableRow';

interface TableRowNoDataProps {
  testId: string;
  colSpan?: number;
  message?: React.ReactNode;
}

export const TableRowNoData: React.FC<TableRowNoDataProps> = ({
  // testId,
  colSpan = 1,
  // message = 'No data available',
}) => {
  return (
    <TableRow noBorderBottom>
      <TableCell colSpan={colSpan}>
        {/* <NoDataMessage testId={testId} message={message} /> */}
      </TableCell>
    </TableRow>
  );
};

TableRowNoData.displayName = 'TableRowNoData';
