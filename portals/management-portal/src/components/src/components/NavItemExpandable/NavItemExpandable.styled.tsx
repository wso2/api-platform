import { alpha, Box, type BoxProps, styled } from '@mui/material';
import type { ComponentType } from 'react';
import { Link, type LinkProps } from 'react-router-dom';


interface StyledNavItemContainerProps {
  isSubNavVisible?: boolean;
  isExpanded?: boolean;
  disabled?: boolean;
}

export const StyledNavItemContainer: ComponentType<
  BoxProps & StyledNavItemContainerProps
> = styled(Box)<BoxProps & StyledNavItemContainerProps>(
  ({ theme, isSubNavVisible, isExpanded, disabled }) => ({
    color: theme.palette.background.default,
    background: alpha(theme.palette.primary.main, isSubNavVisible ? 1 : 0.8),
    display: 'flex',
    flexDirection: 'column',
    width: isExpanded ? '100%' : 'fit-content',
    fontSize: '1rem',
    cursor: 'pointer',
    borderRadius: theme.shape.borderRadius,
    overflow: 'hidden',
    opacity: disabled ? 0.5 : 1,
    pointerEvents: disabled ? 'none' : 'auto',
  })
);

interface StyledSubNavContainerProps {
  isSelected?: boolean;
}

export const StyledSubNavContainer: ComponentType<
  BoxProps & StyledSubNavContainerProps
> = styled(Box)<BoxProps & StyledSubNavContainerProps>(
  ({ theme, isSelected }) => ({
    display: 'flex',
    background: isSelected ? theme.palette.primary.dark : 'transparent',
    cursor: 'pointer',
    width: '100%',
    flexDirection: 'column',
  })
);

export const StyledMainNavItemContainer: ComponentType<
  BoxProps & {
    isExpanded?: boolean;
    isSelected?: boolean;
    isSubNavVisible?: boolean;
  }
> = styled(Box)<
  BoxProps & {
    isExpanded?: boolean;
    isSelected?: boolean;
    isSubNavVisible?: boolean;
  }
>(({ theme, isExpanded, isSelected, isSubNavVisible }) => ({
  display: 'flex',
  gap: theme.spacing(1),
  background:
    isSubNavVisible && isSelected
      ? theme.palette.primary.dark
      : !isSubNavVisible && isSelected
        ? alpha(theme.palette.primary.light, 0.4)
        : 'transparent',
  alignItems: 'center',
  color: theme.palette.background.default,
  padding: theme.spacing(1.625, 1.5),
  paddingLeft: isExpanded ? theme.spacing(3) : theme.spacing(1.5),
  textDecoration: 'none',
  transition: theme.transitions.create(['background', 'padding'], {
    duration: theme.transitions.duration.short,
  }),
  '&:hover': {
    background: alpha(theme.palette.primary.light, 0.4),
  },
}));

export const StyledMainNavItemContainerWithLink: ComponentType<
  LinkProps & {
    isSelected?: boolean;
    isSubNavVisible?: boolean;
  }
> = styled(Link)<
  LinkProps & {
    isSelected?: boolean;
    isSubNavVisible?: boolean;
  }
>(({ theme, isSelected, isSubNavVisible }) => ({
  display: 'flex',
  gap: theme.spacing(1),
  background:
    isSubNavVisible && isSelected
      ? theme.palette.primary.dark
      : !isSubNavVisible && isSelected
        ? alpha(theme.palette.primary.light, 0.4)
        : 'transparent',
  alignItems: 'center',
  color: theme.palette.background.default,
  padding: theme.spacing(1.625, 2.25),
  textDecoration: 'none',
  transition: theme.transitions.create(['background', 'padding'], {
    duration: theme.transitions.duration.short,
  }),
  '&:hover': {
    background: alpha(theme.palette.primary.light, 0.4),
  },
}));

export const StyledSubNavItemContainer: ComponentType<
  LinkProps & { isExpanded?: boolean; isSelected?: boolean }
> = styled(Link)<LinkProps & { isExpanded?: boolean; isSelected?: boolean }>(
  ({ theme, isExpanded, isSelected }) => ({
    display: 'flex',
    gap: theme.spacing(1),
    background: isSelected
      ? alpha(theme.palette.primary.light, 0.4)
      : 'transparent',
    alignItems: 'center',
    color: theme.palette.background.default,
    padding: theme.spacing(1.625, 1.5),
    paddingLeft: isExpanded ? theme.spacing(3) : theme.spacing(1.5),
    textDecoration: 'none',
    transition: theme.transitions.create(['background', 'padding'], {
      duration: theme.transitions.duration.short,
    }),
    '&:hover': {
      background: alpha(theme.palette.primary.light, 0.4),
    },
  })
);

export const StyledSpinIcon: ComponentType<
  BoxProps & { isSubNavVisible: boolean; isExpanded?: boolean }
> = styled(Box)<BoxProps & { isSubNavVisible: boolean; isExpanded?: boolean }>(
  ({ theme, isSubNavVisible, isExpanded }) => ({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 0,
    overflow: 'visible',
    fontSize: '0.5rem',
    padding: isExpanded ? theme.spacing(0.5) : 0,
    transform: isSubNavVisible ? 'rotate(90deg)' : 'rotate(0deg)',
    transition: theme.transitions.create(['transform'], {
      duration: theme.transitions.duration.short,
    }),
  })
);
