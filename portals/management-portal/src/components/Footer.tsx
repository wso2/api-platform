import React from "react";
import { Box, Link } from "@mui/material";
import { LEFT_DRAWER_WIDTH } from "./LeftSidebar";
import { RIGHT_DRAWER_WIDTH } from "./RightSidebar";

export const FOOTER_HEIGHT = 40;

const Footer: React.FC = () => {
  return (
    <Box
      sx={{
        position: "fixed",
        bottom: 0,
        left: `${LEFT_DRAWER_WIDTH}px`,
        right: `${RIGHT_DRAWER_WIDTH}px`,
        height: FOOTER_HEIGHT,
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        px: 2,
        borderTop: "1px solid #e8e8ee",
        bgcolor: "background.paper",
      }}
    >
      <Box sx={{ display: "flex", gap: 2 }}>
        <Link href="#" underline="hover" fontSize={14}>
          Terms of service
        </Link>
        <Link href="#" underline="hover" fontSize={14}>
          Privacy Policy
        </Link>
        <Link href="#" underline="hover" fontSize={14}>
          Support
        </Link>
      </Box>
      <Box component="span" sx={{ color: "text.secondary" }} fontSize={14}>
        © 2025 WSO2 LLC
      </Box>
    </Box>
  );
};

export default Footer;
