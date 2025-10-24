import React from 'react';
import { StyledCardButton } from './CardButton.styled';
import { Box } from '@mui/material';

export interface CardButtonProps {
  icon: React.ReactNode;
  text: React.ReactNode;
  active?: boolean;
  error?: boolean;
  onClick?: () => void;
  testId: string;
  size?: 'small' | 'medium' | 'large';
  fullHeight?: boolean;
  disabled?: boolean;
  endIcon?: React.ReactNode;
}

/**
 * CardButton component
 * @component
 */
export const CardButton = React.forwardRef<HTMLDivElement, CardButtonProps>(
  (
    {
      icon,
      fullHeight = false,
      active,
      text,
      error,
      testId = false,
      onClick,
      size = 'large',
      disabled,
      endIcon,
      ...rest
    },
    _ref
  ) => {
    return (
      <StyledCardButton
        onClick={onClick}
        disabled={disabled}
        variant="text"
        fullWidth
        size={size}
        data-button-root-active={active}
        data-button-root-error={error}
        data-button-root-full-height={fullHeight}
        startIcon={icon}
        data-button-label-size={size}
        data-cyid={`${testId}-card-button`}
        disableRipple
        disableFocusRipple
        disableElevation
        disableTouchRipple
        {...rest}
      >
        <Box className="buttonLabelText">{text}</Box>
        <Box onClick={onClick} className="endIcon">
          {endIcon}
        </Box>
      </StyledCardButton>
    );
  }
);

CardButton.displayName = 'CardButton';
