import React from "react";
import { Box, Paper, Stack, Typography } from "@mui/material";
import InfoOutlinedIcon from "@mui/icons-material/InfoOutlined";
import LibraryBooksOutlinedIcon from "@mui/icons-material/LibraryBooksOutlined";
import SupportAgentOutlinedIcon from "@mui/icons-material/SupportAgentOutlined";

const defaultLearningLinks = [
  {
    icon: <InfoOutlinedIcon sx={{ color: "#1f51ff" }} />,
    title: "Tutorials",
    links: [
      { label: "Create a REST API from an OpenAPI Definition", href: "#" },
      { label: "Engage Access Control to the API", href: "#" },
      { label: "Engage API policies to the API", href: "#" },
    ],
  },
  {
    icon: <LibraryBooksOutlinedIcon sx={{ color: "#0d99ff" }} />,
    title: "References",
    links: [{ label: "API Platform Key Concepts", href: "#" }],
  },
  {
    icon: <SupportAgentOutlinedIcon sx={{ color: "#8854d0" }} />,
    title: "Support",
    links: [{ label: "Get Support on Discord", href: "#" }],
  },
];

const ExploreMore: React.FC = () => {
  return (
    <Box mt={6}>
      <Typography variant="h5" fontWeight={600} mb={2}>
        Explore More
      </Typography>

      <Paper
        variant="outlined"
        sx={{
          borderRadius: 3,
          p: 3,
          display: "grid",
          gap: 3,
          gridTemplateColumns: { xs: "1fr", md: "repeat(3, 1fr)" },
        }}
      >
        {defaultLearningLinks.map((section) => (
          <Stack key={section.title} spacing={1.5}>
            <Stack direction="row" spacing={1} alignItems="center">
              {section.icon}
              <Typography fontWeight={600}>{section.title}</Typography>
            </Stack>
            <Stack spacing={1}>
              {section.links.map((link) => (
                <Typography
                  key={link.label}
                  variant="body2"
                  color="#252525ff"
                  component="a"
                  sx={{ textDecoration: "none", cursor: "pointer" }}
                  href={link.href}
                  onClick={(event) => event.preventDefault()}
                >
                  → {link.label}
                </Typography>
              ))}
            </Stack>
          </Stack>
        ))}
      </Paper>
    </Box>
  );
};

export default ExploreMore;
