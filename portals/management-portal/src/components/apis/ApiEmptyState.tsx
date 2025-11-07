import React from "react";
import { Box, Typography, Stack } from "@mui/material";
import RocketLaunchRoundedIcon from "@mui/icons-material/RocketLaunchRounded";
import CloudUploadRoundedIcon from "@mui/icons-material/CloudUploadRounded";
import BuildRoundedIcon from "@mui/icons-material/BuildRounded";
import SmartToyRoundedIcon from "@mui/icons-material/SmartToyRounded";
import { Button } from "../src/components/Button";
import WithEndpointIcon from "./WithEndpoint.svg";
import WithGenAIIcon from "./WithGenAI.svg";
import APIContractIcon from "./APIContract.svg";
import FromScratchIcon from "./FromScratch.svg";

import { Card, CardActionArea, CardContent } from "../src/components/Card";

type TemplateKey = "endpoint" | "contract" | "scratch" | "genai";

export type EmptyStateAction =
  | { type: "createFromEndpoint" }
  | { type: "learnMore"; template: TemplateKey };

const templates = [
  {
    key: "contract" as TemplateKey,
    title: "Import API Contract",
    description: "Bring an existing OpenAPI definition to get started quickly.",
    icon: <CloudUploadRoundedIcon color="primary" fontSize="large" />,
    showLearnMore: true,
  },
  {
    key: "endpoint" as TemplateKey,
    title: "Start with Endpoint",
    description: "Provide a backend URL and we'll scaffold the API for you.",
    icon: <RocketLaunchRoundedIcon color="secondary" fontSize="large" />,
    showLearnMore: true,
  },
  {
    key: "scratch" as TemplateKey,
    title: "Start from Scratch",
    description: "Design your API resources and operations manually.",
    icon: <BuildRoundedIcon color="success" fontSize="large" />,
    showLearnMore: true,
  },
  {
    key: "genai" as TemplateKey,
    title: "Create with GenAI",
    description: "Generate an API using natural language prompts.",
    icon: <SmartToyRoundedIcon color="info" fontSize="large" />,
    showLearnMore: true,
  },
];

const iconMap: Record<TemplateKey, string> = {
  endpoint: WithEndpointIcon,
  genai: WithGenAIIcon,
  contract: APIContractIcon,
  scratch: FromScratchIcon,
};

type ApiEmptyStateProps = {
  onAction: (action: EmptyStateAction) => void;
};

const ApiEmptyState: React.FC<ApiEmptyStateProps> = ({ onAction }) => (
  <Box>
    <Typography variant="h3" fontWeight={700} gutterBottom>
      Create API Proxy
    </Typography>

    <Typography color="#787575ff" sx={{ mb: 3 }}>
      Choose a starting point to create your API. You can also explore resources
      to learn the basics.
    </Typography>

    <Box
      sx={{
        display: "grid",
        gap: 2,
        gridTemplateColumns: "repeat(auto-fit, minmax(260px, 1fr))",
      }}
    >
      {templates.map((template) => (
        <Card key={template.key} variant="outlined" testId={""}>
          <CardActionArea
            sx={{ height: "100%" }}
            onClick={() =>
              template.key === "endpoint"
                ? onAction({ type: "createFromEndpoint" })
                : onAction({ type: "learnMore", template: template.key })
            }
            testId={""}
          >
            <CardContent
              sx={{
                display: "flex",
                flexDirection: "column",
                gap: 2,
                height: "100%",
              }}
            >
              <Box
                component="img"
                src={iconMap[template.key]}
                alt={template.title}
                sx={{
                  width: 56,
                  height: 56,
                  display: "grid",
                  placeItems: "center",
                }}
              />

              <Box>
                <Typography variant="h4" fontWeight={600}>
                  {template.title}
                </Typography>
                <Typography color="#828181ff" sx={{ flexGrow: 1 }}>
                  You need to create a project to work with components
                </Typography>
              </Box>

              <Stack direction="row" spacing={1}>
                <Button
                  variant="contained"
                  color="success"
                  onClick={(event: { stopPropagation: () => void }) => {
                    event.stopPropagation();
                    if (template.key === "endpoint") {
                      onAction({ type: "createFromEndpoint" });
                    } else {
                      onAction({ type: "learnMore", template: template.key });
                    }
                  }}
                  sx={{ textTransform: "none" }}
                >
                  Create
                </Button>
                {template.showLearnMore && (
                  <Button
                    variant="text"
                    onClick={(event: { stopPropagation: () => void }) => {
                      event.stopPropagation();
                      onAction({ type: "learnMore", template: template.key });
                    }}
                    sx={{ textTransform: "none", color: "success.main" }}
                  >
                    Learn More
                  </Button>
                )}
              </Stack>
            </CardContent>
          </CardActionArea>
        </Card>
      ))}
    </Box>
  </Box>
);

export default ApiEmptyState;
