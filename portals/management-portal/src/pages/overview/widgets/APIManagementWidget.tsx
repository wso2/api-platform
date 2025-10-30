// portals/management-portal/src/pages/overview/widgets/APIManagementWidget.tsx
import {
  Card,
  CardContent,
  Box,
  Typography,
  IconButton,
  ButtonBase,
} from "@mui/material";
import ChevronRightRoundedIcon from "@mui/icons-material/ChevronRightRounded";
import { useNavigate } from "react-router-dom";
import { useApisContext } from "../../../context/ApiContext";

// images
import mcpSvg from "./MCPIcon.svg";
import fromMarketplaceSvg from "./FromMarketplace.svg";   // ← API Products
import withEndpointSvg from "./WithEndpoint.svg";          // ← APIs

type Props = { height?: number; href: string };

const format2 = (v: number | string) =>
  typeof v === "number" ? String(v).padStart(2, "0") : v;

function RowLink({
  imgSrc,
  label,
  value,
  to,
  onClick,
}: {
  imgSrc: string;
  label: string;
  value: number | string;
  to: string;
  onClick: (to: string) => void;
}) {
  return (
    <ButtonBase
      onClick={() => onClick(to)}
      sx={(t) => ({
        width: "100%",
        borderRadius: 2,
        textAlign: "left",
        border: `1px solid ${t.palette.divider}`,
        bgcolor: t.palette.action.hover,
        px: 1.5,
        py: 1,
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: 1.5,
      })}
    >
      {/* left: image + label */}
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.25, minWidth: 0 }}>
        <Box component="img" src={imgSrc} alt="" sx={{ width: 28, height: 28, flex: "0 0 auto" }} />
        <Typography variant="subtitle1" sx={{ color: "text.secondary" }} noWrap fontWeight={600}>
          {label}
        </Typography>
      </Box>

      {/* right: count + chevron */}
      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
        <Typography sx={{ fontSize: 24, fontWeight: 800, lineHeight: 1 }}>
          {format2(value)}
        </Typography>
        <IconButton
          size="small"
          onClick={(e) => {
            e.stopPropagation();
            onClick(to);
          }}
        >
          <ChevronRightRoundedIcon fontSize="small" />
        </IconButton>
      </Box>
    </ButtonBase>
  );
}

export default function APIManagementWidget({ height = 220, href }: Props) {
  const navigate = useNavigate();
  const { apis, loading } = useApisContext();
  const apiCount = apis?.length ?? 0;

  // Build per-destination hrefs based on the base path
  const prefix = href.endsWith("/apis") ? href.slice(0, -"/apis".length) : href;
  const apiHref = `${prefix}/apis`;
  const mcpHref = `${prefix}/mcp`;
  const productHref = `${prefix}/api-products`;

  const go = (to: string) => navigate(to);

  return (
    <Card variant="outlined" sx={{ height }}>
      <CardContent sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        <Typography variant="subtitle1" fontWeight={700}>
          API Management
        </Typography>

        {/* APIs -> WithEndpoint.svg */}
        <RowLink
          imgSrc={withEndpointSvg}
          label="APIs"
          value={loading ? "…" : apiCount}
          to={apiHref}
          onClick={go}
        />

        {/* MCP -> mcp.svg */}
        <RowLink imgSrc={mcpSvg} label="MCP" value={"00"} to={mcpHref} onClick={go} />

        {/* API Products -> FromMarketplace.svg */}
        <RowLink
          imgSrc={fromMarketplaceSvg}
          label="API Products"
          value={"00"}
          to={productHref}
          onClick={go}
        />
      </CardContent>
    </Card>
  );
}
