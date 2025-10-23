import React from 'react';
import { StyledLink } from './Link.styled';

export type linkVariant =
  | 'body1'
  | 'body2'
  | 'button'
  | 'caption'
  | 'h1'
  | 'h2'
  | 'h3'
  | 'h4'
  | 'h5'
  | 'h6'
  | 'inherit'
  | 'overline'
  | 'subtitle1'
  | 'subtitle2';

export type linkColorVariant =
  | 'primary'
  | 'secondary'
  | 'error'
  | 'warning'
  | 'info'
  | 'success'
  | 'textPrimary'
  | 'textSecondary'
  | 'textDisabled'
  | 'inherit'
  | 'textHint';

export type underlineVariant = 'none' | 'hover' | 'always';

export interface LinkProps {
  children?: React.ReactNode;
  className?: string;
  onClick?: (event: React.MouseEvent) => void;
  disabled?: boolean;
  variant?: linkVariant;
  color?: linkColorVariant;
  testId: string;
  underline?: underlineVariant;
  sx?: React.CSSProperties;
  [key: string]: any;
}

export const Link = React.forwardRef<HTMLAnchorElement, LinkProps>(
  ({ children, ...props }, ref) => {
    return (
      <StyledLink
        ref={ref}
        {...props}
        testId={`${props.testId}-link`}
        data-cyid={`${props.testId}-link`}
      >
        {children}
      </StyledLink>
    );
  }
);

Link.displayName = 'Link';
