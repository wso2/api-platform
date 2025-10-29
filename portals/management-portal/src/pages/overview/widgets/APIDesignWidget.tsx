import { Box, Card, CardContent, Divider, Link as MuiLink, Stack, Typography } from "@mui/material";
import { Button } from "../../../components/src/components/Button";

type Props = { height?: number };

export default function APIDesignWidget({ height = 220 }: Props) {
  return (
    <Card variant="outlined" sx={{ height }}>
      <CardContent sx={{ height: "100%", display: "flex", flexDirection: "column" }}>
        <Typography variant="subtitle1" fontWeight={700}>
          API Design
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
          Use your editor to scaffold, lint, and version OpenAPI specs.
        </Typography>

        <Box sx={{ flex: 1 }} />

        <Divider sx={{ my: 1.5 }} />

        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Typography variant="body2" color="text.secondary">
            Open in VS Code
          </Typography>
          <Button size="small" variant="contained" component={MuiLink} underline="none" href="vscode://">
            Open VS Code
          </Button>
        </Stack>
      </CardContent>
    </Card>
  );
}
