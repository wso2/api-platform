import React from "react";
import { Box, Typography } from "@mui/material";
import AccessTimeRoundedIcon from "@mui/icons-material/AccessTimeRounded";
import type { Gateway } from "../../../hooks/gateways";
import { Tooltip } from "../../../components/src/components/Tooltip";
import { Checkbox } from "../../../components/src/components/Checkbox";
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
  gateways: Gateway[];
  selectedIds: Set<string>;
  areAllSelected: boolean; // API compat (not used)
  isSomeSelected: boolean; // API compat (not used)
  onToggleRow: (id: string) => void;
  onToggleAll: () => void; // kept for API compat
  onClear: () => void;
  onAdd: () => void;
  relativeTime: (d?: string | Date | null) => string;

  /** IDs of gateways already deployed for this API. Those will be hidden. */
  deployedIds?: Set<string> | string[];
  onBack?: () => void;
};

const headerOverline = { opacity: 0.7 } as const;

// Grid used by body pills (keep header widths in sync)
const GRID = {
  checkboxColPx: 56,
  nameColWidth: "28%",
  nameColMinPx: 220,
  updatedColPx: 230,
};

const GatewayPickTable: React.FC<Props> = ({
  gateways,
  selectedIds,
  onToggleRow,
  onToggleAll, // not used directly here
  onClear,
  onAdd,
  relativeTime,
  deployedIds,
  onBack,
}) => {
  // Normalize deployed set
  const deployedSet: Set<string> = React.useMemo(() => {
    if (!deployedIds) return new Set();
    return Array.isArray(deployedIds) ? new Set(deployedIds) : deployedIds;
  }, [deployedIds]);

  // Only show NOT deployed gateways
  const visibleGateways = React.useMemo(
    () => gateways.filter((g) => !deployedSet.has(g.id)),
    [gateways, deployedSet]
  );

  // Header checkbox state (over visible only)
  const rowCount = visibleGateways.length;
  const numSelected = React.useMemo(
    () =>
      visibleGateways.reduce(
        (acc, g) => acc + (selectedIds.has(g.id) ? 1 : 0),
        0
      ),
    [visibleGateways, selectedIds]
  );
  const allSelected = rowCount > 0 && numSelected === rowCount;
  const someSelected = numSelected > 0 && numSelected < rowCount;

  // Local select all over visible rows
  const handleToggleAllVisible = () => {
    if (rowCount === 0) return;
    if (allSelected) {
      visibleGateways.forEach((g) => {
        if (selectedIds.has(g.id)) onToggleRow(g.id);
      });
    } else {
      visibleGateways.forEach((g) => {
        if (!selectedIds.has(g.id)) onToggleRow(g.id);
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
      <Typography variant="h4" fontWeight={600}>
        Select Gateways
      </Typography>
      <TableContainer>
        <TableDefault
          variant="default"
          aria-labelledby="gatewaysTable"
          aria-label="gateways table"
          testId="gateways-table"
        >
          {/* HEADER (aligned, no underline/hover) */}
          <TableHead>
            <TableRow>
              <TableCell
                padding="checkbox"
                sx={{
                  width: GRID.checkboxColPx,
                  borderBottom: "none",
                  backgroundColor: "transparent",
                }}
              >
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
                    inputProps={{ "aria-label": "select all gateways" }}
                    testId="table-head"
                  />
                </Tooltip>
              </TableCell>

              <TableCell
                sx={{
                  width: GRID.nameColWidth,
                  minWidth: GRID.nameColMinPx,
                  borderBottom: "none",
                  backgroundColor: "transparent",
                }}
              >
                <Typography variant="overline" sx={headerOverline}>
                  Name
                </Typography>
              </TableCell>

              <TableCell
                sx={{
                  borderBottom: "none",
                  backgroundColor: "transparent",
                }}
              >
                <Typography variant="overline" sx={headerOverline}>
                  Description
                </Typography>
              </TableCell>

              <TableCell
                sx={{
                  width: GRID.updatedColPx,
                  borderBottom: "none",
                  backgroundColor: "transparent",
                }}
              >
                <Typography variant="overline" sx={headerOverline}>
                  Last Updated
                </Typography>
              </TableCell>
            </TableRow>
          </TableHead>

          {/* BODY: each row is one pill inside a single colSpan cell */}
          <TableBody>
            {visibleGateways.length === 0 ? (
              <TableRowNoData
                testId="no-data-row"
                colSpan={4}
                message="No undeployed gateways to show"
              />
            ) : (
              visibleGateways.map((gw, idx) => {
                const isChecked = selectedIds.has(gw.id);
                const title = gw.displayName || gw.name || "Gateway";
                const initial = (title || "?").trim().charAt(0).toUpperCase();
                const last = gw.updatedAt || gw.createdAt || null;

                return (
                  <React.Fragment key={gw.id}>
                    <TableRow>
                      <TableCell
                        colSpan={4}
                        sx={{ p: 0, border: 0, background: "transparent" }}
                      >
                        <Box
                          onClick={() => onToggleRow(gw.id)}
                          role="button"
                          sx={{
                            display: "grid",
                            gridTemplateColumns: `
                              ${GRID.checkboxColPx}px
                              minmax(${GRID.nameColMinPx}px, ${GRID.nameColWidth})
                              1fr
                              ${GRID.updatedColPx}px
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
                          {/* 1) Checkbox (leftmost) */}
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
                                onToggleRow(gw.id);
                              }}
                              onClick={(e: any) => e.stopPropagation()}
                            />
                          </Box>

                          {/* 2) Name with avatar initial */}
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
                          <Box sx={{ minWidth: 0, maxWidth: 500 }}>
                            <Typography
                              variant="body2"
                              color="text.primary"
                              noWrap
                              title={gw.description || ""}
                            >
                              {gw.description || ""}
                            </Typography>
                          </Box>

                          {/* 4) Last Updated */}
                          <Box
                            sx={{
                              display: "flex",
                              alignItems: "center",
                              justifyContent: "flex-start",
                              gap: 0.75,
                            }}
                          >
                            <AccessTimeRoundedIcon
                              fontSize="small"
                              sx={{ opacity: 0.7 }}
                            />
                            <Typography
                              variant="body2"
                              color="text.primary"
                              noWrap
                            >
                              {last ? relativeTime(last) : "—"}
                            </Typography>
                          </Box>
                        </Box>
                      </TableCell>
                    </TableRow>

                    {/* Spacer between pills */}
                    {idx < visibleGateways.length - 1 && (
                      <TableRow aria-hidden>
                        <TableCell
                          colSpan={4}
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

export default GatewayPickTable;
