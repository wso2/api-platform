import React from "react";
import { Box } from "@mui/material";

interface CardsPageLayoutProps {
  /**
   * Content to display at the top-left of the toolbar (e.g., page title)
   */
  topLeft?: React.ReactNode;
  
  /**
   * Content to display at the top-right of the toolbar (e.g., search bar, buttons)
   */
  topRight?: React.ReactNode;
  
  /**
   * Whether to show the toolbar. If false, only cards will be displayed.
   */
  showToolbar?: boolean;
  
  /**
   * The width of each card in pixels. Default is 350px.
   */
  cardWidth?: number;
  
  /**
   * The card elements to display in the grid
   */
  children: React.ReactNode;
}

/**
 * A reusable layout component for displaying cards in a responsive grid
 * with an optional toolbar containing left and right content.
 * 
 * The grid automatically fits 350px-wide cards and centers them.
 */
const CardsPageLayout: React.FC<CardsPageLayoutProps> = ({
  topLeft,
  topRight,
  showToolbar = true,
  cardWidth = 350,
  children,
}) => {
  return (
    <Box
      sx={{
        display: 'grid',
        gridTemplateColumns: `repeat(auto-fit, ${cardWidth}px)`,
        gap: 2,
        justifyContent: 'center',
        alignItems: 'stretch',
      }}
    >
      {showToolbar && (topLeft || topRight) && (
        <Box
          sx={{
            gridColumn: '1 / -1',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 2,
            mb: 0,
          }}
        >
          {topLeft}
          {topRight}
        </Box>
      )}
      {children}
    </Box>
  );
};

export default CardsPageLayout;
