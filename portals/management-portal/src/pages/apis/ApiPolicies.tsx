import React from "react";
import { Typography, Box } from "@mui/material";

const ApiPolicies: React.FC = () => (
  <Box>
    <Typography variant="h4" fontWeight={800} gutterBottom>
      APIs · Policies
    </Typography>
    <Typography color="text.secondary">
      Rate limits, auth, and governance policies for your APIs.
    </Typography>
  </Box>
);

export default ApiPolicies;
