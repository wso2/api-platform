import { Switch, type SwitchProps, styled } from '@mui/material';
import type { ComponentType } from 'react';

export type colorVariant = 'primary' | 'default'; // Updated to match requirements
export type sizeVariant = 'small' | 'medium';

export interface StyledTogglerProps extends SwitchProps {
  disabled?: boolean;
  size?: sizeVariant;
  color?: colorVariant;
}

export const StyledToggler: ComponentType<StyledTogglerProps> = styled(
  Switch
)<StyledTogglerProps>(({
  disabled,
  theme,
  size = 'medium',
  color = 'default', // Changed default to 'default'
}) => {
  const getColor = () => {
    switch (color) {
      case 'primary':
        return theme.palette.primary.main;
      case 'default':
      default:
        return theme.palette.success.main; // Default is success color as required
    }
  };

  return {
    padding: theme.spacing(0.1),
    margin: 0,
    cursor: disabled ? 'not-allowed' : 'pointer',
    width: size === 'small' ? theme.spacing(3.5) : theme.spacing(5.5),
    height: size === 'small' ? theme.spacing(2) : theme.spacing(3),
    display: 'flex',
    alignItems: 'center',
    opacity: disabled ? 0.5 : 1,
    pointerEvents: disabled ? 'none' : 'auto',

    '& .MuiSwitch-switchBase': {
      padding: theme.spacing(0.25),
      margin: 0,

      '&.Mui-disabled': {
        '& + .MuiSwitch-track': {
          opacity: 0.5,
          pointerEvents: 'none',
        },
        '& .MuiSwitch-thumb': {
          backgroundColor: theme.palette.grey[200],
        },
        '&.Mui-checked': {
          '& .MuiSwitch-thumb': {
            backgroundColor:
              color === 'primary'
                ? theme.palette.primary.main
                : theme.palette.success.main,
          },
          '& + .MuiSwitch-track': {
            backgroundColor: 'transparent',
            border: `1px solid ${color === 'primary' ? theme.palette.primary.main : theme.palette.success.main}`,
            opacity: 1,
          },
        },
      },
      '&.Mui-checked': {
        transform: size === 'small' ? 'translateX(12px)' : 'translateX(20px)',
        '& + .MuiSwitch-track': {
          opacity: 1,
          backgroundColor: getColor(),
          border: `1px solid ${getColor()}`,
        },
        '& .MuiSwitch-thumb': {
          backgroundColor: theme.palette.common.white,
        },
      },
    },

    '& .MuiSwitch-thumb': {
      boxShadow: 'none',
      backgroundColor: theme.palette.grey[200],
      width: size === 'small' ? theme.spacing(1.5) : theme.spacing(2.5),
      height: size === 'small' ? theme.spacing(1.5) : theme.spacing(2.5),
      borderRadius:
        size === 'small' ? theme.spacing(0.75) : theme.spacing(1.25),
    },

    '& .MuiSwitch-track': {
      border: `1px solid ${theme.palette.grey[200]}`,
      borderRadius: size === 'small' ? theme.spacing(1) : theme.spacing(1.5),
      backgroundColor: theme.palette.common.white,
      opacity: 1,
    },
  };
});
