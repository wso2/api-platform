import {
  Box,
  Card,
  CardContent,
  Typography,
  Stack,
} from "@mui/material";
import { useNavigate } from "react-router-dom";
import { useDevPortals } from "../../../context/DevPortalContext";
import { IconButton } from "../../../components/src/components/IconButton";
import LaunchOutlinedIcon from "@mui/icons-material/LaunchOutlined";

// Import the logo image
import BijiraDPLogo from "../../BijiraDPLogo.png";

type Portal = { id: string; name: string; href: string; description?: string };

type Props = {
  height?: number;
  href: string;
  portals?: Portal[];
};

export default function APIPortalWidget({ height = 220, href, portals = [] }: Props) {
  const navigate = useNavigate();
  const { devportals } = useDevPortals();

  // Use provided portals or map devportals
  const displayPortals = portals.length > 0 ? portals : devportals
    .filter(portal => portal.isActive)
    .map(portal => ({
      id: portal.uuid,
      name: portal.name,
      href: portal.uiUrl || '#',
      description: portal.description || ''
    }));

  return (
    <Card variant="outlined" sx={{ height }}>
      <CardContent sx={{ height: "100%", display: "flex", flexDirection: "column" }}>
        <Typography variant="subtitle1" fontWeight={700} sx={{ mb: 1.5 }}>
          Dev Portals
        </Typography>

        <Box sx={{ flex: 1, overflow: "auto" }}>
          {displayPortals.length === 0 ? (
            <Box
              sx={{
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
                justifyContent: "center",
                height: "100%",
                textAlign: "center",
                py: 2,
              }}
            >
              <Box
                component="img"
                src={BijiraDPLogo}
                alt="Bijira DP Logo"
                sx={{ width: 48, height: 48, opacity: 0.6, mb: 1 }}
              />
              <Typography variant="body2" color="text.secondary">
                No API portals available
              </Typography>
            </Box>
          ) : (
            <Stack spacing={1.5}>
              {displayPortals.slice(0, 3).map((portal) => (
                <Box
                  key={portal.id}
                  sx={(t) => ({
                    display: "flex",
                    alignItems: "center",
                    gap: 1.5,
                    p: 1.5,
                    borderRadius: 2,
                    border: `1px solid ${t.palette.divider}`,
                    bgcolor: t.palette.action.hover,
                  })}
                >
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Typography
                      variant="body2"
                      fontWeight={600}
                      sx={{
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        whiteSpace: "nowrap",
                      }}
                    >
                      {portal.name}
                    </Typography>
                    {portal.description && (
                      <Typography
                        variant="caption"
                        color="#666666"
                        sx={{
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                          display: "block",
                          mt: 0.25,
                        }}
                      >
                        {portal.description}
                      </Typography>
                    )}
                  </Box>
                  {portal.href && portal.href !== '#' && (
                    <IconButton
                      size="small"
                      onClick={(e: React.MouseEvent) => {
                        e.stopPropagation();
                        window.open(portal.href, '_blank', 'noopener,noreferrer');
                      }}
                      sx={{ flexShrink: 0 }}
                    >
                      <LaunchOutlinedIcon fontSize="small" />
                    </IconButton>
                  )}
                </Box>
              ))}
            </Stack>
          )}
        </Box>

        {displayPortals.length > 0 && (
          <Typography
            variant="body2"
            color="primary"
            sx={{ mt: 1.5, cursor: "pointer" }}
            onClick={() => navigate(href)}
          >
            Manage Dev Portals →
          </Typography>
        )}
      </CardContent>
    </Card>
  );
}
