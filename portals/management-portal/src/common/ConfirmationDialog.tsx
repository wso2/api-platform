import React from "react";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogActions,
  Typography,
  Box,
} from "@mui/material";
import { Button } from "../components/src/components/Button";

type ConfirmationDialogProps = {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  severity?: 'info' | 'warning' | 'error';
};

const ConfirmationDialog: React.FC<ConfirmationDialogProps> = ({
  open,
  onClose,
  onConfirm,
  title,
  message,
  confirmText = "Confirm",
  cancelText = "Cancel",
  severity = "warning",
}) => {
  const getSeverityColor = () => {
    switch (severity) {
      case 'error':
        return 'error.main';
      case 'warning':
        return 'warning.main';
      case 'info':
      default:
        return 'info.main';
    }
  };

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
          fontWeight: 700,
          fontSize: 20,
          pb: 1,
          display: 'flex',
          alignItems: 'center',
          gap: 1,
        }}
      >
        <Box
          sx={{
            width: 6,
            height: 6,
            borderRadius: '50%',
            backgroundColor: getSeverityColor(),
            flexShrink: 0,
          }}
        />
        {title}
      </DialogTitle>

      <DialogContent sx={{ pt: 1, pb: 3 }}>
        <Typography 
          variant="body1" 
          sx={{ 
            color: 'text.primary',
            fontSize: '1rem',
            lineHeight: 1.5,
          }}
        >
          {message}
        </Typography>
      </DialogContent>

      <DialogActions sx={{ px: 3, pb: 3, gap: 1 }}>
        <Button
          variant="outlined"
          onClick={onClose}
          sx={{ textTransform: "none" }}
        >
          {cancelText}
        </Button>
        <Button
          variant="contained"
          onClick={() => {
            // start the parent operation and close immediately so caller can manage background state
            try {
              void onConfirm();
            } catch {
              /* parent handles errors */
            }
            onClose();
          }}
          sx={{
            textTransform: "none",
            backgroundColor: severity === 'error' ? 'error.main' : undefined,
            '&:hover': {
              backgroundColor: severity === 'error' ? 'error.dark' : undefined,
            },
          }}
        >
          {confirmText}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default ConfirmationDialog;