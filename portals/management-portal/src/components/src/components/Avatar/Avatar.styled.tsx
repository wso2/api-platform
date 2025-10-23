import { Avatar as MUIAvatar, styled } from '@mui/material';
import type { ComponentType } from 'react';

export type colorVariant =
  | 'primary'
  | 'secondary'
  | 'error'
  | 'warning'
  | 'info'
  | 'success';
export type avatarVariant = 'circular' | 'rounded' | 'square';
export type avatarBackgroundColorVariant =
  | 'default'
  | 'primary'
  | 'secondary'
  | 'error'
  | 'warning'
  | 'info'
  | 'success';

export interface AvatarProps {
  /**
   * The content of the component
   */
  children?: React.ReactNode;
  /**
   * Additional className for the component
   */
  className?: string;
  /**
   * color variant of the avatar
   */
  color?: colorVariant;
  /**
   * Optional click handler
   */
  onClick?: (event: React.MouseEvent) => void;
  /**
   * If true, the component will be disabled
   */
  disabled?: boolean;
  /**
   * width of the avatar
   */
  width?: string | number;
  /**
   * height of the avatar
   * */
  height?: string | number;
  /**
   * The variant of the component
   */
  variant?: avatarVariant;
  /**
   * The sx prop for custom styles
   */
  sx?: React.CSSProperties;
}

export const StyledAvatar: ComponentType<AvatarProps> = styled(
  MUIAvatar
)<AvatarProps>(({
  theme,
  variant = 'circular',
  color = 'primary',
  disabled = false,
  height,
  width,
}) => {
  const getBorderRadius = () => {
    switch (variant) {
      case 'circular':
        return '50%';
      case 'rounded':
        return '8px';
      case 'square':
        return '0px';
      default:
        return '50%';
    }
  };
  const getBackgroundColor = () => {
    switch (color) {
      case 'primary':
        return theme.palette.primary.main;
      case 'secondary':
        return theme.palette.secondary.light;
      case 'error':
        return theme.palette.error.main;
      case 'warning':
        return theme.palette.warning.main;
      case 'info':
        return theme.palette.info.main;
      case 'success':
        return theme.palette.success.main;
      default:
        return theme.palette.primary.main;
    }
  };
  const getColor = () => {
    switch (color) {
      case 'primary':
        return theme.palette.primary.contrastText;
      case 'secondary':
        return theme.palette.primary.light;
      case 'error':
        return theme.palette.error.contrastText;
      case 'warning':
        return theme.palette.warning.contrastText;
      case 'info':
        return theme.palette.info.contrastText;
      case 'success':
        return theme.palette.success.contrastText;
      default:
        return theme.palette.primary.contrastText;
    }
  };
  return {
    borderRadius: getBorderRadius(),
    backgroundColor: getBackgroundColor(),
    opacity: disabled ? 0.5 : 1,
    color: getColor(),
    cursor: disabled ? 'not-allowed' : 'pointer',
    textAlign: 'center',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width,
    height,
    boxShadow: theme.shadows[1],
    pointerEvents: disabled ? 'none' : ('auto' as const),
  };
});
