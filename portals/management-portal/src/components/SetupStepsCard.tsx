import * as React from "react";
import { Box, Card, Typography, Stack } from "@mui/material";
import { alpha, useTheme } from "@mui/material/styles";

type Step = { title: string; subtitle: string };

type Props = {
  title: string;
  steps: Step[];
  imageSrc?: string;
};

const SetupStepsCard: React.FC<Props> = ({ title, steps, imageSrc }) => {
  const theme = useTheme();
  const ACCENT = theme.palette.primary.main;

  return (
    <Card
      sx={{
        borderRadius: 3,
        overflow: "hidden",
        border: "1px solid",
        borderColor: "divider",
        height: "100%",
        display: "flex",
        flexDirection: "column",
      }}
    >
      <Box px={3} pt={3}>
        <Typography variant="subtitle2" sx={{ color: "#AEAEAE" }}>
          {title}
        </Typography>
      </Box>
      <Box
        mx={3}
        mt={2}
        borderRadius={3}
        sx={{
          position: "relative",
          overflow: "hidden",
          height: { xs: 180, sm: 200, md: 220 },
        }}
      >
        {imageSrc && (
          <Box
            component="img"
            src={imageSrc}
            alt={title}
            sx={{
              width: "100%",
              height: "100%",
              display: "block",
              objectFit: "cover",
            }}
          />
        )}
      </Box>

      <Box px={3} py={3} flexGrow={1}>
        <Typography fontWeight={600}>Setup Steps for {title}</Typography>
        <Stack spacing={2} mt={2}>
          {steps.map((step, index) => (
            <Stack key={`${index}-${step.title}`} direction="row" spacing={2} alignItems="flex-start">
              <Box
                width={32}
                height={32}
                borderRadius="50%"
                bgcolor={alpha(ACCENT, 0.15)}
                color={ACCENT}
                display="flex"
                alignItems="center"
                justifyContent="center"
                fontWeight={700}
              >
                {index + 1}
              </Box>
              <Box>
                <Typography fontWeight={600}>{step.title}</Typography>
                <Typography variant="body2" sx={{ color: "#AEAEAE" }}>
                  {step.subtitle}
                </Typography>
              </Box>
            </Stack>
          ))}
        </Stack>
      </Box>
    </Card>
  );
};

export default SetupStepsCard;
