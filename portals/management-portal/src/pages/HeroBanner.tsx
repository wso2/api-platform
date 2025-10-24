// src/pages/overview/HeroBanner.tsx
import React from "react";
import { Box, Paper, Stack, Typography } from "@mui/material";
import { keyframes } from "@emotion/react";
import { Button } from "../components/src/components/Button";

export type BannerSlide = {
  id: string;
  title: string;
  subtitle?: string;
  ctaLabel?: string;
  onCtaClick?: () => void;
  imageUrl?: string;
  imageNode?: React.ReactNode;
  tag?: string;
};

type Props = {
  slides: BannerSlide[];
  intervalMs?: number; // default 5000
  height?: number;     // default 180
};

// Only animate the Paper: simple fade/slide-in
const slideFadeIn = keyframes`
  0%   { opacity: 0; transform: translateY(8px); }
  100% { opacity: 1; transform: translateY(0); }
`;

const HeroBanner: React.FC<Props> = ({
  slides,
  intervalMs = 5000,
  height = 180,
}) => {
  const [index, setIndex] = React.useState(0);

  // Auto-advance (no other animations)
  React.useEffect(() => {
    if (slides.length <= 1) return;
    const id = setInterval(
      () => setIndex((i) => (i + 1) % slides.length),
      intervalMs
    );
    return () => clearInterval(id);
  }, [intervalMs, slides.length]);

  const slide = slides[index];

  return (
    <Paper
      key={slide.id} // re-run Paper fade each slide
      elevation={0}
      sx={{
        position: "relative",
        overflow: "hidden",
        height,
        borderRadius: 2,
        border: "0.5px solid",
        borderColor: "#10B981",
        // STATIC gradient (no animation)
        backgroundImage:
          "linear-gradient(90deg, #e1f8c7ff 0%, #FFFFFF 55%, #F6FAE9 100%)",
        // Only Paper animates
        animation: `${slideFadeIn} 420ms ease-out`,
        px: { xs: 2, sm: 3 },
        py: { xs: 2, sm: 2.5 },
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          height: "100%",
          gap: 2,
        }}
      >
        {/* Left: text */}
        <Box sx={{ flex: 1, minWidth: 0, pr: 2 }}>
          {slide.tag && (
            <Box
              component="span"
              sx={{
                display: "inline-block",
                px: 1,
                py: 0.25,
                borderRadius: 1,
                fontSize: 12,
                fontWeight: 600,
                bgcolor: "rgba(0,0,0,0.06)",
                color: "text.primary",
                mb: 1,
              }}
            >
              {slide.tag}
            </Box>
          )}
          <Typography variant="h6" sx={{ fontWeight: 800, lineHeight: 1.2 }} noWrap>
            {slide.title}
          </Typography>
          {slide.subtitle && (
            <Typography
              variant="body2"
              color="text.secondary"
              sx={{ mt: 0.75, maxWidth: 720 }}
            >
              {slide.subtitle}
            </Typography>
          )}

          {slide.ctaLabel && (
            <Stack direction="row" spacing={1.5} sx={{ mt: 1.5 }}>
              <Button variant="contained" onClick={slide.onCtaClick} style={{backgroundColor: '#059669', borderColor: '#059669'}}>
                {slide.ctaLabel}
              </Button>
            </Stack>
          )}
        </Box>

        {/* Right: static artwork (no animation) */}
        {(slide.imageNode || slide.imageUrl) && (
          <Box
            sx={{
              flex: 0.6,
              display: { xs: "none", sm: "flex" },
              justifyContent: "flex-end",
            }}
          >
            {slide.imageNode ? (
              <Box>{slide.imageNode}</Box>
            ) : (
              <Box
                component="img"
                src={slide.imageUrl}
                alt=""
                sx={{ maxHeight: height - 24, objectFit: "contain" }}
              />
            )}
          </Box>
        )}
      </Box>
    </Paper>
  );
};

export default HeroBanner;
