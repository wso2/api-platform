// REPLACE your existing OptionCard with this
import * as React from "react";
import { Box, Divider, Typography } from "@mui/material";
import LaunchOutlinedIcon from "@mui/icons-material/LaunchOutlined";
import {
  Card,
  CardActionArea,
  CardContent,
} from "../../components/src/components/Card";
import Edit from "../../components/src/Icons/generated/Edit";
import { Link } from "../../components/src/components/Link";
import { Button } from "../../components/src/components/Button";
import { IconButton } from "../../components/src/components/IconButton";
import { Chip } from "../../components/src/components/Chip";

const valuePill = (
  text: string,
  variant: "green" | "grey" | "red" = "grey"
) => {
  if (variant === "green") {
    return <Chip label={text} color="success" />;
  }
  if (variant === "red") {
    return <Chip label={text} color="error" variant="outlined" />;
  }
  return <Chip label={text} variant="outlined" color="default" />;
};

type OptionCardProps = {
  title: string;
  description: string;
  selected: boolean;
  onClick: () => void;

  /** optional extras */
  logoSrc?: string;
  logoAlt?: string;
  portalUrl?: string;
  userAuthLabel?: string;
  authStrategyLabel?: string;
  visibilityLabel?: string;

  /** NEW: action callbacks */
  onEdit?: () => void;
  onActivate?: () => void;
};

const PortalCard: React.FC<OptionCardProps> = ({
  title,
  description,
  selected, // currently unused visually, but kept for parity
  onClick,
  logoSrc,
  logoAlt = "Portal logo",
  portalUrl = "https://test12345.eu.wso2.com",
  userAuthLabel = "Asgardeo Thunder",
  authStrategyLabel = "Auth-Key",
  visibilityLabel = "Private",
  onEdit,
  onActivate,
}) => {
  return (
    <Card testId={""}>
      <CardActionArea onClick={onClick} testId={""}>
        <CardContent>
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: "auto 1fr auto",
              gap: 2,
              alignItems: "start",
            }}
          >
            {/* Logo block */}
            <Box
              sx={{
                width: 100,
                height: 100,
                borderRadius: 2,
                bgcolor: "transparent",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
              }}
            >
              <Box
                sx={{
                  width: 100,
                  height: 100,
                  borderRadius: "8px",
                  border: "0.5px solid #abb8c2ff",
                  bgcolor: "#d9e0e4ff",
                  overflow: "hidden",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                }}
              >
                {logoSrc ? (
                  <Box
                    component="img"
                    src={logoSrc}
                    alt={logoAlt}
                    sx={{ width: 90, height: 90, objectFit: "contain" }}
                  />
                ) : null}
              </Box>
            </Box>

            {/* Title + description + URL */}
            <Box sx={{ minWidth: 0 }}>
              <Box sx={{ display: "flex", alignItems: "center" }}>
                <Typography sx={{ mr: 1 }} fontSize={18} fontWeight={600}>
                  {title}
                </Typography>
                <IconButton
                  size="small"
                  onClick={(e: React.MouseEvent) => {
                    e.stopPropagation(); // prevent CardActionArea click
                    onEdit?.();
                  }}
                >
                  <Edit style={{ fontSize: 16 }} />
                </IconButton>
              </Box>

              <Typography
                sx={{
                  mt: 0.5,
                  lineHeight: 1.5,
                  color: "rgba(0,0,0,0.6)",
                  maxWidth: 300,
                }}
                variant="body2"
              >
                {description}
              </Typography>

              <Box sx={{ mt: 1, display: "flex", alignItems: "center" }}>
                <Link
                  href={portalUrl}
                  underline="hover"
                  target="_blank"
                  rel="noopener"
                  sx={{ fontWeight: 600 }}
                  onClick={(e: React.MouseEvent) => e.stopPropagation()}
                >
                  {portalUrl}
                </Link>
                <IconButton
                  size="small"
                  sx={{ ml: 0.5 }}
                  onClick={(e: React.MouseEvent) => {
                    e.stopPropagation();
                    window.open(portalUrl, "_blank", "noopener,noreferrer");
                  }}
                >
                  <LaunchOutlinedIcon fontSize="inherit" />
                </IconButton>
              </Box>
            </Box>

            {/* right spacer */}
            <Box />
          </Box>

          {/* Divider */}
          <Divider sx={{ my: 2 }} />

          {/* Spec rows */}
          <Box sx={{ display: "grid", gap: 3 }}>
            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: "1fr auto",
                alignItems: "center",
                columnGap: 16,
              }}
            >
              <Typography>User authentication</Typography>
              {valuePill(userAuthLabel, "grey")}
            </Box>

            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: "1fr auto",
                alignItems: "center",
                columnGap: 16,
              }}
            >
              <Typography>Authentication strategy</Typography>
              {valuePill(authStrategyLabel, "grey")}
            </Box>

            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: "1fr auto",
                alignItems: "center",
                columnGap: 16,
              }}
            >
              <Typography>Visibility</Typography>
              {valuePill(visibilityLabel, "red")}
            </Box>
          </Box>

          {/* CTA */}
          <Box sx={{ mt: 2 }}>
            <Button
              fullWidth
              onClick={(e: React.MouseEvent) => {
                e.stopPropagation(); // don’t trigger card onClick
                onActivate?.();
              }}
            >
              Activate Developer Portal
            </Button>
          </Box>
        </CardContent>
      </CardActionArea>
    </Card>
  );
};

export default PortalCard;
