import React from 'react';
import { StyledSelectMenuItem } from './SelectMenuItem.styled';
import { Box, Typography } from '@mui/material';

export interface SelectMenuItemProps {
  disabled?: boolean;
  testId: string;
  description?: React.ReactNode;
  children?: React.ReactNode;
  value?: number;
}

export const SelectMenuItem = (props: SelectMenuItemProps) => {
  const { disabled, testId, description, children, ...rest } = props;

  return (
    <StyledSelectMenuItem
      testId={testId}
      disabled={disabled}
      data-cyid={`${testId}-select-item`}
      description={description}
      {...rest}
    >
      <Box>
        {children}
        {description && (
          <Typography variant="body2" className="description">
            {description}
          </Typography>
        )}
      </Box>
    </StyledSelectMenuItem>
  );
};
