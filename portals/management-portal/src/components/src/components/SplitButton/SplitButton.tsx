import React from 'react';
import { StyledSplitButton } from './SplitButton.styled';
import {
  Button,
  ButtonGroup,
  ClickAwayListener,
  Grow,
  Paper,
  Popper,
  Box,
} from '@mui/material';
import ChevronDown from '../../Icons/generated/ChevronDown';

export type colorVariant =
  | 'inherit'
  | 'primary'
  | 'secondary'
  | 'success'
  | 'error'
  | 'info'
  | 'warning';

export type buttonVariant = 'text' | 'outlined' | 'contained';

export type sizeVariant = 'small' | 'medium' | 'large';

export interface SplitButtonProps {
  children?: React.ReactNode;
  className?: string;
  onClick?: (event: React.MouseEvent<HTMLElement>) => void;
  disabled?: boolean;
  label?: string;
  selectedValue: string;
  open: boolean;
  setOpen: React.Dispatch<React.SetStateAction<boolean>>;
  startIcon?: React.ReactNode;
  color?: colorVariant;
  variant?: buttonVariant;
  size?: sizeVariant;
  testId?: string;
  fullWidth?: boolean;
  sx?: React.CSSProperties;
}

/**
 * SplitButton component
 * @component
 */
export const SplitButton = React.forwardRef<HTMLDivElement, SplitButtonProps>(
  (
    {
      children,
      className,
      onClick,
      disabled = false,
      label,
      selectedValue,
      open,
      setOpen,
      startIcon,
      variant = 'contained',
      color = 'primary',
      size,
      testId,
      fullWidth = false,
      ...props
    },
    ref
  ) => {
    const handleClick = React.useCallback(
      (event: React.MouseEvent<HTMLDivElement>) => {
        if (!disabled && onClick) {
          onClick(event);
        }
      },
      [disabled, onClick]
    );

    const anchorRef = React.useRef<HTMLDivElement>(null);

    const handleToggle = () => {
      setOpen((prevOpen) => !prevOpen);
    };

    const handleClose = (event: MouseEvent | TouchEvent) => {
      if (
        anchorRef.current &&
        anchorRef.current.contains(event.target as HTMLElement)
      ) {
        return;
      }
      setOpen(false);
    };

    return (
      <StyledSplitButton
        ref={ref}
        className={className}
        onClick={handleClick}
        disabled={disabled}
        {...props}
      >
        <ButtonGroup
          ref={anchorRef}
          aria-label="split button"
          variant={variant}
          color={color}
          size={size}
          disabled={disabled}
          data-testid={`${testId}-split`}
          disableFocusRipple
          disableRipple
          disableElevation
          fullWidth={fullWidth}
        >
          <Button onClick={onClick} startIcon={startIcon}>
            {label && <Box>{label}:&nbsp;</Box>}
            {selectedValue}
          </Button>
          <Button
            aria-controls={open ? 'split-button-menu' : undefined}
            aria-expanded={open ? 'true' : undefined}
            aria-label="select merge strategy"
            aria-haspopup="menu"
            onClick={handleToggle}
            data-testid={`${testId}-split-toggle-button`}
          >
            <ChevronDown fontSize="inherit" />
          </Button>
        </ButtonGroup>

        <Popper
          open={open}
          anchorEl={anchorRef.current}
          role={undefined}
          transition
          placement="bottom-end"
          style={{
            width: anchorRef.current
              ? anchorRef.current.offsetWidth
              : 'initial',
          }}
        >
          {({ TransitionProps, placement }) => (
            <Grow
              {...TransitionProps}
              style={{
                transformOrigin:
                  placement === 'bottom' ? 'right top' : 'right bottom',
              }}
            >
              <Paper>
                <ClickAwayListener onClickAway={handleClose}>
                  <Box>{children}</Box>
                </ClickAwayListener>
              </Paper>
            </Grow>
          )}
        </Popper>
      </StyledSplitButton>
    );
  }
);

SplitButton.displayName = 'SplitButton';
