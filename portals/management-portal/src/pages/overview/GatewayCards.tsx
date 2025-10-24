import React from "react";
import {
  Grid,
  Card,
  CardActionArea,
  CardContent,
  Typography,
  Box,
} from "@mui/material";
import type { GwType } from "./types";
import hybridImg from "../../images/hybrid-gateway.svg";
import cloudImg from "../../images/cloud-gateway.svg";

export default function GatewayCards({
  selected,
  onSelect,
}: {
  selected: GwType | null;
  onSelect: (t: GwType) => void;
}) {
  const CardOne: React.FC<{ label: string; img: string; value: GwType }> = ({
    label,
    img,
    value,
  }) => {
    const isSelected = selected === value;
    // Highlight the first card ("hybrid") by default when nothing is selected.
    const highlighted = selected === null ? value === "hybrid" : isSelected;

    return (
      <Card
        variant="outlined"
        sx={{
          height: 340,
          borderRadius: 2,
          borderColor: highlighted ? "#069668" : "divider",
          boxShadow: highlighted ? 3 : 0,
          transition: "box-shadow 120ms ease, border-color 120ms ease",
        }}
      >
        <CardActionArea sx={{ height: "100%" }} onClick={() => onSelect(value)}>
          <CardContent
            sx={{
              height: "100%",
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              justifyContent: "center",
              gap: 1,
            }}
          >
            <Box sx={{ p: 3, borderColor: "divider", borderRadius: 2 }}>
              <Box
                component="img"
                src={img}
                alt={label}
                sx={{ width: 140, height: "auto" }}
              />
            </Box>
            <Typography variant="h6" fontWeight={600}>
              {label}
            </Typography>
            <Typography color="text.secondary" align="center" fontSize={14}>
              Let’s get started with creating your Gateways
            </Typography>
          </CardContent>
        </CardActionArea>
      </Card>
    );
  };

  return (
    <Grid container spacing={4} justifyContent="center">
      <Grid >
        <CardOne label="On Premise Gateway" img={hybridImg} value="hybrid" />
      </Grid>
      <Grid>
        <CardOne label="Cloud Gateway" img={cloudImg} value="cloud" />
      </Grid>
    </Grid>
  );
}
