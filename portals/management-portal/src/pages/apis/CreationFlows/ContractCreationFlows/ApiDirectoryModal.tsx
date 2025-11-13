import * as React from "react";
import {
  Box,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Typography,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Divider,
  Collapse,
  Chip,
  InputAdornment,
} from "@mui/material";
import FolderOutlinedIcon from "@mui/icons-material/FolderOutlined";
import InsertDriveFileOutlinedIcon from "@mui/icons-material/InsertDriveFileOutlined";
import ExpandMoreRoundedIcon from "@mui/icons-material/ExpandMoreRounded";
import ChevronRightRoundedIcon from "@mui/icons-material/ChevronRightRounded";
import SearchRoundedIcon from "@mui/icons-material/SearchRounded";
import { TextInput } from "../../../../components/src/components/TextInput";
import { Button } from "../../../../components/src/components/Button";

/** Align with hook types; inline here for convenience */
type GitTreeItemType = "tree" | "blob";
type GitTreeItem = {
  path: string;
  subPath: string;
  children: GitTreeItem[];
  type: GitTreeItemType;
};

type Props = {
  open: boolean;
  currentPath: string;
  rootLabel: string; // e.g., "repo-name"
  items: GitTreeItem[]; // full tree for branch
  onCancel: () => void;
  onContinue: (path: string) => void;
};

const normalizePath = (value: string) => `/${value}`.replace(/\/+/g, "/");

