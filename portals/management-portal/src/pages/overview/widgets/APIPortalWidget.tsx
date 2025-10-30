import { Box, Card, CardActionArea, CardContent, Divider, Link as MuiLink, Stack, Typography } from "@mui/material";
import { useNavigate } from "react-router-dom";

type Portal = { id: string; name: string; href: string };
type Props = { height?: number; href: string; portals?: Portal[] };

export default function APIPortalWidget({ height = 220, href, portals = [] }: Props) {
  const navigate = useNavigate();
  const recent = portals.slice(0, 3);

  return (
    <Card variant="outlined" sx={{ height }}>
      <CardActionArea sx={{ height: "100%" }} onClick={() => navigate(href)}>
        <CardContent sx={{ height: "100%", display: "flex", flexDirection: "column" }}>
          <Typography variant="subtitle1" fontWeight={700}>
            API Portal
          </Typography>

          <Box
            sx={(t) => ({
              mt: 1.5,
              borderRadius: 2,
              flex: 1,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              border: `1px dashed ${t.palette.divider}`,
              bgcolor: t.palette.action.hover,
              px: 1,
              width: "100%",
            })}
          >
            {recent.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                No portals configured.
              </Typography>
            ) : (
              <Stack sx={{ width: "100%" }}>
                {recent.map((p) => (
                  <Stack
                    key={p.id}
                    direction="row"
                    alignItems="center"
                    justifyContent="space-between"
                    sx={{ py: 0.5 }}
                  >
                    <Typography variant="body2">{p.name}</Typography>
                    <MuiLink
                      variant="body2"
                      href={p.href}
                      target="_blank"
                      rel="noreferrer"
                      underline="hover"
                      onClick={(e) => e.stopPropagation()}
                    >
                      View →
                    </MuiLink>
                  </Stack>
                ))}
              </Stack>
            )}
          </Box>

          <Divider sx={{ mt: 1.5 }} />
          <Typography variant="body2" color="primary.main" sx={{ mt: 1 }}>
            Manage →
          </Typography>
        </CardContent>
      </CardActionArea>
    </Card>
  );
}
