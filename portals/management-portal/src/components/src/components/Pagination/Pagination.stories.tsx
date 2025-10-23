import type { Meta, StoryObj } from '@storybook/react';
import { Pagination } from './Pagination';
import { Box } from '@mui/material';
import React from 'react';

const meta: Meta<typeof Pagination> = {
  title: 'Components/Pagination',
  component: Pagination,
  tags: ['autodocs'],
  argTypes: {
    disabled: {
      control: 'boolean',
      description: 'Disables the element',
      table: {
        type: { summary: 'boolean' },
        defaultValue: { summary: 'false' },
      },
    },
    onClick: {
      action: 'clicked',
      description: 'Click event handler',
    },
  },
};

export default meta;
type Story = StoryObj<typeof Pagination>;

const PaginationDemo = () => {
  const [page, setPage] = React.useState(0);
  const [rowsPerPage, setRowsPerPage] = React.useState(10);

  const handlePageChange = (
    _event: React.MouseEvent<HTMLButtonElement> | null,
    newPage: number
  ) => {
    setPage(newPage);
  };

  const handleRowsPerPageChange = (value: string) => {
    setRowsPerPage(parseInt(value, 10));
    setPage(0);
  };

  return (
    <Box display="flex">
      <Pagination
        rowsPerPageOptions={[5, 10, 15, 20]}
        count={25}
        rowsPerPage={rowsPerPage}
        page={page}
        onPageChange={handlePageChange}
        onRowsPerPageChange={handleRowsPerPageChange}
        rowsPerPageLabel="Items per page"
        testId="items-per-page"
      />
    </Box>
  );
};

export const Default: Story = {
  render: () => <PaginationDemo />,
};

// Additional stories with different configurations
export const WithManyItems: Story = {
  render: () => {
    const PaginationManyItems = () => {
      const [page, setPage] = React.useState(0);
      const [rowsPerPage, setRowsPerPage] = React.useState(10);

      const handlePageChange = (
        _event: React.MouseEvent<HTMLButtonElement> | null,
        newPage: number
      ) => {
        setPage(newPage);
      };

      const handleRowsPerPageChange = (value: string) => {
        setRowsPerPage(parseInt(value, 10));
        setPage(0);
      };

      return (
        <Box display="flex">
          <Pagination
            rowsPerPageOptions={[10, 25, 50, 100]}
            count={1000}
            rowsPerPage={rowsPerPage}
            page={page}
            onPageChange={handlePageChange}
            onRowsPerPageChange={handleRowsPerPageChange}
            rowsPerPageLabel="Records per page"
            testId="many-items-pagination"
          />
        </Box>
      );
    };

    return <PaginationManyItems />;
  },
};

export const Disabled: Story = {
  render: () => {
    const DisabledPagination = () => {
      const [page] = React.useState(2);
      const [rowsPerPage] = React.useState(10);

      return (
        <Box display="flex">
          <Pagination
            rowsPerPageOptions={[5, 10, 15, 20]}
            count={50}
            rowsPerPage={rowsPerPage}
            page={page}
            onPageChange={() => {}}
            onRowsPerPageChange={() => {}}
            rowsPerPageLabel="Items per page"
            testId="disabled-pagination"
            disabled
          />
        </Box>
      );
    };

    return <DisabledPagination />;
  },
};
