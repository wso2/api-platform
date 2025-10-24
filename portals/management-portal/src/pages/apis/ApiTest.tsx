import React from "react";
import { Typography, Box } from "@mui/material";

const ApiTest: React.FC = () => (
  <Box>
    <Typography variant="h4" fontWeight={800} gutterBottom>
      APIs · Test
    </Typography>
    <Typography color="text.secondary">
      Tools and consoles to try out your APIs.
    </Typography>
  </Box>
);

export default ApiTest;
