import { Box, Card, CardActionArea, CardContent, Divider, Typography, useTheme } from "@mui/material";
import { useNavigate } from "react-router-dom";

type Props = { height?: number; href: string };

type Point = { label: string; value: number };

// sample data (replace with real data when ready)
const DATA: Point[] = [
  { label: "Page A", value: 4000 },
  { label: "Page B", value: 3000 },
  { label: "Page C", value: 2000 },
  { label: "Page D", value: 2800 },
  { label: "Page E", value: 1890 },
  { label: "Page F", value: 2400 },
  { label: "Page G", value: 3500 },
];

/** Catmull-Rom -> cubic Bézier conversion for a smooth path */
function smoothPath(points: { x: number; y: number }[]) {
  if (points.length < 2) return "";
  const d: string[] = [`M ${points[0].x},${points[0].y}`];

  // helper to get point safely
  const p = (i: number) => points[Math.max(0, Math.min(points.length - 1, i))];

  for (let i = 0; i < points.length - 1; i++) {
    const p0 = p(i - 1);
    const p1 = p(i);
    const p2 = p(i + 1);
    const p3 = p(i + 2);

    // Catmull–Rom to Bezier
    const cp1x = p1.x + (p2.x - p0.x) / 6;
    const cp1y = p1.y + (p2.y - p0.y) / 6;
    const cp2x = p2.x - (p3.x - p1.x) / 6;
    const cp2y = p2.y - (p3.y - p1.y) / 6;

    d.push(`C ${cp1x},${cp1y} ${cp2x},${cp2y} ${p2.x},${p2.y}`);
  }
  return d.join(" ");
}

export default function APIAnalyticsWidget({ height = 220, href }: Props) {
  const navigate = useNavigate();
  const theme = useTheme();

  // layout paddings inside the SVG
  const PAD_LEFT = 36;
  const PAD_RIGHT = 10;
  const PAD_TOP = 8;
  const PAD_BOTTOM = 24;

  // scales
  const values = DATA.map((d) => d.value);
  const minY = 0; // baseline like the screenshot
  const maxY = Math.max(...values) * 1.02;

  const toX = (i: number, w: number) =>
    PAD_LEFT + (i * (w - PAD_LEFT - PAD_RIGHT)) / (DATA.length - 1);

  const toY = (v: number, h: number) =>
    PAD_TOP + (1 - (v - minY) / (maxY - minY)) * (h - PAD_TOP - PAD_BOTTOM);

  return (
    <Card variant="outlined" sx={{ height }}>
      <CardActionArea sx={{ height: "100%" }} onClick={() => navigate(href)}>
        <CardContent sx={{ height: "100%", display: "flex", flexDirection: "column" }}>
          <Typography variant="subtitle1" fontWeight={700}>
            API Analytics
          </Typography>

          <Box sx={{ mt: 1.5, flex: 1 }}>
            <Box
              component="svg"
              viewBox="0 0 1000 320"
              width="100%"
              height="100%"
              sx={{ display: "block" }}
            >
              {/* grid */}
              {/** horizontal grid lines */}
              {[0, 0.25, 0.5, 0.75, 1].map((t) => {
                const y = PAD_TOP + (1 - t) * (320 - PAD_TOP - PAD_BOTTOM);
                return (
                  <line
                    key={`h-${t}`}
                    x1={PAD_LEFT}
                    x2={1000 - PAD_RIGHT}
                    y1={y}
                    y2={y}
                    stroke={theme.palette.divider}
                    strokeDasharray="4 6"
                    strokeWidth={1}
                  />
                );
              })}

              {/** vertical grid lines */}
              {DATA.map((_, i) => {
                const x = toX(i, 1000);
                return (
                  <line
                    key={`v-${i}`}
                    x1={x}
                    x2={x}
                    y1={PAD_TOP}
                    y2={320 - PAD_BOTTOM}
                    stroke={theme.palette.divider}
                    strokeDasharray="4 6"
                    strokeWidth={1}
                  />
                );
              })}

              {/* axes labels (x) */}
              {DATA.map((d, i) => {
                const x = toX(i, 1000);
                return (
                  <text
                    key={`label-${i}`}
                    x={x}
                    y={320 - 6}
                    textAnchor="middle"
                    fontSize="24"
                    fill={theme.palette.text.secondary}
                  >
                    {d.label}
                  </text>
                );
              })}

              {/* y-axis ticks (0 to max) */}
              {[0, 0.25, 0.5, 0.75, 1].map((t, idx) => {
                const value = Math.round(minY + t * (maxY - minY));
                const y = PAD_TOP + (1 - t) * (320 - PAD_TOP - PAD_BOTTOM);
                return (
                  <text
                    key={`yt-${idx}`}
                    x={6}
                    y={y + 6}
                    fontSize="24"
                    fill={theme.palette.text.secondary}
                  >
                    {value}
                  </text>
                );
              })}

              {/* area + stroke */}
              {(() => {
                const pts = DATA.map((d, i) => ({
                  x: toX(i, 1000),
                  y: toY(d.value, 320),
                }));
                const curve = smoothPath(pts);

                // area path to baseline
                const areaD = `${curve} L ${pts[pts.length - 1].x},${320 - PAD_BOTTOM} L ${pts[0].x},${320 - PAD_BOTTOM} Z`;

                // colors
                const stroke = theme.palette.primary.main;
                const fill = theme.palette.mode === "dark"
                  ? `${stroke}55`
                  : `${stroke}33`;

                return (
                  <>
                    <path d={areaD} fill={fill} />
                    <path d={curve} fill="none" stroke={stroke} strokeWidth={4} />
                  </>
                );
              })()}
            </Box>
          </Box>
        </CardContent>
      </CardActionArea>
    </Card>
  );
}
