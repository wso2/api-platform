import React from "react";
import { Box, Typography } from "@mui/material";
import LaunchIcon from "@mui/icons-material/Launch";
import type { Portal } from "../../../hooks/devportals";
import { Tooltip } from "../../../components/src/components/Tooltip";
import { Checkbox } from "../../../components/src/components/Checkbox";
import { Chip } from "../../../components/src/components/Chip";
import { TableRowNoData } from "../../../components/src/components/TableDefault/TableRowNoData";
import { Button } from "../../../components/src/components/Button";
import { TableBody } from "../../../components/src/components/TableDefault/TableBody/TableBody";
import { TableDefault } from "../../../components/src/components/TableDefault";
import { TableHead } from "../../../components/src/components/TableDefault/TableHead/TableHead";
import { TableRow } from "../../../components/src/components/TableDefault/TableRow/TableRow";
import { TableCell } from "../../../components/src/components/TableDefault/TableCell/TableCell";
import { TableContainer } from "../../../components/src/components/TableDefault/TableContainer/TableContainer";
import ArrowLeftLong from "../../../components/src/Icons/generated/ArrowLeftLong";

type Props = {
  portals: Portal[];
  selectedIds: Set<string>;
  areAllSelected: boolean; // API compat (not used)
  isSomeSelected: boolean; // API compat (not used)
  onToggleRow: (id: string) => void;
  onToggleAll: () => void; // kept for API compat
  onClear: () => void;
  onAdd: () => void;

  /** IDs of portals already published for this API. Those will be hidden. */
  publishedIds?: Set<string> | string[];
  onBack?: () => void;
};

const headerOverline = { opacity: 0.7 } as const;

// Keep header widths in sync with the body grid
const GRID = {
  checkboxColPx: 56,
  nameColWidth: "20%", // reduced to give more space to URL
  nameColMinPx: 180,
  descriptionColPx: 400, // fixed width for description
  uiUrlColPx: 250, // increased for better URL display
  visibilityColPx: 100,
};

