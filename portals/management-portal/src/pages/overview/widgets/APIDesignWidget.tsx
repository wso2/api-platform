// portals/management-portal/src/pages/overview/widgets/APIDesignWidget.tsx
import {
  Box,
  Card,
  CardContent,
  Divider,
  Link as MuiLink,
  Stack,
  Typography,
} from "@mui/material";
import { Button } from "../../../components/src/components/Button";

// VS Code SVG (used as the button's start icon)
import vscodeSvg from "../../../components/src/Images/svgs/Various/vs-code.svg";

type Props = { height?: number };

export default function APIDesignWidget({ height = 220 }: Props) {
  return (
    <Card variant="outlined" sx={{ height }}>
      <CardContent
        sx={{ height: "100%", display: "flex", flexDirection: "column" }}
      >
        <Typography variant="subtitle1" fontWeight={700}>
          API Design
        </Typography>
        {/* 
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
          Get the VS Code extension and start designing instantly.
        </Typography> */}

        {/* Steps */}
        <Box
          component="ol"
          sx={{
            mt: 1.25,
            pl: 2.25,
            mb: 0.5,
            pr: 0.5,
            "& li": { mb: 0.75 },
          }}
        >
          <li>
            <Typography variant="body2">
              Open VS Code and go to <strong>Extensions</strong>.
            </Typography>
          </li>
          <li>
            <Typography variant="body2">
              Search for <strong>wso2-api-designer</strong> and install it.
            </Typography>
          </li>
          <li>
            <Typography variant="body2">
              Open your OpenAPI file, click <strong>API Designer</strong>, and
              start designing.
            </Typography>
          </li>
        </Box>
        <Stack
          direction="row"
          justifyContent="space-between"
          alignItems="center"
        >
          <Button
            size="small"
            variant="contained"
            component={MuiLink}
            underline="none"
            href="vscode://"
            // start icon from the imported svg
            startIcon={
              <Box
                component="img"
                src={vscodeSvg}
                alt=""
                sx={{
                  width: 16,
                  height: 16,
                  display: "block",
                  filter: "brightness(0) invert(1)", // makes it white
                }}
              />
            }
          >
            Open VS Code
          </Button>
        </Stack>
      </CardContent>
    </Card>
  );
}
