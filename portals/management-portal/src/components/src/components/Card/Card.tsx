import React, { type ReactNode } from 'react';
import { StyledCard } from './Card.styled';

export type CardBorderRadius = 'xs' | 'sm' | 'md' | 'lg' | 'square';
export type CardBoxShadow = 'none' | 'light' | 'dark';
export type CardBgColor = 'default' | 'secondary';

export interface CardProps {
  children?: ReactNode;
  borderRadius?: CardBorderRadius;
  boxShadow?: CardBoxShadow;
  disabled?: boolean;
  testId: string;
  bgColor?: CardBgColor;
  className?: string;
  fullHeight?: boolean;
  variant?: 'elevation' | 'outlined';
  onClick?: (event: React.MouseEvent<HTMLDivElement>) => void;
  style?: React.CSSProperties;
}

export const Card = ({
  children,
  borderRadius = 'sm',
  boxShadow = 'light',
  disabled = false,
  variant = 'elevation',
  testId,
  fullHeight = false,
  bgColor = 'default',
  ...rest
}: CardProps) => (
  <StyledCard
    {...rest}
    data-cyid={`${testId}-card`}
    data-border-radius={borderRadius}
    data-box-shadow={boxShadow}
    data-disabled={disabled}
    data-bg-color={bgColor}
    variant={variant}
    style={{ ...rest.style }}
  >
    {children}
  </StyledCard>
);
