import React from "react";
import { Drawer, Toolbar, Box, IconButton, Tooltip } from "@mui/material";
import ChatBubbleOutlineIcon from "@mui/icons-material/ChatBubbleOutline";
import ArticleOutlinedIcon from '@mui/icons-material/ArticleOutlined';

export const RIGHT_DRAWER_WIDTH = 72;

const RightSidebar: React.FC = () => {
  return (
    <Drawer
      variant="permanent"
      anchor="right"
      sx={{
        // width: RIGHT_DRAWER_WIDTH,
        flexShrink: 0,
        "& .MuiDrawer-paper": {
          width: RIGHT_DRAWER_WIDTH,
          boxSizing: "border-box",
          borderLeft: "1px solid #e8e8ee",
        },
      }}
    >
      <Toolbar />
      <Box
        sx={{
          height: "100%",
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          pt: 2,
          gap: 2,
        }}
      >
        <Tooltip title="Feedback">
          <IconButton>
            <ChatBubbleOutlineIcon />
          </IconButton>
        </Tooltip>
        <Tooltip title="Docs">
          <IconButton>
            <ArticleOutlinedIcon />
          </IconButton>
        </Tooltip>
      </Box>
    </Drawer>
  );
};

export default RightSidebar;
