import React from 'react';
import { StyledNoDataMessage } from './NoDataMessage.styled';
import { FormattedMessage } from 'react-intl';
import { Box, Typography } from '@mui/material';
import NoData from '../../Images/generated/NoData';

export type sizeVariant = 'sm' | 'md' | 'lg';

export interface NoDataMessageProps {
  message?: React.ReactNode;
  size?: sizeVariant;
  testId?: string;
  className?: string;
}

/**
 * NoDataMessage component
 * @component
 */
export const NoDataMessage = React.forwardRef<
  HTMLDivElement,
  NoDataMessageProps
>(({ message, size = 'md', testId, className, ...props }, ref) => {
  return (
    <StyledNoDataMessage
      ref={ref}
      data-noData-container="true"
      data-noData-size={size}
      data-cyid={`${testId}-no-data-message`}
      className={className}
      {...props}
    >
      <Box data-noData-icon-wrap="true" data-noData-icon-size={size}>
        <NoData />
      </Box>
      <Box data-noData-message-wrap="true" data-noData-message-size={size}>
        <Typography
          className="noDataMessage"
          variant={
            size === 'lg' ? 'body1' : size === 'md' ? 'body2' : 'caption'
          }
        >
          {message || (
            <FormattedMessage
              id="modules.cioDashboard.NoDataMessage.label"
              defaultMessage="No data available"
            />
          )}
        </Typography>
      </Box>
    </StyledNoDataMessage>
  );
});

NoDataMessage.displayName = 'NoDataMessage';
