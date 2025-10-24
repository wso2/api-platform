import { Box, type BoxProps, styled } from '@mui/material';
import type { ComponentType } from 'react';

export interface StyledAvatarUserNameProps extends BoxProps {
  disabled?: boolean;
}

export const StyledAvatarUserName: ComponentType<StyledAvatarUserNameProps> =
  styled(Box, {
    shouldForwardProp: (prop) => !['disabled'].includes(prop as string),
  })<StyledAvatarUserNameProps>(({ disabled, theme }) => ({
    opacity: disabled ? 0.5 : 1,
    cursor: disabled ? 'not-allowed' : 'pointer',
    backgroundColor: 'transparent',
    '.avatarUserName': {
      display: 'flex',
      alignItems: 'center',
      gridGap: theme.spacing(1),
    },
    display: 'flex',
    alignItems: 'center',
    textAlign: 'left',
    gap: theme.spacing(1),
    '& span': {
      color: theme.palette.text.primary,
      fontSize: theme.typography.body1.fontSize,
      fontWeight: theme.typography.fontWeightRegular,
    },
    '&:disabled': {
      cursor: 'not-allowed',
      opacity: 0.5,
      pointerEvents: 'none',
    },
    '& .MuiAvatar-root': {
      width: theme.spacing(5),
      height: theme.spacing(5),
      fontSize: theme.typography.body1.fontSize,
      backgroundColor: theme.palette.grey[100],
      color: theme.palette.primary.main,
      fontWeight: theme.typography.fontWeightBold,
    },
  }));
