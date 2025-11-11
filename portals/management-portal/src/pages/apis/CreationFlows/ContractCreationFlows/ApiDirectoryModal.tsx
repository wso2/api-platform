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
} from "@mui/material";
import FolderOutlinedIcon from "@mui/icons-material/FolderOutlined";
import ChevronRightRoundedIcon from "@mui/icons-material/ChevronRightRounded";
import { TextInput } from "../../../../components/src/components/TextInput";
import { Button } from "../../../../components/src/components/Button";

type Props = {
  open: boolean;
  currentPath: string;
  rootLabel: string; // e.g., "bijira-samples"
  onCancel: () => void;
  onContinue: (path: string) => void;
};

const ApiDirectoryModal: React.FC<Props> = ({
  open,
  currentPath,
  rootLabel,
  onCancel,
  onContinue,
}) => {
  // UI only; static list
  const [path, setPath] = React.useState(currentPath || "/");
  const [query, setQuery] = React.useState("");

  React.useEffect(() => {
    if (open) {
      setPath(currentPath || "/");
      setQuery("");
    }
  }, [open, currentPath]);

  const items = [
    ".samples",
    "chat-service-api",
    "cloudmersive-currency-api",
    "external-lib",
    "mcp-card-promotion-server",
    "mcp-chat-agent",
  ];

  const filtered = items.filter((n) =>
    n.toLowerCase().includes(query.toLowerCase())
  );

  return (
    <Dialog open={open} onClose={onCancel} maxWidth="md" fullWidth>
      <DialogTitle sx={{ textAlign: "center", fontWeight: 700, fontSize: 28, mt: 1 }}>
        API directory
      </DialogTitle>

      <DialogContent sx={{ pt: 1 }}>
        {/* Current path */}
        <TextInput
          label=""
          placeholder="/"
          value={path}
          onChange={(v: string) => setPath(v)}
          size="medium"
          testId=""
          disabled
        />

        {/* Search */}
        <Box sx={{ mt: 2 }}>
          <TextInput
            label=""
            placeholder="Search Directories"
            value={query}
            onChange={(v: string) => setQuery(v)}
            size="medium"
            testId=""
          />
        </Box>

        {/* Directory list */}
        <Box
          sx={{
            mt: 2,
            border: "1px solid",
            borderColor: "divider",
            borderRadius: 2,
          }}
        >
          <Box
            sx={{
              px: 2,
              py: 1.25,
              borderBottom: "1px solid",
              borderColor: "divider",
              display: "flex",
              alignItems: "center",
              gap: 1,
            }}
          >
            <FolderOutlinedIcon fontSize="small" />
            <Typography variant="subtitle2" fontWeight={700}>
              {rootLabel}
            </Typography>
          </Box>

          <Box sx={{ maxHeight: 360, overflow: "auto" }}>
            <List disablePadding>
              {filtered.map((name) => (
                <React.Fragment key={name}>
                  <ListItem disablePadding>
                    <ListItemButton onClick={() => setPath(`/${name}`)} sx={{ px: 2 }}>
                      <ListItemIcon sx={{ minWidth: 32 }}>
                        <FolderOutlinedIcon fontSize="small" />
                      </ListItemIcon>
                      <ListItemText
                        primary={
                          <Typography variant="body2" sx={{ fontSize: 14 }}>
                            {name}
                          </Typography>
                        }
                      />
                      <ChevronRightRoundedIcon fontSize="small" />
                    </ListItemButton>
                  </ListItem>
                  <Divider />
                </React.Fragment>
              ))}
            </List>
          </Box>
        </Box>
      </DialogContent>

      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button variant="outlined" onClick={onCancel}>
          Cancel
        </Button>
        <Button variant="contained" onClick={() => onContinue(path)}>
          Continue
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default ApiDirectoryModal;
