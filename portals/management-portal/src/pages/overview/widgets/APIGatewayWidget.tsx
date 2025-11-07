// portals/management-portal/src/pages/overview/widgets/APIGatewayWidget.tsx
import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  Stack,
  Typography,
} from "@mui/material";
import { useNavigate, Link as RouterLink } from "react-router-dom";
import { useGateways } from "../../../context/GatewayContext";
import { IconButton } from "../../../components/src/components/IconButton";
import { Button } from "../../../components/src/components/Button";
import ArrowForwardIosIcon from "@mui/icons-material/ArrowForwardIos";

// empty-state illustration
import emptyImg from "./Gateways.svg";

type Props = {
  height?: number;
  href: string; // e.g., `/${orgHandle}/${projectSlug}/gateway`
};

export default function APIGatewayWidget({ height = 220, href }: Props) {
  const { gateways, getGatewayStatus, gatewayStatuses } = useGateways();
  const navigate = useNavigate();

  const typeOf = (g: any) => String(g?.type ?? "").toLowerCase();
  const byType = (t: string) => gateways.filter((g) => typeOf(g) === t).length;

  const isActiveById = (id: string) => {
    const s =
      typeof getGatewayStatus === "function"
        ? getGatewayStatus(id)
        : gatewayStatuses?.[id];
    return s?.isActive === true;
  };

  const activeCount = gateways.reduce(
    (n, g) => n + (isActiveById(g.id) ? 1 : 0),
    0
  );
  const inactiveCount = Math.max(gateways.length - activeCount, 0);

  const aiCount = byType("ai");
  const eventCount = byType("event");
  const mcpCount = byType("mcp");
  const regularCount = byType("regular") + byType("hybrid");
  const hasInactive = inactiveCount > 0;
  const hasGateways = gateways.length > 0;

  return (
    <Card variant="outlined" sx={{ height }}>
      <CardActionArea sx={{ height: "100%" }} onClick={() => navigate(href)}>
        <CardContent
          sx={{
            height: "100%",
            display: "flex",
            flexDirection: "column",
            gap: 1,
          }}
        >
          {/* Header */}
          <Box
            display="flex"
            justifyContent="space-between"
            alignItems="center"
          >
            <Typography variant="subtitle1" fontWeight={700}>
              API Gateway
            </Typography>
            <IconButton
              aria-label="Go to Gateway"
              size="small"
              component={RouterLink}
              to={href}
              onClick={(e: any) => e.stopPropagation()} // don't trigger CardActionArea
            >
              <ArrowForwardIosIcon fontSize="inherit" />
            </IconButton>
          </Box>

          {/* Empty state */}
          {!hasGateways ? (
            <Box
              sx={{
                flex: 1,
                display: "grid",
                placeItems: "center",
                textAlign: "center",
                py: 1,
              }}
            >
              <Box>
                <Box
                  component="img"
                  src={emptyImg}
                  alt="No gateways"
                  sx={{ width: 60, height: 60, mx: "auto" }}
                />
                <Typography
                  variant="body2"
                  color="#6a6969ff"
                  sx={{ mb: 1.5 }}
                >
                  No Gateways available
                </Typography>
                <Button
                  size="small"
                  variant="outlined"
                  onClick={(e: any) => {
                    e.stopPropagation();
                    navigate(href);
                  }}
                >
                  Add Gateways
                </Button>
              </Box>
            </Box>
          ) : (
            <>
              {/* Top stats: Active / Inactive — reduced height */}
              <Stack direction="row" spacing={2} sx={{ mt: 0.5 }}>
                {/* Active */}
                <Box
                  sx={(t) => ({
                    flex: 1,
                    px: 1.25,
                    py: 1,
                    borderRadius: 2,
                    border: `1px solid ${t.palette.success.dark}`,
                    bgcolor:
                      t.palette.mode === "dark"
                        ? t.palette.success.dark + "22"
                        : t.palette.success.light + "33",
                    display: "grid",
                    placeItems: "center",
                  })}
                >
                  <Typography
                    component="div"
                    sx={{ fontSize: 30, fontWeight: 800, lineHeight: 1 }}
                    color="success.main"
                  >
                    {String(activeCount).padStart(2, "0")}
                  </Typography>
                  <Typography
                    variant="body2"
                    sx={{
                      mt: 0.25,
                      textAlign: "center",
                      color: "success.dark",
                    }}
                  >
                    Active gateways
                  </Typography>
                </Box>

                {/* Inactive */}
                <Box
                  sx={(t) => ({
                    flex: 1,
                    px: 1.25,
                    py: 1,
                    borderRadius: 2,
                    border: `1px solid ${
                      hasInactive ? t.palette.error.dark : t.palette.divider
                    }`,
                    bgcolor: hasInactive
                      ? t.palette.mode === "dark"
                        ? t.palette.error.dark + "22"
                        : t.palette.error.light + "33"
                      : t.palette.action.hover,
                    display: "grid",
                    placeItems: "center",
                  })}
                >
                  <Typography
                    component="div"
                    sx={(t) => ({
                      fontSize: 30,
                      fontWeight: 800,
                      lineHeight: 1,
                      color: hasInactive
                        ? t.palette.error.main
                        : t.palette.text.secondary,
                    })}
                  >
                    {String(inactiveCount).padStart(2, "0")}
                  </Typography>

                  <Typography
                    variant="body2"
                    sx={(t) => ({
                      mt: 0.25,
                      textAlign: "center",
                      color: hasInactive
                        ? t.palette.error.dark
                        : t.palette.text.secondary,
                    })}
                  >
                    Inactive gateways
                  </Typography>
                </Box>
              </Stack>

              {/* Gateway Types */}
              <Typography
                variant="caption"
                color="#787575ff"
                sx={{ mt: 1 }}
              >
                Gateway Types
              </Typography>

              {/* Single row with 4 equal columns */}
              <Box
                sx={(t) => ({
                  display: "grid",
                  gridTemplateColumns: "repeat(4, 1fr)",
                  gap: 1,
                })}
              >
                {[
                  { label: "Regular", value: regularCount },
                  { label: "AI", value: aiCount },
                  { label: "MCP", value: mcpCount },
                  { label: "Event", value: eventCount },
                ].map((it) => (
                  <Box
                    key={it.label}
                    sx={{
                      display: "inline-flex",
                      alignItems: "center",
                      justifyContent: "space-between",
                      gap: 1,
                      px: 1.25,
                      py: 0.75,
                      borderRadius: 1.5,
                      border: (t) => `1px solid ${t.palette.divider}`,
                      bgcolor: (t) => t.palette.action.hover,
                      width: "100%",
                    }}
                  >
                    <Typography variant="body2">{it.label}</Typography>
                    <Typography variant="body2" fontWeight={700}>
                      {String(it.value).padStart(2, "0")}
                    </Typography>
                  </Box>
                ))}
              </Box>
            </>
          )}
        </CardContent>
      </CardActionArea>
    </Card>
  );
}
