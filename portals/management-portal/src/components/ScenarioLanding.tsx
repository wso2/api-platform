import React from "react";
import {
  Avatar,
  Box,
  Card,
  Chip,
  Radio,
  Stack,
  Typography,
} from "@mui/material";
import { alpha, useTheme } from "@mui/material/styles";
import ChevronRightRoundedIcon from "@mui/icons-material/ChevronRightRounded";
import { useOrganization } from "../context/OrganizationContext";
import { SCENARIOS, type Scenario } from "../data/scenarios";
import { Button } from "./src/components/Button";
import vscodeSvg from "../../src/components/src/Images/svgs/Various/vs-code.svg";
import { Link as MuiLink } from "@mui/material";

const ACCENT = "#049669";
const TEXT_SECONDARY = "#AEAEAE";

type ScenarioLandingProps = {
  onContinue: (scenarioId: string) => void;
  onSkip: () => void;
};

const CARD_SHADOW =
  "0px 24px 80px rgba(15, 23, 42, 0.08), 0px 2px 6px rgba(15, 23, 42, 0.04)";

const ScenarioLanding: React.FC<ScenarioLandingProps> = ({
  onContinue,
  onSkip,
}) => {
  const theme = useTheme();
  const { organization } = useOrganization();
  const displayName = organization?.name || "there";

  const [activeScenarioId, setActiveScenarioId] = React.useState(
    SCENARIOS[0].id
  );

  const activeScenario = React.useMemo(
    () =>
      SCENARIOS.find((item) => item.id === activeScenarioId) ?? SCENARIOS[0],
    [activeScenarioId]
  );

  const selectScenario = (scenarioId: string) => {
    setActiveScenarioId(scenarioId);
  };

  return (
    <Box>

      {/* ===== MAIN CARD CONTENT ===== */}
      <Card
        elevation={0}
        sx={{
          p: { xs: 3, md: 4 },
          mb: 4,
          mt: 2,
          borderRadius: 4,
          border: "1px solid",
          borderColor: "divider",
          boxShadow: CARD_SHADOW,
          bgcolor: (t) =>
            t.palette.mode === "dark" ? "background.default" : "#fffdfa",
        }}
      >
        <Typography variant="h3" fontWeight={600} mt={0.5}>
          {displayName !== "default" ? (
            <>
              <Box component="span" fontWeight={800}>
                Hi {displayName},
              </Box>{' '}
            </>
          ) : null}
          Welcome to the API Platform
        </Typography>

        <Typography sx={{ color: TEXT_SECONDARY }}>
          <Box
            component="span"
            sx={{ fontWeight: 600, fontSize: { xs: 12, sm: 14, md: 16 } }}
          >
            What would you like to do today?
          </Box>{" "}
          Pick one or more journeys to get a head start.
        </Typography>

        <Stack
          direction={{ xs: "column", lg: "row" }}
          spacing={4}
          mt={4}
          alignItems="stretch"
        >
          {/* Left column - scenarios */}
          <Box flex={{ xs: "auto", lg: 1.2 }}>
            <Stack spacing={2}>
              {SCENARIOS.map((scenario: Scenario) => {
                const Icon = scenario.icon;
                const isActive = activeScenarioId === scenario.id;

                return (
                  <Card
                    key={scenario.id}
                    onClick={() => !scenario.comingSoon && selectScenario(scenario.id)}
                    sx={{
                      cursor: scenario.comingSoon ? "not-allowed" : "pointer",
                      border: "1px solid",
                      borderColor: isActive ? ACCENT : "divider",
                      bgcolor: isActive
                        ? alpha(ACCENT, 0.1)
                        : "background.paper",
                      transition:
                        "border-color 150ms ease, box-shadow 150ms ease",
                      boxShadow: isActive ? CARD_SHADOW : "none",
                      opacity: scenario.comingSoon ? 0.6 : 1,
                      pointerEvents: scenario.comingSoon ? "none" : "auto",
                    }}
                  >
                    <Stack
                      direction="row"
                      alignItems="center"
                      spacing={2}
                      px={2}
                      py={2.25}
                    >
                      <Radio
                        checked={activeScenarioId === scenario.id}
                        tabIndex={-1}
                        onClick={(event) => {
                          event.stopPropagation();
                          setActiveScenarioId(scenario.id);
                        }}
                        sx={{
                          color: ACCENT,
                          "&.Mui-checked": { color: ACCENT },
                        }}
                      />

                      <Avatar
                        variant="rounded"
                        sx={{
                          bgcolor: alpha(ACCENT, 0.15),
                          color: ACCENT,
                          width: 44,
                          height: 44,
                        }}
                      >
                        <Icon />
                      </Avatar>
                      <Box flexGrow={1}>
                        <Stack
                          direction="row"
                          alignItems={{ xs: "flex-start", sm: "center" }}
                          spacing={1}
                        >
                          <Typography fontWeight={600}>
                            {scenario.title}
                          </Typography>
                          {scenario.badge && (
                            <Chip
                              size="small"
                              label={scenario.badge}
                              sx={{
                                fontWeight: 600,
                                bgcolor: alpha(ACCENT, 0.15),
                                color: ACCENT,
                              }}
                            />
                          )}
                        </Stack>
                        <Typography
                          variant="body2"
                          sx={{ color: TEXT_SECONDARY }}
                          mt={0.5}
                        >
                          {scenario.description}
                        </Typography>
                      </Box>
                      <ChevronRightRoundedIcon
                        sx={{
                          color: isActive
                            ? ACCENT
                            : theme.palette.action.disabled,
                        }}
                      />
                    </Stack>
                  </Card>
                );
              })}
            </Stack>

            <Stack
              direction={{ xs: "column", sm: "row" }}
              spacing={2}
              mt={3}
              justifyContent="space-between"
            >
              <Button
                variant="contained"
                size="large"
                disabled={activeScenario.comingSoon}
                component={
                  activeScenario.id === "design-api"
                    ? (MuiLink as any)
                    : undefined
                }
                href={
                  activeScenario.id === "design-api" ? "vscode://" : undefined
                }
                onClick={
                  activeScenario.id === "design-api"
                    ? undefined
                    : () => onContinue(activeScenarioId)
                }
                startIcon={
                  activeScenario.id === "design-api" ? (
                    <Box
                      component="img"
                      src={vscodeSvg}
                      alt=""
                      sx={{
                        width: 16,
                        height: 16,
                        display: "block",
                        filter: "brightness(0) invert(1)",
                      }}
                    />
                  ) : undefined
                }
                sx={{ bgcolor: ACCENT, "&:hover": { bgcolor: ACCENT } }}
              >
                {activeScenario.id === "design-api"
                  ? "Open VS Code"
                  : "Continue"}
              </Button>

              <Button variant="text" onClick={onSkip}>
                Skip for now
              </Button>
            </Stack>
          </Box>

          {/* Right column - recommendations */}
          <Box flex={{ xs: "auto", lg: 0.8 }}>
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
                <Typography variant="subtitle2" sx={{ color: TEXT_SECONDARY }}>
                  We Recommend
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
                <Box
                  component="img"
                  src={activeScenario.imageSrc}
                  alt={activeScenario.title}
                  sx={{
                    width: "100%",
                    height: "100%",
                    display: "block",
                    objectFit: "cover",
                  }}
                />
              </Box>

              <Box px={3} py={3} flexGrow={1}>
                <Typography fontWeight={600}>
                  Setup Steps for {activeScenario.title}
                </Typography>
                <Stack spacing={2} mt={2}>
                  {activeScenario.steps.map((step, index) => (
                    <Stack
                      key={step.title}
                      direction="row"
                      spacing={2}
                      alignItems="flex-start"
                    >
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
                        <Typography
                          variant="body2"
                          sx={{ color: TEXT_SECONDARY }}
                        >
                          {step.subtitle}
                        </Typography>
                      </Box>
                    </Stack>
                  ))}
                </Stack>
              </Box>
            </Card>
          </Box>
        </Stack>
      </Card>
    </Box>
  );
};

export default ScenarioLanding;
