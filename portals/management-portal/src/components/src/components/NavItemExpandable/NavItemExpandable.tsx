import React, { useMemo, useState } from 'react';
import {
  StyledMainNavItemContainer,
  StyledMainNavItemContainerWithLink,
  StyledNavItemContainer,
  StyledSpinIcon,
  StyledSubNavContainer,
  StyledSubNavItemContainer,
} from './NavItemExpandable.styled';
import { Box, Collapse, Typography, useTheme } from '@mui/material';
import { ChevronRightIcon } from '../../Icons';

export interface NavItemBase {
  title: string;
  id: string;
  icon: React.ReactNode;
  href?: string;
  selectedIcon: React.ReactNode;
  pathPattern: string;
}

export interface NavItemExpandableSubMenu extends NavItemBase {
  subMenuItems?: NavItemBase[];
  href?: string;
}

export interface NavItemExpandableProps extends NavItemExpandableSubMenu {
  className?: string;
  onClick?: (id: string) => void;
  disabled?: boolean;
  selectedId?: string;
  isExpanded?: boolean;
}

/**
 * NavItemExpandable component
 * @component
 */
export const NavItemExpandable = React.forwardRef<
  HTMLDivElement,
  NavItemExpandableProps
>((props, ref) => {
  const {
    className,
    onClick,
    disabled,
    selectedId,
    isExpanded,
    title,
    icon,
    selectedIcon,
    subMenuItems,
    id,
    href,
  } = props;
  const [isSubNavVisible, setIsSubNavVisible] = useState(false);
  const theme = useTheme();
  const isSelected = useMemo(
    () =>
      id === selectedId ||
      !!subMenuItems?.find((item) => item.id === selectedId),
    [id, selectedId, subMenuItems]
  );
  const handleOnClick = (id: string) => {
    if (!disabled && onClick) {
      onClick(id);
    }
  };

  const handleMainNavItemClick = () => {
    if (!disabled && onClick && !subMenuItems) {
      onClick(id);
    }
    setIsSubNavVisible(!isSubNavVisible);
  };

  const isSubNavExpanded = useMemo(
    () => !!(isSubNavVisible && subMenuItems),
    [isSubNavVisible, subMenuItems]
  );

  if (!subMenuItems || subMenuItems.length === 0) {
    return (
      <StyledNavItemContainer
        isSubNavVisible={isSubNavExpanded}
        className={className}
        isExpanded={isExpanded}
        disabled={disabled}
        ref={ref}
      >
        <StyledMainNavItemContainerWithLink
          to={href ?? ''}
          onClick={() => handleOnClick(id)}
          isSelected={id === selectedId}
          key={id}
        >
          {isSubNavExpanded ? selectedIcon : icon}
          {isExpanded && (
            <Typography variant="body2" color="inherit">
              {title}
            </Typography>
          )}
        </StyledMainNavItemContainerWithLink>
      </StyledNavItemContainer>
    )
  }

  return (
    <StyledNavItemContainer
      isSubNavVisible={isSubNavExpanded}
      className={className}
      isExpanded={isExpanded}
      disabled={disabled}
      ref={ref}
    >
      <StyledMainNavItemContainer
        onClick={handleMainNavItemClick}
        isSelected={isSelected}
        isSubNavVisible={isSubNavExpanded}
      >
        <Box
          flexDirection="row"
          display="flex"
          flexGrow={1}
          alignItems="center"
          gap={1}
          pl={theme.spacing(0.5)}
          whiteSpace="nowrap"
        >
          {isSubNavExpanded ? selectedIcon : icon}
          {isExpanded && (
            <Typography variant="body2" color="inherit">
              {title}
            </Typography>
          )}
        </Box>
        {subMenuItems ? (
          <StyledSpinIcon isSubNavVisible={isSubNavExpanded}>
            <ChevronRightIcon fontSize="inherit" />
          </StyledSpinIcon>
        ) : (
          <Box />
        )}
      </StyledMainNavItemContainer>

      <Collapse in={isSubNavExpanded} mountOnEnter unmountOnExit>
        <StyledSubNavContainer isSelected={isSelected}>
          {subMenuItems?.map((item) => (
            <StyledSubNavItemContainer
              to={item.href ?? ''}
              onClick={() => handleOnClick(item.id)}
              isExpanded={isExpanded}
              isSelected={item.id === selectedId}
              key={item.id}
            >
              <Box
                flexDirection="row"
                display="flex"
                pl={theme.spacing(0.5)}
                flexGrow={1}
                alignItems="center"
                gap={1}
              >
                {item.id === selectedId ? item.selectedIcon : item.icon}
                {isExpanded && (
                  <Typography variant="body2" color="inherit" noWrap>
                    {item.title}
                  </Typography>
                )}
              </Box>
            </StyledSubNavItemContainer>
          ))}
        </StyledSubNavContainer>
      </Collapse>
    </StyledNavItemContainer>
  );
});

NavItemExpandable.displayName = 'NavItemExpandable';
