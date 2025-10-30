import { Box, Card, CardContent, Divider, Stack, Typography } from "@mui/material";
import { Button } from "../../../components/src/components/Button";
import { useNavigate } from "react-router-dom";

type Props = { height?: number; href: string };

export default function EnterprisePortalWidget({ height = 220, href }: Props) {
  const navigate = useNavigate();
  return (
    <Card variant="outlined" sx={{ height }}>
      <CardContent sx={{ height: "100%", display: "flex", flexDirection: "column" }}>
        <Typography variant="subtitle1" fontWeight={700}>
          Enterprise Portal
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
          Centralize developer onboarding, docs, and governance for your org.
        </Typography>

        <Box sx={{ flex: 1 }} />

        <Divider sx={{ my: 1.5 }} />
        <Stack direction="row" justifyContent="flex-start">
          <Button size="small" variant="contained" onClick={() => navigate(href)}>
            Open
          </Button>
        </Stack>
      </CardContent>
    </Card>
  );
}
