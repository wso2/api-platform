import React from 'react';
import { StyledAvatarUserName } from './AvatarUserName.styled';
import { Avatar } from '../Avatar/Avatar';
import { Typography } from '@mui/material';

export interface AvatarUserNameProps {
  /** The content to be rendered within the component */
  children?: React.ReactNode;
  /** Additional CSS class names */
  className?: string;
  /** Click event handler */
  onClick?: (event: React.MouseEvent<HTMLDivElement>) => void;
  /** Whether the component is disabled */
  disabled?: boolean;
  /**
   * username to be displayed
   */
  username?: string | 'John Doe';
  /**
   * hide the username
   */
  hideUsername?: boolean;
  /**
   * The sx prop for custom styles
   */
  sx?: React.CSSProperties;
  /**
   * Additional props for the component
   */
  [key: string]: any;
}

/**
 * AvatarUserName component
 * @component
 */
export const AvatarUserName = React.forwardRef<
  HTMLDivElement,
  AvatarUserNameProps
>(({ children, className, onClick, disabled = false, ...props }, ref) => {
  return (
    <StyledAvatarUserName
      ref={ref}
      className={className}
      disabled={disabled}
      {...props}
    >
      {disabled ? (
        <>
          <Avatar disabled={true}>{children}</Avatar>
          {!props.hideUsername && props.username && (
            <Typography component="span">{props.username}</Typography>
          )}
        </>
      ) : (
        <>
          <Avatar>{children}</Avatar>
          {!props.hideUsername && props.username && (
            <Typography component="span">{props.username}</Typography>
          )}
        </>
      )}
    </StyledAvatarUserName>
  );
});

AvatarUserName.displayName = 'AvatarUserName';
