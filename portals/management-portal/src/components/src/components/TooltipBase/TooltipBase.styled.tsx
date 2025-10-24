import { styled, Tooltip, type TooltipProps } from '@mui/material';
import type { ComponentType } from 'react';

export interface StyledTooltipBaseProps extends TooltipProps {
  disabled?: boolean;
}

export const StyledTooltipBase: ComponentType<StyledTooltipBaseProps> = styled(
  Tooltip,
  { shouldForwardProp: (prop) => !['disabled'].includes(prop as string) }
)<StyledTooltipBaseProps>(({ theme }) => ({
  '& .MuiTooltip-tooltip': {
    '&.infoTooltipDark': {
      color: theme.palette.grey[300],
      backgroundColor: theme.palette.secondary.dark,
      borderRadius: 5,
    },
    '&.infoTooltipLight': {
      color: theme.palette.secondary.dark,
      backgroundColor: theme.palette.common.white,
      borderRadius: 5,
      maxWidth: theme.spacing(53),
    },
  },
  '& .MuiTooltip-arrow': {
    '&.infoArrowDark': {
      color: theme.palette.secondary.dark,
    },
    '&.infoArrowLight': {
      color: theme.palette.common.white,
    },
  },
}));
