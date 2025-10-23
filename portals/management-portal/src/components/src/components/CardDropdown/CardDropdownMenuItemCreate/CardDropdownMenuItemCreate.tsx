import { Box } from '@mui/material';
import React from 'react';
import CardDropdownMenuItem from '../CardDropdownMenuItem';
import Add from '../../../Icons/generated/Add';

export interface CardDropdownMenuItemCreateProps {
  createText: string;
  testId: string;
  onClick?: React.MouseEventHandler<HTMLLIElement>;
  disabled?: boolean;
}

export const CardDropdownMenuItemCreate = React.forwardRef<
  HTMLLIElement,
  CardDropdownMenuItemCreateProps
>(({ createText, onClick, disabled = false, testId }) => {
  return (
    <CardDropdownMenuItem
      onClick={onClick}
      data-cyid={`${testId}-menu-action`}
      disabled={disabled}
      sx={(theme) => ({
        color: theme.palette.primary.main,
        alignItems: 'center',
      })}
    >
      <Box
        sx={(theme) => ({
          marginRight: theme.spacing(1),
          fontSize: theme.spacing(1.5),
          alignItems: 'center',
          display: 'flex',
        })}
      >
        <Add fontSize="inherit" />
      </Box>
      <Box>{createText}</Box>
    </CardDropdownMenuItem>
  );
});

CardDropdownMenuItemCreate.displayName = 'CardDropdownMenuItemCreate';
