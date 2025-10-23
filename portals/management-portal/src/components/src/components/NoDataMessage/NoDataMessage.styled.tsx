import { Box, type BoxProps, styled } from '@mui/material';
import type { ComponentType } from 'react';

export interface StyledNoDataMessageProps extends BoxProps {
  // disabled?: boolean;
}

export const StyledNoDataMessage: ComponentType<StyledNoDataMessageProps> =
  styled(Box)<StyledNoDataMessageProps>(({ theme }) => ({
    '&[data-noData-container="true"]': {
      width: '100%',
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
    },
    '&[data-noData-size="sm"]': {
      padding: theme.spacing(1.5, 2),
    },
    '&[data-noData-size="md"]': {
      padding: theme.spacing(2),
    },
    '&[data-noData-size="lg"]': {
      padding: theme.spacing(4),
    },
    '& [data-noData-icon-wrap="true"]': {
      display: 'flex',
      alignItems: 'center',
      marginBottom: theme.spacing(0.5),
      '& svg': {
        width: '100%',
        maxWidth: '100%',
        aspectRatio: '1 / 1',
        height: '100%',
        objectFit: 'contain',
      },
    },
    '& [data-noData-icon-size="sm"]': {
      width: theme.spacing(4),
    },
    '& [data-noData-icon-size="md"]': {
      width: theme.spacing(5),
    },
    '& [data-noData-icon-size="lg"]': {
      width: theme.spacing(6),
    },
    '& [data-noData-message-wrap="true"]': {
      textAlign: 'center',
    },
    '& [data-noData-message-size="sm"] .noDataMessage': {
      fontSize: theme.typography.caption.fontSize,
    },
    '& [data-noData-message-size="md"] .noDataMessage': {
      fontSize: theme.typography.body2.fontSize,
    },
    '& [data-noData-message-size="lg"] .noDataMessage': {
      fontSize: theme.typography.body1.fontSize,
    },
    '& .noDataMessage': {
      color: theme.palette.secondary.main,
      textAlign: 'center',
    },
  }));
