import React, { type JSX, useEffect, useMemo, useState } from 'react';
import { StyledDataTable } from './DataTable.styled';
import { Box, CircularProgress } from '@mui/material';
import {
  TableBody,
  TableCell,
  TableContainer,
  TableDefault,
  TableHead,
  TableRow,
  TableSortLabel,
} from '../TableDefault';
import { Pagination } from '../Pagination';

export type DataTableColumn<T> = {
  title: string;
  field: string;
  render?: (rowData: T, isHover: boolean) => JSX.Element | null;
  customFilterAndSearch?: (term: string, rowData: T) => boolean;
  width?: string;
  align?: 'left' | 'right' | 'center';
  headerStyle?: React.CSSProperties; // TODO ignored for now
};

export interface DataTableProps<T> {
  enableFrontendSearch?: boolean;
  searchQuery: string;
  isLoading: boolean;
  testId: string;
  columns: DataTableColumn<T>[];
  data: T[];
  totalRows?: number;
  getRowId(rowData: T): string;
  onRowClick?: (row: any) => void;
}

type OrderState = {
  orderBy: string;
  order: 'desc' | 'asc';
};

function RenderRow<T>(props: {
  rowData: T;
  columns: DataTableProps<T>['columns'];
  onRowClick: () => void;
}) {
  const { rowData, columns, onRowClick } = props;
  const [isHover, setIsHover] = useState(false);
  return (
    <TableRow onClick={onRowClick}>
      <div
        onMouseEnter={() => setIsHover(true)}
        onMouseLeave={() => setIsHover(false)}
        style={{ display: 'contents' }}
      >
        {columns.map((col) => {
          const content = col.render
            ? col.render(rowData, isHover)
            : rowData[col.field as keyof T];
          return (
            <TableCell key={col.field}>
              <Box>{content as React.ReactNode}</Box>
            </TableCell>
          );
        })}
      </div>
    </TableRow>
  );
}

function useSortData<T>(
  columns: DataTableColumn<T>[],
  data: T[],
  searchQuery: string,
  enableFrontendSearch: boolean
) {
  const filteredData = useMemo(() => {
    if (!searchQuery || !enableFrontendSearch) return data;
    return data.filter((item) =>
      columns.some((col) => {
        if (col.customFilterAndSearch) {
          return col.customFilterAndSearch(searchQuery, item);
        }
        const val = item[col.field as keyof T];
        if (typeof val === 'string') {
          return val.toLowerCase().includes(searchQuery.toLowerCase());
        }
        return false;
      })
    );
  }, [searchQuery, data]);
  const [sortParams, setSortParams] = useState<OrderState | null>(null);
  const handlerSort = (field: string) => {
    const isAsc = sortParams?.orderBy === field && sortParams?.order === 'asc';
    setSortParams({
      orderBy: field,
      order: isAsc ? 'desc' : 'asc',
    });
  };
  const sortedData = useMemo(() => {
    if (!sortParams) return filteredData;
    return [...filteredData].sort((a, b) => {
      const aVal = a[sortParams.orderBy as keyof T];
      const bVal = b[sortParams.orderBy as keyof T];
      const order = sortParams.order === 'asc' ? 1 : -1;
      if (typeof aVal === 'number' && typeof bVal === 'number') {
        return (aVal - bVal) * order;
      }
      if (typeof aVal === 'string' && typeof bVal === 'string') {
        return aVal.localeCompare(bVal) * order;
      }
      return 0;
    });
  }, [filteredData, sortParams]);
  return { sortParams, handlerSort, sortedData };
}

/**
 * DataTable component
 * @component
 */
export const DataTable = <T,>(
  props: DataTableProps<T> & { ref?: React.Ref<HTMLDivElement> }
) => {
  const {
    enableFrontendSearch = true,
    searchQuery,
    isLoading,
    testId,
    columns,
    data,
    totalRows,
    onRowClick,
    getRowId,
    ...restProps
  } = props;

  const { sortParams, handlerSort, sortedData } = useSortData(
    columns,
    data,
    searchQuery,
    enableFrontendSearch
  );

  const [page, setPage] = React.useState(0);
  const [originPage, setOriginPage] = React.useState(0);
  const [rowsPerPage, setRowsPerPage] = React.useState(5);

  useEffect(() => {
    if (searchQuery && page !== 0) {
      setOriginPage(page);
      setPage(0);
    } else if (!searchQuery && page !== originPage) {
      setPage(originPage);
    }
  }, [searchQuery]);

  const pageData = useMemo(
    () =>
      sortedData.slice(page * rowsPerPage, page * rowsPerPage + rowsPerPage),
    [sortedData, page, rowsPerPage]
  );

  if (isLoading)
    return (
      <Box className={'loaderWrapper'}>
        <CircularProgress />
      </Box>
    );

  return (
    <StyledDataTable ref={props.ref} {...restProps}>
      <TableContainer>
        <TableDefault variant="default" testId={testId}>
          <TableHead data-cyid={`${testId}-table-columns`}>
            <TableRow>
              {columns.map((col) => {
                const sortDirection =
                  sortParams?.orderBy === col.field
                    ? sortParams?.order
                    : undefined;
                return (
                  <TableCell
                    key={col.field}
                    sortDirection={sortDirection}
                    width={col.width}
                  >
                    <TableSortLabel
                      active={sortParams?.orderBy === col.field}
                      direction={sortDirection}
                      onClick={() => handlerSort(col.field)}
                      data-alignment={col.align}
                    >
                      {col.title}
                      {sortParams?.orderBy === col.field ? (
                        <span className="visually-hidden">
                          {sortParams.order === 'desc'
                            ? 'sorted descending'
                            : 'sorted ascending'}
                        </span>
                      ) : null}
                    </TableSortLabel>
                  </TableCell>
                );
              })}
            </TableRow>
          </TableHead>
          <TableBody data-cyid={`${testId}-table-rows`}>
            {pageData.length === 0 && (
              <TableRow>
                <TableCell
                  colSpan={columns.length}
                  className="noRecordsTextRow"
                >
                  No records to display
                </TableCell>
              </TableRow>
            )}
            {pageData.map((item) => (
              <RenderRow
                key={getRowId(item)}
                rowData={item}
                columns={columns}
                onRowClick={() => onRowClick?.(item)}
              />
            ))}
          </TableBody>
        </TableDefault>
      </TableContainer>
      <Box display="flex" mb={2} py={1} alignItems="center">
        <Box className="tablePagination">
          <Pagination
            rowsPerPageOptions={[5, 10, 15, 20, 25, 50]}
            count={totalRows ?? sortedData.length}
            rowsPerPage={rowsPerPage}
            page={page}
            onPageChange={(_, newPage) => setPage(newPage)}
            onRowsPerPageChange={(v) => setRowsPerPage(Number(v))}
            rowsPerPageLabel="Items per page"
            testId="items-per-page"
          />
        </Box>
      </Box>
    </StyledDataTable>
  );
};

DataTable.displayName = 'DataTable';
