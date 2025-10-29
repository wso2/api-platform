import Box from "@mui/material/Box";
import APIDesignWidget from "./widgets/APIDesignWidget";
import APIGatewayWidget from "./widgets/APIGatewayWidget";
import APIPortalWidget from "./widgets/APIPortalWidget";
import APIAnalyticsWidget from "./widgets/APIAnalyticsWidget";
import APIManagementWidget from "./widgets/APIManagementWidget";
import EnterprisePortalWidget from "./widgets/EnterprisePortalWidget";
import { Grid } from "@mui/material";

type Portal = { id: string; name: string; href: string };
type Props = {
  orgHandle: string;
  projectSlug: string;
  portals?: Portal[];
};

const CARD_HEIGHT = 220;

export default function ProjectFeatureCards({ orgHandle, projectSlug, portals = [] }: Props) {
  const base = `/${orgHandle}/${projectSlug}`;

  return (
    <Box sx={{ mt: 3 }}>
      <Grid container spacing={2} columns={12}>
        {/* 1) API Design */}
        <Grid  size={{ xs: 12, md: 4 }}>
          <APIDesignWidget height={CARD_HEIGHT} />
        </Grid>

        {/* 2) API Gateway */}
        <Grid size={{ xs: 12, md: 4 }}>
          <APIGatewayWidget height={CARD_HEIGHT} href={`${base}/gateway`} />
        </Grid>

        {/* 3) API Portal */}
        <Grid size={{ xs: 12, md: 4 }}>
          <APIPortalWidget height={CARD_HEIGHT} href={`${base}/portals`} portals={portals} />
        </Grid>

        {/* 4) API Analytics */}
        <Grid size={{ xs: 12, md: 4 }}>
          <APIAnalyticsWidget height={CARD_HEIGHT} href={`${base}/analytics`} />
        </Grid>

        {/* 5) API Management */}
        <Grid size={{ xs: 12, md: 4 }}>
          <APIManagementWidget height={CARD_HEIGHT} href={`${base}/apis`} />
        </Grid>

        {/* 6) Enterprise Portal */}
        <Grid size={{ xs: 12, md: 4 }}>
          <EnterprisePortalWidget height={CARD_HEIGHT} href={`${base}/enterprise-portal`} />
        </Grid>
      </Grid>
    </Box>
  );
}
