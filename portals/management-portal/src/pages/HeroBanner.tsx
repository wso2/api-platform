// src/pages/overview/HeroBanner.tsx
import React from "react";
import { Box, Stack, Typography } from "@mui/material";
import { alpha } from "@mui/material/styles";
import { Button } from "../components/src/components/Button";

export type BannerSlide = {
  id: string;
  title: string;
  subtitle?: string;
  imageUrl?: string; // full-height background on the right
  imageNode?: React.ReactNode; // or pass a node; forced to full height
};

type Props = {
  slides?: BannerSlide[];
  intervalMs?: number; // default 5000
  height?: number; // default 180
  onStart?: () => void;
  rightWidthPct?: number; // default 40 (% width for right image)
};

const DEFAULT_SLIDES: BannerSlide[] = [
  {
    id: "step-1",
    title: "Create a Gateway in minutes",
    subtitle:
      "Spin up a Hybrid or Cloud gateway and start proxying traffic with a single command.",
  },
  {
    id: "step-2",
    title: "Import and discover your APIs",
    subtitle:
      "Push your OpenAPI / AsyncAPI definition and curate them with tags, versions, and contexts.",
  },
  {
    id: "step-3",
    title: "Validate & monitor in one place",
    subtitle:
      "Run smoke tests and observe latency, errors, and throughput—before and after deploy.",
  },
];

const STEP_LABELS = [
  "1. Add Your Gateway",
  "2. Add Your APIs",
  "3. Test Your APIs",
];

const HeroBanner: React.FC<Props> = ({
  slides = DEFAULT_SLIDES,
  intervalMs = 5000,
  height = 180,
  onStart,
  rightWidthPct = 40,
}) => {
  const [index, setIndex] = React.useState(0);

  React.useEffect(() => {
    if (slides.length <= 1) return;
    const id = setInterval(
      () => setIndex((i) => (i + 1) % slides.length),
      intervalMs
    );
    return () => clearInterval(id);
  }, [intervalMs, slides.length]);

  const slide = slides[index];
  const rightWidth = `${rightWidthPct}%`;

  return (
    <Box
      sx={{
        position: "relative",
        overflow: "hidden",
        height,
        borderRadius: 2,
        border: "0.5px solid",
        borderColor: "#10B981",
        backgroundImage:
          "linear-gradient(90deg, #E1F8C7 0%, #FFFFFF 55%, #F6FAE9 100%)",
        px: { xs: 2, sm: 3 },
        py: { xs: 2, sm: 2.5 },
        display: "flex",
        alignItems: "stretch",
        gap: 2,
      }}
    >
      {/* RIGHT: image as absolute background so it fills FULL height (ignores px/py) */}
      {(slide.imageUrl || slide.imageNode) && (
        <>
          {slide.imageUrl ? (
            <Box
              aria-hidden
              sx={{
                position: "absolute",
                top: 0,
                right: 0,
                bottom: 0,
                width: { xs: 0, sm: rightWidth },
                backgroundImage: `url(${slide.imageUrl})`,
                backgroundRepeat: "no-repeat",
                backgroundPosition: "right center",
                backgroundSize: "auto 100%", // always full height
                pointerEvents: "none",
              }}
            />
          ) : (
            <Box
              aria-hidden
              sx={{
                position: "absolute",
                top: 0,
                right: 0,
                bottom: 0,
                width: { xs: 0, sm: rightWidth },
                display: { xs: "none", sm: "flex" },
                alignItems: "center",
                justifyContent: "flex-end",
                pr: 1,
                "& img, & svg": {
                  height: "100%",
                  width: "auto",
                  display: "block",
                },
                pointerEvents: "none",
              }}
            >
              {slide.imageNode}
            </Box>
          )}
        </>
      )}

      {/* LEFT: Title → Stepper → Subtitle → CTA (reserve space for right image) */}
      <Box
        sx={{
          flex: 1,
          minWidth: 0,
          pr: { xs: 0, sm: rightWidth }, // keep text clear of the image
          display: "flex",
          flexDirection: "column",
          justifyContent: "center",
          gap: 1,
        }}
      >
        <Box display={"flex"} flexDirection="column" gap={0.5}>
          <Typography
            variant="h6"
            sx={{
              fontWeight: 800,
            }}
          >
            Create Your First API
          </Typography>
          <Box
            sx={(t) => ({
              display: "inline-flex",
              alignItems: "center",
              gap: 1.75,
              px: 2,
              py: 0.75,
              borderRadius: 1,
              bgcolor: alpha(t.palette.text.primary, 0.04),
              boxShadow: `inset 0 0 0 1px ${alpha(
                t.palette.text.primary,
                0.06
              )}`,
              maxWidth: 480,
            })}
          >
            {STEP_LABELS.map((label, i) => {
              const active = i === index;
              return (
                <React.Fragment key={label}>
                  <Typography
                    sx={(t) => ({
                      fontSize: 14,
                      fontWeight: active ? 700 : 600,
                      color: active
                        ? t.palette.text.primary
                        : alpha(t.palette.text.primary, 0.45),
                      whiteSpace: "nowrap",
                    })}
                  >
                    {label}
                  </Typography>
                  {i < STEP_LABELS.length - 1 && (
                    <Typography
                      component="span"
                      sx={(t) => ({
                        mx: 0.25,
                        fontSize: 18,
                        lineHeight: 1,
                        fontWeight: 800,
                        color: t.palette.text.primary,
                      })}
                    >
                      ›
                    </Typography>
                  )}
                </React.Fragment>
              );
            })}
          </Box>
        </Box>

        {/* Subtitle + CTA */}
        {slide.subtitle && (
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{ mt: 0.25, maxWidth: 720, position: "relative", zIndex: 1 }}
          >
            {slide.subtitle}
          </Typography>
        )}

        <Stack direction="row" spacing={1.5} sx={{ mt: 1.25 }}>
          <Button
            size="small"
            variant="contained"
            onClick={onStart}
            style={{ backgroundColor: "#059669", borderColor: "#059669" }}
          >
            Get Started
          </Button>
        </Stack>
      </Box>
    </Box>
  );
};

export default HeroBanner;