const DevPortalPickTable: React.FC<Props> = ({
  portals,
  selectedIds,
  onToggleRow,
  onToggleAll, // not used directly here
  onClear,
  onAdd,
  publishedIds,
  onBack,
}) => {
  console.log("Rendering DevPortalPickTable");
  console.log("portals:", portals);
  console.log("selectedIds:", Array.from(selectedIds));
  console.log("publishedIds:", publishedIds);
  // Normalize published set
  const publishedSet: Set<string> = React.useMemo(() => {
    if (!publishedIds) return new Set();
    return Array.isArray(publishedIds) ? new Set(publishedIds) : publishedIds;
  }, [publishedIds]);

  // Only show NOT published portals
  const visiblePortals = React.useMemo(
    () => portals.filter((p) => !publishedSet.has(p.uuid)),
    [portals, publishedSet]
  );

  // Header checkbox state (over visible only)
  const rowCount = visiblePortals.length;
  const numSelected = React.useMemo(
    () =>
      visiblePortals.reduce(
        (acc, p) => acc + (selectedIds.has(p.uuid) ? 1 : 0),
        0
      ),
    [visiblePortals, selectedIds]
  );
  const allSelected = rowCount > 0 && numSelected === rowCount;
  const someSelected = numSelected > 0 && numSelected < rowCount;

  // Local select all over visible rows
  const handleToggleAllVisible = () => {
    if (rowCount === 0) return;
    if (allSelected) {
      visiblePortals.forEach((p) => {
        if (selectedIds.has(p.uuid)) onToggleRow(p.uuid);
      });
    } else {
      visiblePortals.forEach((p) => {
        if (!selectedIds.has(p.uuid)) onToggleRow(p.uuid);
      });
    }
  };

  return (
    <Box>
      {onBack && (
        <Box mb={2}>
          <Button
            onClick={onBack}
            variant="link"
            startIcon={<ArrowLeftLong fontSize="small" />}
          >
            Back to List
          </Button>
        </Box>
      )}

      <Typography variant="h4" fontWeight={600} sx={{ mb: 1 }}>
        Select Dev Portals
      </Typography>

      <TableContainer>
        <TableDefault
          variant="default"
          aria-labelledby="portalsTable"
          aria-label="dev portals table"
          testId="portals-table"
        >
          {/* HEADER (aligned, no underline/hover) */}
          <TableHead>
            <TableRow>
              <TableCell
                colSpan={6}
                sx={{
                  p: 0,
                  borderBottom: "none",
                  backgroundColor: "transparent",
                }}
              >
                <Box
                  sx={{
                    display: "grid",
                    gridTemplateColumns: `
            ${GRID.checkboxColPx}px
            minmax(${GRID.nameColMinPx}px, ${GRID.nameColWidth})
            ${GRID.descriptionColPx}px
            ${GRID.uiUrlColPx}px
            ${GRID.visibilityColPx}px
          `,
                    alignItems: "center",
                    columnGap: 2,
                    px: 2, // <-- matches body pill padding
                    py: 1, // subtle vertical breathing room
                  }}
                >
                  {/* Checkbox header (Select All) */}
                  <Box>
                    <Tooltip title="Select all" placement="bottom-start">
                      <Checkbox
                        indeterminate={someSelected}
                        checked={allSelected}
                        onChange={(e: any) => {
                          e.stopPropagation();
                          handleToggleAllVisible();
                        }}
                        onClick={(e: any) => e.stopPropagation()}
                        disableRipple
                        inputProps={{ "aria-label": "select all dev portals" }}
                        testId="table-head"
                      />
                    </Tooltip>
                  </Box>

                  {/* Name */}
                  <Typography variant="overline" sx={headerOverline}>
                    Name
                  </Typography>

                  {/* Description */}
                  <Typography variant="overline" sx={headerOverline}>
                    Description
                  </Typography>

                  {/* UI URL */}
                  <Typography variant="overline" sx={headerOverline}>
                    UI URL
                  </Typography>

                  {/* Visibility */}
                  <Typography variant="overline" sx={headerOverline}>
                    Visibility
                  </Typography>
                </Box>
              </TableCell>
            </TableRow>
          </TableHead>

          {/* BODY: each row is a pill across all columns */}
          <TableBody>
            {visiblePortals.length === 0 ? (
              <TableRowNoData
                testId="no-data-row"
                colSpan={5}
                message="No unpublished dev portals to show"
              />
            ) : (
              visiblePortals.map((portal, idx) => {
                const isChecked = selectedIds.has(portal.uuid);
                const title = portal.name || "Dev Portal";
                const initial = (title || "?").trim().charAt(0).toUpperCase();

                return (
                  <React.Fragment key={portal.uuid}>
                    <TableRow>
                      <TableCell
                        colSpan={6}
                        sx={{ p: 0, border: 0, background: "transparent" }}
                      >
                        <Box
                          onClick={() => onToggleRow(portal.uuid)}
                          role="button"
                          sx={{
                            display: "grid",
                            gridTemplateColumns: `
                              ${GRID.checkboxColPx}px
                              minmax(${GRID.nameColMinPx}px, ${GRID.nameColWidth})
                              ${GRID.descriptionColPx}px
                              ${GRID.uiUrlColPx}px
                              ${GRID.visibilityColPx}px
                            `,
                            alignItems: "center",
                            columnGap: 2,
                            px: 2,
                            py: 1.25,
                            borderRadius: 2,
                            backgroundColor: "grey.50",
                            boxShadow:
                              "0 1px 2px rgba(16,24,40,0.04), 0 1px 3px rgba(16,24,40,0.06)",
                            border: "1px solid",
                            borderColor: isChecked
                              ? "primary.light"
                              : "divider",
                            cursor: "pointer",
                            transition:
                              "background-color 120ms ease, border-color 120ms ease",
                            "&:hover": { backgroundColor: "grey.100" },
                          }}
                        >
                          {/* 1) Checkbox */}
                          <Box
                            sx={{
                              display: "flex",
                              alignItems: "center",
                              pl: 0.25,
                            }}
                          >
                            <Checkbox
                              checked={isChecked}
                              disableRipple
                              aria-label={`select ${title}`}
                              testId="table-row-checkbox"
                              onChange={(e: any) => {
                                e.stopPropagation();
                                onToggleRow(portal.uuid);
                              }}
                              onClick={(e: any) => e.stopPropagation()}
                            />
                          </Box>

                          {/* 2) Name (with avatar initial) */}
                          <Box
                            sx={{
                              display: "flex",
                              alignItems: "center",
                              gap: 1.25,
                              minWidth: 0,
                            }}
                          >
                            <Box
                              sx={{
                                width: 35,
                                height: 35,
                                borderRadius: "50%",
                                display: "grid",
                                placeItems: "center",
                                backgroundColor: "grey.100",
                                color: "text.primary",
                                fontSize: 12,
                                fontWeight: 700,
                                flex: "0 0 auto",
                              }}
                              aria-hidden
                            >
                              {initial}
                            </Box>
                            <Typography
                              variant="body2"
                              fontWeight={700}
                              noWrap
                              title={title}
                              sx={{ minWidth: 0 }}
                            >
                              {title}
                            </Typography>
                          </Box>

                          {/* 3) Description */}
                          <Box sx={{ minWidth: 0, maxWidth: GRID.descriptionColPx, marginRight: 3 }}>
                            <Typography
                              variant="body2"
                              color="text.primary"
                              noWrap
                              title={portal.description || ""}
                            >
                              {portal.description || ""}
                            </Typography>
                          </Box>

                          {/* 4) UI URL */}
                          <Box sx={{ minWidth: 0 }}>
                            {portal.uiUrl ? (
                              <Box
                                sx={{
                                  display: "flex",
                                  alignItems: "center",
                                  gap: 0.5,
                                  cursor: "pointer",
                                  color: "primary.main",
                                  "&:hover": { 
                                    textDecoration: "underline",
                                    color: "primary.dark"
                                  },
                                }}
                                onClick={(e) => {
                                  e.stopPropagation();
                                  window.open(portal.uiUrl, "_blank");
                                }}
                                title={portal.uiUrl}
                              >
                                <LaunchIcon fontSize="small" />
                                <Typography
                                  variant="body2"
                                  color="inherit"
                                  noWrap
                                  sx={{ 
                                    textDecoration: "underline",
                                    maxWidth: "180px"
                                  }}
                                >
                                  {portal.uiUrl}
                                </Typography>
                              </Box>
                            ) : (
                              <Typography variant="body2" color="text.secondary">
                                —
                              </Typography>
                            )}
                          </Box>

                          {/* 5) Visibility */}
                          <Box>
                            <Chip
                              label={portal.visibility === "public" ? "Public" : portal.visibility === "private" ? "Private" : "Unknown"}
                              size="small"
                              color={portal.visibility === "public" ? "success" : "default"}
                              variant="outlined"
                            />
                          </Box>
                        </Box>
                      </TableCell>
                    </TableRow>

                    {/* Spacer between pills */}
                    {idx < visiblePortals.length - 1 && (
                      <TableRow aria-hidden>
                        <TableCell
                          colSpan={6}
                          sx={{
                            py: 0.75,
                            border: 0,
                            background: "transparent",
                          }}
                        />
                      </TableRow>
                    )}
                  </React.Fragment>
                );
              })
            )}
          </TableBody>
        </TableDefault>
      </TableContainer>

      {/* Actions */}
      <Box display="flex" justifyContent="flex-start" gap={2} mt={2}>
        <Button variant="outlined" onClick={onClear}>
          Clear
        </Button>
        <Button
          variant="contained"
          onClick={onAdd}
          disabled={selectedIds.size === 0}
        >
          Add
        </Button>
      </Box>
    </Box>
  );
};

export default DevPortalPickTable;
