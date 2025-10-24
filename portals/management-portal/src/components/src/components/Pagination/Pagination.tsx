import React from 'react';
import { StyledPagination, StyledDiv } from './Pagination.styled';
import { IconButton } from '../IconButton';
import {
  FirstPage,
  KeyboardArrowLeft,
  KeyboardArrowRight,
  LastPage,
} from '@mui/icons-material';
import { Typography, Box } from '@mui/material';
import { Select } from '../Select';

export interface PaginationProps {
  /** Additional CSS class names */
  className?: string;
  /** Click event handler */
  onClick?: (event: React.MouseEvent<HTMLDivElement>) => void;
  /** Whether the component is disabled */
  disabled?: boolean;
  count?: number;
  rowsPerPageOptions: number[];
  rowsPerPage: number;
  page: number;
  onPageChange: (
    event: React.MouseEvent<HTMLButtonElement> | null,
    newPage: number
  ) => void;
  onRowsPerPageChange: (value: string) => void;
  rowsPerPageLabel?: React.ReactNode;
  testId: string;
  sx?: React.CSSProperties;
}

/**
 * Pagination component
 * @component
 */
export const Pagination = React.forwardRef<HTMLDivElement, PaginationProps>(
  ({ className, onClick, disabled = false, onPageChange, ...props }, ref) => {
    const totalPages = Math.ceil((props.count ?? 0) / props.rowsPerPage);
    const from = props.page * props.rowsPerPage + 1;
    const to = Math.min((props.page + 1) * props.rowsPerPage, props.count ?? 0);
    const displayedRowsLabel = `${from}–${to} of ${props.count ?? 0}`;

    const handleFirstPageButtonClick = (
      event: React.MouseEvent<HTMLButtonElement>
    ) => {
      if (props.page > 0) {
        onPageChange(event, 0);
      }
    };

    const handleBackButtonClick = (
      event: React.MouseEvent<HTMLButtonElement>
    ) => {
      if (props.page > 0) {
        onPageChange(event, props.page - 1);
      }
    };

    const handleNextButtonClick = (
      event: React.MouseEvent<HTMLButtonElement>
    ) => {
      if (props.page < totalPages - 1) {
        onPageChange(event, props.page + 1);
      }
    };

    const handleLastPageButtonClick = (
      event: React.MouseEvent<HTMLButtonElement>
    ) => {
      const lastPage = Math.max(0, totalPages - 1);
      if (props.page < lastPage) {
        onPageChange(event, lastPage);
      }
    };

    const isFirstPage = props.page === 0;
    const isLastPage = props.page >= totalPages - 1;

    return (
      <StyledPagination
        ref={ref}
        className={className}
        onClick={onClick}
        data-cyid={`${props.testId}-pagination`}
      >
        <StyledDiv>
          <IconButton
            onClick={handleFirstPageButtonClick}
            disabled={disabled || isFirstPage}
            disableRipple={true}
            aria-label="first page"
            color="secondary"
            variant="text"
            testId="first-page"
          >
            <FirstPage />
          </IconButton>
          <IconButton
            onClick={handleBackButtonClick}
            disabled={disabled || isFirstPage}
            disableRipple={true}
            aria-label="previous page"
            color="secondary"
            variant="text"
            testId="previous-page"
          >
            <KeyboardArrowLeft />
          </IconButton>
          <Typography>{displayedRowsLabel}</Typography>
          <IconButton
            onClick={handleNextButtonClick}
            disabled={disabled || isLastPage}
            disableRipple={true}
            aria-label="next page"
            color="secondary"
            variant="text"
            testId="next-page"
          >
            <KeyboardArrowRight />
          </IconButton>
          <IconButton
            onClick={handleLastPageButtonClick}
            disabled={disabled || isLastPage}
            disableRipple={true}
            aria-label="last page"
            color="secondary"
            variant="text"
            testId="last-page"
          >
            <LastPage />
          </IconButton>
        </StyledDiv>
        <Typography>{props.rowsPerPageLabel || 'Rows per page'}</Typography>
        <Box>
          <Select
            defaultValue={props.rowsPerPage.toString()}
            getOptionLabel={(option: string) => option}
            onChange={(val: string | null) =>
              val && props.onRowsPerPageChange(val)
            }
            labelId="pagination-dropdown"
            name="pagination-dropdown"
            options={props.rowsPerPageOptions.map((num: number) =>
              num.toString()
            )}
            value={props.rowsPerPage.toString()}
            size="small"
            testId="pagination-dropdown"
          />
        </Box>
      </StyledPagination>
    );
  }
);

Pagination.displayName = 'Pagination';
