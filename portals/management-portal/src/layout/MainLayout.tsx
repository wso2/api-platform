import React from "react";
import { Box } from "@mui/material";
import Header from "../components/Header";
import LeftSidebar from "../components/LeftSidebar";
import Footer, { FOOTER_HEIGHT } from "../components/Footer";

const MainLayout: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  return (
    <Box sx={{ display: "flex", minHeight: "100%" }}>
      <Header />
      <LeftSidebar />
      {/* <RightSidebar /> */}

      <Box
        component="main"
        sx={{
          flexGrow: 1,
          mt: 7,
          mr: 0,
          p: 5,
          pb: `${FOOTER_HEIGHT + 16}px`,
          bgcolor: "background.paper",
        }}
      >
        {children}
      </Box>

      <Footer />
    </Box>
  );
};

export default MainLayout;
