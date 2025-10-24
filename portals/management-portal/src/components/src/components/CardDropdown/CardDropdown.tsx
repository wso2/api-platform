import React from 'react';
import { StyledCardDropdown } from './CardDropdown.styled';
import { Box, MenuList, Popover, useTheme } from '@mui/material';
import ChevronUp from '../../Icons/generated/ChevronUp';
import ChevronDown from '../../Icons/generated/ChevronDown';

export interface CardDropdownProps {
  icon: React.ReactNode;
  text: React.ReactNode;
  active?: boolean;
  children: React.ReactNode;
  onClick?: React.MouseEventHandler<HTMLDivElement>;
  disabled?: boolean;
  'data-cyid'?: string;
  testId: string;
  size?: 'small' | 'medium' | 'large';
  fullHeight?: boolean;
}

/**
 * CardDropdown component
 * @component
 */
export const CardDropdown = React.forwardRef<HTMLDivElement, CardDropdownProps>(
  (
    {
      children,
      icon,
      text,
      active = false,
      testId,
      size = 'medium',
      fullHeight = false,
      ...props
    },
    _ref
  ) => {
    const [anchorEl, setAnchorEl] = React.useState<HTMLButtonElement | null>(
      null
    );

    const [buttonWidth, setButtonWidth] = React.useState<number>(0);
    const theme = useTheme();
    const buttonRef = React.useRef<HTMLButtonElement | null>(null);

    React.useEffect(() => {
      if (buttonRef.current) {
        const width = buttonRef.current.clientWidth;
        setButtonWidth(width);
      }
    }, []);

    const handleClick = (event: React.MouseEvent<HTMLElement>) => {
      setAnchorEl(event.currentTarget as HTMLButtonElement);
    };

    const handleClose = () => {
      setAnchorEl(null);
    };

    const open = Boolean(anchorEl);
    const id = open ? 'card-popover' : undefined;

    const handleMenuItemClick =
      (
        onClick:
          | ((event: React.MouseEvent<HTMLButtonElement>) => void)
          | undefined
      ) =>
      (event: React.MouseEvent<HTMLButtonElement>) => {
        handleClose();
        if (onClick) {
          onClick(event);
        }
      };

    return (
      <Box>
        <StyledCardDropdown
          ref={buttonRef}
          aria-describedby={id}
          onClick={handleClick}
          data-cyid={`${testId}-card-button`}
          data-card-dropdown-size={size}
          data-button-root-full-height={fullHeight}
          data-button-root-active={active}
          {...props}
        >
          <Box className="startIcon">{icon}</Box>
          <Box>{text}</Box>
          <Box className="endIcon">
            {open ? (
              <ChevronUp fontSize="inherit" />
            ) : (
              <ChevronDown fontSize="inherit" />
            )}
          </Box>
        </StyledCardDropdown>
        <Popover
          id={id}
          open={open}
          anchorEl={anchorEl}
          onClose={handleClose}
          anchorOrigin={{
            vertical: 'bottom',
            horizontal: 'center',
          }}
          transformOrigin={{
            vertical: 'top',
            horizontal: 'center',
          }}
          PaperProps={{
            style: {
              width: buttonWidth,
              maxHeight: theme.spacing(40),
              boxShadow: theme.shadows[3],
              border: `1px solid ${theme.palette.grey[100]}`,
              borderRadius: '8px',
            },
            className: 'popoverPaper',
          }}
          elevation={0}
          data-cyid={`${testId}-popover`}
        >
          <MenuList>
            {React.Children.map(children, (menuItem) => {
              if (!menuItem) return null;
              return (
                <div>
                  {React.cloneElement(menuItem as React.ReactElement<any>, {
                    onClick: handleMenuItemClick(
                      (menuItem as React.ReactElement<any>).props.onClick
                    ),
                  })}
                </div>
              );
            })}
          </MenuList>
        </Popover>
      </Box>
    );
  }
);

CardDropdown.displayName = 'CardDropdown';
