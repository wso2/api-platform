import {
  Box,
  styled,
  type Theme,
  Radio as MuiRadio,
  type RadioProps as MuiRadioProps,
} from '@mui/material';
import type { ComponentType } from 'react';

export type colorVariant =
  | 'primary'
  | 'default'
  | 'secondary'
  | 'error'
  | 'warning'
  | 'info'
  | 'success';

export interface StyledRadioProps {
  className?: string;
  onClick?: (event: React.MouseEvent) => void;
  disabled?: boolean;
  children?: React.ReactNode;
  theme?: Theme;
}

export const StyledRadio: ComponentType<StyledRadioProps> = styled(Box, {
  shouldForwardProp: (prop) => !['disabled'].includes(prop as string),
})<StyledRadioProps>(({ theme, disabled }) => ({
  display: 'inline-flex',
  alignItems: 'center',
  cursor: disabled ? 'default' : 'pointer',
  opacity: disabled ? 0.6 : 1,
  pointerEvents: disabled ? 'none' : 'auto',
  radioButton: {
    margin: theme.spacing(-1, 0),
  },
  radioLabelRoot: {
    marginLeft: theme.spacing(-1),
  },
  radioLabelDisabled: {
    color: theme.palette.grey[200],
  },
}));

export interface RadioIndicatorProos {
  color?: colorVariant;
}

export const StyledRadioIndicator: ComponentType<
  MuiRadioProps & RadioIndicatorProos
> = styled(MuiRadio)<MuiRadioProps & RadioIndicatorProos>(
  ({ theme, color = 'default' }) => ({
    color: theme.palette.text.primary,
    '&.Mui-checked': {
      color:
        color === 'primary'
          ? theme.palette.primary.main
          : color === 'secondary'
            ? theme.palette.secondary.main
            : color === 'error'
              ? theme.palette.error.main
              : color === 'warning'
                ? theme.palette.warning.main
                : color === 'info'
                  ? theme.palette.info.main
                  : color === 'success'
                    ? theme.palette.success.main
                    : color === 'default'
                      ? theme.palette.text.primary
                      : theme.palette.primary.main,
    },
    '&.Mui-disabled': {
      color: theme.palette.action.disabled,
    },
  })
);