const ApiDirectoryModal: React.FC<Props> = ({
  open,
  currentPath,
  rootLabel,
  items,
  onCancel,
  onContinue,
}) => {
  const [path, setPath] = React.useState(currentPath || "/");
  const [query, setQuery] = React.useState("");

  // track expanded folders (keyed by item.path)
  const [expanded, setExpanded] = React.useState<Record<string, boolean>>({});

  React.useEffect(() => {
    if (open) {
      setPath(currentPath || "/");
      setQuery("");
      setExpanded({});
    }
  }, [open, currentPath]);

  const toggle = (key: string) =>
    setExpanded((prev) => ({ ...prev, [key]: !prev[key] }));

  const matchesQuery = (node: GitTreeItem, q: string) => {
    if (!q.trim()) return true;
    const s = q.toLowerCase();
    return (
      node.path.toLowerCase().includes(s) ||
      node.subPath.toLowerCase().includes(s)
    );
  };

  /** Recursively render tree; only folders (type==="tree") are selectable */
  const renderNodes = (nodes: GitTreeItem[], depth = 0): React.ReactNode => {
    return nodes.map((n) => {
      const childMatches =
        n.children?.some((c) => matchesQuery(c, query)) || false;
      const show = matchesQuery(n, query) || childMatches;
      if (!show) return null;

      const isFolder = n.type === "tree";
      const normalizedPath = normalizePath(n.path);
      const shouldForceOpen =
        !!query.trim() && (matchesQuery(n, query) || childMatches);
      const isOpen = shouldForceOpen || !!expanded[n.path];
      const isSelected = normalizedPath === path;

      return (
        <React.Fragment key={n.path}>
          <ListItem disablePadding sx={{ pl: 2 + depth * 2, pr: 2 }}>
            <ListItemButton
              onClick={() => {
                if (isFolder) {
                  setPath(normalizedPath);
                  toggle(n.path);
                }
              }}
              disabled={!isFolder}
              sx={{
                px: 1.5,
                py: 1.25,
                borderRadius: 2,
                border: "1px solid",
                borderColor: isSelected ? "#afe7caff" : "transparent",
                backgroundColor: isSelected
                  ? "rgba(171, 225, 198, 0.32)"
                  : "transparent",
                opacity: isFolder ? 1 : 0.85,
                "&:hover": {
                  backgroundColor: isFolder
                    ? "rgba(171, 225, 182, 0.25)"
                    : "transparent",
                },
                "&.Mui-disabled": {
                  opacity: 0.8,
                },
              }}
            >
              <ListItemIcon
                sx={{
                  minWidth: 32,
                  color: isFolder ? "#585c5aff" : "#626467ff",
                }}
              >
                {isFolder ? (
                  <FolderOutlinedIcon fontSize="small" />
                ) : (
                  <InsertDriveFileOutlinedIcon fontSize="small" />
                )}
              </ListItemIcon>
              <ListItemText
                primary={
                  <Box display="flex" alignItems="center" gap={1}>
                    <Typography
                      variant="h6"
                      sx={{
                        fontWeight: isFolder ? 600 : 400,
                        color: "#1F2937",
                      }}
                    >
                      {n.subPath}
                    </Typography>
                    {!isFolder && (
                      <Chip
                        size="small"
                        label="file"
                        variant="outlined"
                        sx={{
                          borderRadius: 1,
                          fontSize: 10,
                          color: "#6B7280",
                          borderColor: "#E5E7EB",
                        }}
                      />
                    )}
                  </Box>
                }
              />
              {isFolder ? (
                isOpen ? (
                  <ExpandMoreRoundedIcon fontSize="small" />
                ) : (
                  <ChevronRightRoundedIcon fontSize="small" />
                )
              ) : null}
            </ListItemButton>
          </ListItem>
          <Divider
            sx={{ borderColor: "rgba(15, 23, 42, 0.06)", ml: 6 + depth * 2 }}
          />

          {isFolder && n.children && n.children.length > 0 && (
            <Collapse in={isOpen} timeout="auto" unmountOnExit>
              <List disablePadding>{renderNodes(n.children, depth + 1)}</List>
            </Collapse>
          )}
        </React.Fragment>
      );
    });
  };

  const isRootSelected = path === "/";

  return (
    <Dialog open={open} onClose={onCancel} maxWidth="md" fullWidth>
      <DialogTitle sx={{ textAlign: "center", fontWeight: 600, fontSize: 24 }}>
        API directory
      </DialogTitle>

      <DialogContent>
        <TextInput
          label=""
          placeholder="/"
          value={path}
          onChange={() => {}}
          size="medium"
          testId="GH-repo-path-input"
          fullWidth
          disabled
        />

        <Box sx={{ mt: 1 }}>
          <TextInput
            label=""
            placeholder="Search Directories"
            value={query}
            onChange={(v: string) => setQuery(v)}
            size="medium"
            testId="GH-repo-search-input"
            fullWidth
            InputProps={{
              startAdornment: (
                <InputAdornment position="start">
                  <SearchRoundedIcon sx={{ color: "#A0AEC0" }} />
                </InputAdornment>
              ),
            }}
          />
        </Box>

        <Box
          sx={{
            mt: 2,
            border: "1px solid #E2E8F0",
            borderRadius: 1,
            backgroundColor: "#fff",
          }}
        >
          {/* Header */}
          <Box
            sx={{
              px: 2.5,
              py: 1.5,
              borderBottom: "1px solid #E2E8F0",
              display: "flex",
              alignItems: "center",
              gap: 1.5,
            }}
          >
            <FolderOutlinedIcon sx={{ color: "#6C8CAB" }} fontSize="small" />
            <Typography variant="h5" fontWeight={600}>
              {rootLabel}
            </Typography>
          </Box>

          {/* Selectable Root row */}
          <List disablePadding>
            <ListItem disablePadding sx={{ pl: 2, pr: 2 }}>
              <ListItemButton
                onClick={() => setPath("/")}
                sx={{
                  px: 1.5,
                  py: 1.1,
                  borderRadius: 2,
                  border: "1px solid",
                  borderColor: isRootSelected ? "#afe7caff" : "transparent",
                  backgroundColor: isRootSelected
                    ? "rgba(171, 225, 198, 0.32)"
                    : "transparent",
                }}
              >
                <ListItemIcon sx={{ minWidth: 32, color: "#585c5aff" }}>
                  <FolderOutlinedIcon fontSize="small" />
                </ListItemIcon>
                <ListItemText
                  primary={
                    <Box display="flex" alignItems="center" gap={1}>
                      <Typography
                        variant="h6"
                        sx={{ fontWeight: 600, color: "#1F2937" }}
                      >
                        /
                      </Typography>
                      <Chip
                        size="small"
                        label="root"
                        variant="outlined"
                        sx={{
                          borderRadius: 1,
                          fontSize: 10,
                          color: "#6B7280",
                          borderColor: "#E5E7EB",
                        }}
                      />
                    </Box>
                  }
                />
              </ListItemButton>
            </ListItem>

            <Divider sx={{ borderColor: "rgba(15, 23, 42, 0.06)", ml: 6 }} />

            {/* Tree */}
            <Box sx={{ maxHeight: 420, overflow: "auto", px: 0.5, py: 1 }}>
              <List disablePadding>{renderNodes(items)}</List>
            </Box>
          </List>
        </Box>
      </DialogContent>

      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button variant="outlined" onClick={onCancel}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={() => onContinue(path)}
          disabled={!path} // allow "/" now
        >
          Continue
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default ApiDirectoryModal;
