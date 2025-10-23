import React from "react";
import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  Typography,
  Stack,
  Button,
} from "@mui/material";
import RocketLaunchRoundedIcon from "@mui/icons-material/RocketLaunchRounded";
import CloudUploadRoundedIcon from "@mui/icons-material/CloudUploadRounded";
import BuildRoundedIcon from "@mui/icons-material/BuildRounded";
import SmartToyRoundedIcon from "@mui/icons-material/SmartToyRounded";

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

type ApiEmptyStateProps = {
  onAction: (action: EmptyStateAction) => void;
};

const ApiEmptyState: React.FC<ApiEmptyStateProps> = ({ onAction }) => (
  <Box>
    <Typography variant="h5" fontWeight={700} gutterBottom>
      Create API Proxy
    </Typography>

    <Typography color="text.secondary" sx={{ mb: 3 }}>
      Choose a starting point to create your API. You can also explore
      resources to learn the basics.
    </Typography>

    <Box
      sx={{
        display: "grid",
        gap: 2,
        gridTemplateColumns: "repeat(auto-fit, minmax(260px, 1fr))",
      }}
    >
      {templates.map((template) => (
        <Card
          key={template.key}
          variant="outlined"
          sx={{
            borderRadius: 3,
            height: "100%",
            borderColor: "divider",
          }}
        >
          <CardActionArea
            disableRipple
            sx={{ height: "100%" }}
            onClick={() =>
              template.key === "endpoint"
                ? onAction({ type: "createFromEndpoint" })
                : onAction({ type: "learnMore", template: template.key })
            }
          >
            <CardContent
              sx={{
                display: "flex",
                flexDirection: "column",
                gap: 2,
                height: "100%",
              }}
            >
              <Box>{template.icon}</Box>

              <Typography variant="h6" fontWeight={700}>
                {template.title}
              </Typography>

              <Typography color="text.secondary" sx={{ flexGrow: 1 }}>
                {template.description}
              </Typography>

              <Stack direction="row" spacing={1}>
                <Button
                  variant="contained"
                  color="success"
                  onClick={(event) => {
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
                    onClick={(event) => {
                      event.stopPropagation();
                      onAction({ type: "learnMore", template: template.key });
                    }}
                    sx={{ textTransform: "none" }}
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

    {/* <Box
      sx={{
        mt: 4,
        p: 2.5,
        borderRadius: 3,
        border: "1px solid",
        borderColor: "divider",
      }}
    >
      <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 1 }}>
        Explore More
      </Typography>
      <Stack direction="row" spacing={3} flexWrap="wrap">
        <Button variant="text" sx={{ textTransform: "none" }}>
          Create Your First REST API
        </Button>
        <Button variant="text" sx={{ textTransform: "none" }}>
          Build an API From Scratch
        </Button>
        <Button variant="text" sx={{ textTransform: "none" }}>
          Connect Your GitHub Repository
        </Button>
        <Button variant="text" sx={{ textTransform: "none" }}>
          Manage Components with DevOps
        </Button>
      </Stack>
    </Box> */}
  </Box>
);

export default ApiEmptyState;
