import * as React from "react";
import { Box, Stack, Typography, Card } from "@mui/material";
import { Button } from "../../components/src/components/Button";
import SetupStepsCard from "../../components/SetupStepsCard";
import { SCENARIOS } from "../../data/scenarios";

type Props = {
  onBackToChoices: () => void;
  onSkip: () => void;
};

const DesignApiWizard: React.FC<Props> = ({ onBackToChoices, onSkip }) => {
  const scenario = SCENARIOS.find((s) => s.id === "design-api");

  return (
    <Box px={{ xs: 2, md: 0 }}>
      <Box maxWidth={1240} mx="auto" mb={3}>
        <Stack
          direction={{ xs: "column", sm: "row" }}
          alignItems={{ xs: "flex-start", sm: "center" }}
          justifyContent="space-between"
          spacing={2}
          mb={2}
        >
          <Box>
            <Typography variant="h4" fontWeight={700}>
              Design an API
            </Typography>
            <Typography color="#AEAEAE">
              Use the WSO2 API Designer in VS Code to build and document APIs fast.
            </Typography>
          </Box>
          <Stack direction="row" spacing={1}>
            <Button variant="text" onClick={onBackToChoices}>
              Back to choices
            </Button>
            <Button variant="outlined" onClick={onSkip}>
              Skip for now
            </Button>
          </Stack>
        </Stack>

        <Card
          elevation={0}
          sx={{
            maxWidth: 1100,
            mx: "auto",
            p: { xs: 3, md: 4 },
            borderRadius: 3,
            border: "1px solid",
            borderColor: "divider",
            display: "flex",
            gap: 4,
            alignItems: "stretch",
            flexDirection: "column",
            bgcolor: (t) => (t.palette.mode === "dark" ? "background.paper" : "#fff"),
          }}
        >
          <Box>
            {scenario && (
              <SetupStepsCard
                title={scenario.title}
                steps={scenario.steps}
                imageSrc={scenario.imageSrc}
              />
            )}
          </Box>

          <Box mt={2} sx={{ display: "flex", justifyContent: { xs: "center", md: "flex-end" } }}>
            <Button
              component={"a" as any}
              href={"vscode://"}
              variant="contained"
              size="large"
            >
              Open VS Code
            </Button>
          </Box>
        </Card>
      </Box>
    </Box>
  );
};

export default DesignApiWizard;
