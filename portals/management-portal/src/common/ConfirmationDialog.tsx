import React from "react";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogActions,
  Box,
} from "@mui/material";
import { Button } from "../components/src/components/Button";
import { TextInput } from "../components/src/components/TextInput";

type ConfirmationDialogProps = {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  message: string | React.ReactNode;
  primaryBtnText?: string;
  cancelText?: string;
  severity?: "info" | "warning" | "error";
  confirmationText?: string;
  confirmationPlaceholder?: string;
};

const ConfirmationDialog: React.FC<ConfirmationDialogProps> = ({
  open,
  onClose,
  onConfirm,
  title,
  message,
  primaryBtnText = "Confirm",
  cancelText = "Cancel",
  severity = "warning",
  confirmationText,
  confirmationPlaceholder,
}) => {
  const [inputValue, setInputValue] = React.useState("");
  const requiresConfirmation = confirmationText && confirmationText.trim();
  const isConfirmDisabled =
    requiresConfirmation && inputValue !== confirmationText;

  React.useEffect(() => {
    if (open) {
      setInputValue("");
    }
  }, [open]);

  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="sm"
      fullWidth
      PaperProps={{
        sx: {
          borderRadius: 2,
        },
      }}
    >
      <DialogTitle
        sx={{
          fontWeight: 600,
          fontSize: "1.25rem",
          display: "flex",
          alignItems: "center",
          gap: 1.5,
          paddingBottom: 0,
        }}
      >
        {title}
      </DialogTitle>

      <DialogContent sx={{ pt: 0 }}>
        {message && <Box sx={{ mb: 2 }}>{message}</Box>}
        {requiresConfirmation && (
          <Box sx={{ mt: 2 }}>
            <TextInput
              fullWidth
              size="small"
              placeholder={
                confirmationPlaceholder ||
                (confirmationText
                  ? `Type "${confirmationText}" to confirm`
                  : "")
              }
              value={inputValue}
              onChange={(value: string) => setInputValue(value)}
              testId="confirmation-input"
            />
          </Box>
        )}
      </DialogContent>

      <DialogActions sx={{ px: 3, pb: 3, gap: 1 }}>
        <Button
          variant="subtle"
          onClick={onClose}
          sx={{ textTransform: "none" }}
        >
          {cancelText}
        </Button>
        <Button
          variant="contained"
          color={
            severity === "error"
              ? "error"
              : severity === "warning"
              ? "warning"
              : "primary"
          }
          disabled={isConfirmDisabled}
          onClick={() => {
            try {
              void onConfirm();
            } catch {}
            onClose();
          }}
        >
          {primaryBtnText}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default ConfirmationDialog;
