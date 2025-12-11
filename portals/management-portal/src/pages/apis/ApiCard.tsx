import React from "react";
import {
  Box,
  CardActionArea,
  CardContent,
  Chip,
  Rating,
  Stack,
  Typography,
} from "@mui/material";
import DeleteIcon from "@mui/icons-material/Delete";

import theme from "../../theme";
import { IconButton } from "../../components/src/components/IconButton";
import { Card } from "../../components/src/components/Card";
import { Tooltip } from "../../components/src/components/Tooltip";

export type ApiCardData = {
  id: string;
  name: string;
  provider: string;
  version: string;
  context: string;
  description: string;
  tags: string[];
  extraTagsCount: number;
  rating: number;
};

const initials = (name: string) => {
  const letters = name.replace(/[^A-Za-z]/g, "");
  if (!letters) return "API";
  return (letters[0] + (letters[1] || "")).toUpperCase();
};

export interface ApiCardProps {
  api: ApiCardData;
  onClick: () => void;
  onDelete: (e: React.MouseEvent) => void;
}

const ApiCard: React.FC<ApiCardProps> = ({ api, onClick, onDelete }) => (
  <Card
    style={{
      padding: 16,
      position: "relative",
      width: 320,
      minHeight: 260,
      border: "1px solid transparent",
      transition: "border-color 0.2s ease",
    }}
    className="api-card-hover"
    testId={api.id}
    onClick={onClick}
  >
    <CardActionArea
      sx={{
        borderRadius: 0,
        height: "100%",
        "&.MuiCardActionArea-root:hover .MuiCardActionArea-focusHighlight": {
          opacity: 0,
        },
      }}
    >
      <CardContent
        sx={{
          p: 0,
          display: "flex",
          flexDirection: "column",
          height: "100%",
        }}
      >
        <Box sx={{ flex: 1 }}>
          <Box display="flex" alignItems="center" gap={2}>
            <Box
              sx={{
                width: 65,
                height: 65,
                borderRadius: 0.5,
                backgroundImage: `linear-gradient(135deg,
${theme.palette.augmentColor({ color: { main: "#059669" } }).light} 0%,
#059669 55%,
${theme.palette.augmentColor({ color: { main: "#059669" } }).dark} 100%)`,
                color: "common.white",
                fontWeight: 800,
                fontSize: 28,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                mb: 1.5,
              }}
            >
              {initials(api.name)}
            </Box>
            <Box sx={{ minWidth: 0, flex: 1 }}>
              <Typography
                variant="h4"
                sx={{
                  lineHeight: 1.2,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {api.name}
              </Typography>
              <Stack direction="row" spacing={2} useFlexGap>
                <Typography variant="caption" color="text.secondary">
                  By: {api.provider}
                </Typography>
              </Stack>
            </Box>
          </Box>

          <Stack direction="row" spacing={3} sx={{ mb: 1 }}>
            <Box>
              <Typography variant="caption" color="text.secondary">
                Version
              </Typography>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>
                {api.version}
              </Typography>
            </Box>
            <Box>
              <Typography variant="caption" color="text.secondary">
                Context
              </Typography>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>
                {api.context}
              </Typography>
            </Box>
          </Stack>

          {/* Description */}
          {(() => {
            const desc = api.description ?? "";
            const isTruncated = desc.length > 90;
            const short = isTruncated
              ? desc.slice(0, 90).trimEnd() + "…"
              : desc;

            return isTruncated ? (
              <Tooltip title={desc} placement="top" arrow>
                <Typography fontSize={12} color="#bbbabaff" sx={{ mb: 1.25 }}>
                  {short}
                </Typography>
              </Tooltip>
            ) : (
              <Typography fontSize={12} color="#bbbabaff" sx={{ mb: 1.25 }}>
                {desc}
              </Typography>
            );
          })()}

          {/* Tags */}
          <Stack direction="row" spacing={1} sx={{ flexWrap: "wrap", mb: 1 }}>
            {api.tags.map((t) => (
              <Chip
                key={t}
                label={t}
                size="small"
                variant="outlined"
                sx={{ borderRadius: 1 }}
              />
            ))}
            {!!api.extraTagsCount && (
              <Chip
                label={`+${api.extraTagsCount}`}
                size="small"
                variant="outlined"
                sx={{ borderRadius: 1 }}
              />
            )}
          </Stack>
        </Box>
        <Box>
          {/* BOTTOM SECTION — fixed to bottom */}
          <Stack
            direction="row"
            alignItems="center"
            justifyContent="space-between"
            spacing={1}
            sx={{ mt: "auto", pt: 1 }}
          >
            <Stack direction="row" alignItems="center" spacing={0.5}>
              <Rating
                size="small"
                readOnly
                value={api.rating}
                precision={0.5}
              />
              <Typography variant="caption" color="text.secondary">
                {api.rating.toFixed(1)}/5.0
              </Typography>
            </Stack>
            <IconButton
              onClick={onDelete}
              size="small"
              color="error"
              aria-label="Delete API"
            >
              <DeleteIcon fontSize="small" />
            </IconButton>
          </Stack>
        </Box>
      </CardContent>
    </CardActionArea>
  </Card>
);

export default ApiCard;
