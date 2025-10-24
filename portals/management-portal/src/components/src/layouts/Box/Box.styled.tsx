import styled from '@emotion/styled';
import { Box } from '@mui/material';
import type { ComponentType, MouseEvent, ReactNode } from 'react';

export interface BoxProps {
  children?: ReactNode;
  className?: string;
  testId?: string;
  onClick?: (event: MouseEvent<HTMLDivElement>) => void;
  onMouseEnter?: (event: MouseEvent<HTMLDivElement>) => void;
  onMouseLeave?: (event: MouseEvent<HTMLDivElement>) => void;
  disabled?: boolean;

  // Style props
  backgroundColor?: string;
  height?: string | number;
  width?: string | number;
  display?: 'flex' | 'block' | 'inline-block' | 'grid' | 'inline-grid';
  flexDirection?: 'row' | 'column' | 'row-reverse' | 'column-reverse';
  overflow?: 'visible' | 'hidden' | 'scroll' | 'auto';
  padding?: string | number;
  margin?: string | number;
  border?: 'small' | 'medium';
  borderBottom?: 'small' | 'medium';
  borderTop?: 'small' | 'medium';
  borderLeft?: 'small' | 'medium';
  borderRight?: 'small' | 'medium';
  borderColor?: string;
  borderRadius?: string | number;
  boxShadow?: string;
  cursor?: 'pointer' | 'default' | 'not-allowed';
  color?: string;
  transition?: string;
  minHeight?: string | number;
  maxHeight?: string | number;
  minWidth?: string | number;
  maxWidth?: string | number;
  flexGrow?: string | number;
  justifyContent?:
  | 'flex-start'
  | 'flex-end'
  | 'center'
  | 'space-between'
  | 'space-around'
  | 'space-evenly';
  alignItems?: 'flex-start' | 'flex-end' | 'center' | 'baseline' | 'stretch';
  position?: 'static' | 'relative' | 'absolute' | 'fixed' | 'sticky';
  gap?: string | number;
  zIndex?: string | number;
}

function getBorder(border: BoxProps['border']) {
  if (border === 'small') {
    return '0.5px solid';
  }
  if (border === 'medium') {
    return '1px solid';
  }
  return border;
}

export const StyledBox: ComponentType<BoxProps> = styled(Box)<BoxProps>(
  ({
    transition,
    backgroundColor,
    height,
    width,
    display,
    flexDirection,
    overflow,
    padding,
    margin,
    border,
    borderRadius,
    boxShadow,
    cursor,
    color,
    minHeight,
    maxHeight,
    minWidth,
    maxWidth,
    flexGrow,
    position,
    borderColor,
    borderBottom,
    borderTop,
    borderLeft,
    borderRight,
    gap,
    justifyContent,
    alignItems,
    zIndex,
  }) => ({
    transition: transition,
    backgroundColor: backgroundColor,
    height: height,
    width: width,
    display: display,
    flexDirection: flexDirection,
    overflow: overflow,
    padding: padding,
    margin: margin,
    border: getBorder(border),
    borderBottom: getBorder(borderBottom),
    borderTop: getBorder(borderTop),
    borderLeft: getBorder(borderLeft),
    borderRight: getBorder(borderRight),
    borderColor: borderColor,
    borderRadius: borderRadius,
    boxShadow: boxShadow,
    cursor: cursor,
    color: color,
    minHeight: minHeight,
    maxHeight: maxHeight,
    minWidth: minWidth,
    maxWidth: maxWidth,
    flexGrow: flexGrow,
    position: position,
    gap: gap,
    justifyContent: justifyContent,
    alignItems: alignItems,
    zIndex: zIndex,
  })
);
