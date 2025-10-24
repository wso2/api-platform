import React from "react";
import { Typography } from "@mui/material";
import { Button } from "../components/src/components/Button";
const Admin: React.FC = () => (
  <Typography variant="h5">
    <Button testId="share" size="small" color="primary" variant="outlined">
      Share
    </Button>
    Policies
  </Typography>
);
export default Admin;
