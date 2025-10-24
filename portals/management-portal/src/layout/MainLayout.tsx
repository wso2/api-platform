import React from "react";
import { Box, Toolbar } from "@mui/material";
import Header from "../components/Header";
import LeftSidebar, { LEFT_DRAWER_WIDTH } from "../components/LeftSidebar";
import RightSidebar, { RIGHT_DRAWER_WIDTH } from "../components/RightSidebar";
import Footer, { FOOTER_HEIGHT } from "../components/Footer";

const MainLayout: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  return (
    <Box sx={{ display: "flex", minHeight: "100%" }}>
      <Header />
      <LeftSidebar />
      <RightSidebar />

      <Box
        component="main"
        sx={{
          flexGrow: 1,
        //   ml: `${LEFT_DRAWER_WIDTH}px`,
          mr: `${RIGHT_DRAWER_WIDTH}px`,
          p: 3,
          pb: `${FOOTER_HEIGHT + 16}px`,
          bgcolor: "background.default",
          minHeight: "100vh",
        }}
      >
        <Toolbar /> 
        {children}
      </Box>

      <Footer />
    </Box>
  );
};

export default MainLayout;
