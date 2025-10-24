import { Button, type ButtonProps } from "@mui/material";
import { styled, alpha } from "@mui/material/styles";
import type { ComponentType } from "react";

export const StyledButton: ComponentType<ButtonProps> = styled(
  Button
)<ButtonProps>(({ theme, disabled }) => ({
  // Common styles
  boxShadow: `0 1px 2px ${alpha(theme.palette.common.black, 0.15)}`,
  borderRadius: 5,
  color: theme.palette.common.white,
  padding: theme.spacing(0.875, 2),
  gap: theme.spacing(1),
  fontWeight: 400,
  fontSize: theme.spacing(1.625),
  lineHeight: `${theme.spacing(3)}px`,
  textTransform: "none",
  pointerEvents: disabled ? "none" : "auto",
  backgroundColor: "#069668",

  "&.Mui-disabled": {
    opacity: 0.5,
    cursor: "default",
    pointerEvents: "none",
    color: theme.palette.common.white,
    "&:hover": {
      textDecoration: "none",
    },
  },

  "& .MuiButton-startIcon": {
    marginRight: theme.spacing(1),
    "& > *:first-of-type": {
      fontSize: theme.spacing(2),
    },
  },

  "& .MuiButton-endIcon": {
    marginLeft: theme.spacing(1),
    "& > *:first-of-type": {
      fontSize: theme.spacing(2),
    },
  },

  // Contained variant
  "&.MuiButton-contained": {
    border: "1px solid transparent",
    "&:hover": {
      boxShadow: `0 1px 3px ${alpha(theme.palette.common.black, 0.1)}`,
    },
    "&:focus": {
      boxShadow: `0 1px 6px 2px ${alpha(theme.palette.common.black, 0.1)}`,
    },
    "&.MuiButton-containedPrimary": {
      backgroundColor: "#069668",
      borderColor: "#069668",
      "&:hover": {
        backgroundColor: "#02835aff",
        borderColor: "#02835aff",
      },
    },
    "&.MuiButton-containedSecondary": {
      backgroundColor: theme.palette.secondary.light,
      color: theme.palette.common.black,
      border: `1px solid ${theme.palette.grey[100]}`,
      boxShadow: `0 1px 2px ${alpha(theme.palette.common.black, 0.05)}`,
      "&:hover": {
        backgroundColor: theme.palette.secondary.light,
        color: theme.palette.common.black,
      },
      "&.Mui-disabled": {
        color: theme.palette.common.black,
      },
    },
    "&.MuiButton-containedError": {
      backgroundColor: theme.palette.error.main,
      borderColor: theme.palette.error.main,
      "&:hover": {
        backgroundColor: theme.palette.error.dark,
        borderColor: theme.palette.error.dark,
      },
    },
    "&.MuiButton-containedWarning": {
      backgroundColor: theme.palette.warning.main,
      borderColor: theme.palette.warning.main,
      "&:hover": {
        backgroundColor: theme.palette.warning.dark,
        borderColor: theme.palette.warning.dark,
      },
    },
    "&.MuiButton-containedSuccess": {
      backgroundColor: theme.palette.success.main,
      borderColor: theme.palette.success.main,
      "&:hover": {
        backgroundColor: theme.palette.success.dark,
        borderColor: theme.palette.success.dark,
      },
    },
  },

  // Outlined variant
  "&.MuiButton-outlined": {
    backgroundColor: "transparent",
    boxShadow: `0 1px 2px ${alpha(theme.palette.common.black, 0.05)}`,
    "&:hover, &:focus": {
      backgroundColor: "transparent",
      boxShadow: `0 1px 6px 2px ${alpha(theme.palette.common.black, 0.1)}`,
    },
    "&.MuiButton-outlinedPrimary": {
      color: "#069668",
      border: `1px solid ${"#069668"}`,
      "&.Mui-disabled": {
        color: theme.palette.primary.main,
      },
    },
    "&.MuiButton-outlinedSecondary": {
      color: theme.palette.secondary.main,
      border: `1px solid ${theme.palette.secondary.main}`,
      "&:hover": {
        borderColor: theme.palette.secondary.dark,
      },
      "&.Mui-disabled": {
        color: theme.palette.secondary.main,
      },
    },
    "&.MuiButton-outlinedError": {
      color: theme.palette.error.main,
      border: `1px solid ${theme.palette.error.main}`,
      "&:hover": {
        borderColor: theme.palette.error.dark,
      },
      "&.Mui-disabled": {
        color: theme.palette.error.main,
      },
    },
    "&.MuiButton-outlinedWarning": {
      color: theme.palette.warning.main,
      border: `1px solid ${theme.palette.warning.main}`,
      "&:hover": {
        borderColor: theme.palette.warning.dark,
      },
      "&.Mui-disabled": {
        color: theme.palette.warning.main,
      },
    },
    "&.MuiButton-outlinedSuccess": {
      color: theme.palette.success.main,
      border: `1px solid ${theme.palette.success.main}`,
      "&:hover": {
        borderColor: theme.palette.success.dark,
      },
      "&.Mui-disabled": {
        color: theme.palette.success.main,
      },
    },
  },

  // Text variant
  "&.MuiButton-text": {
    backgroundColor: "transparent",
    border: "none",
    boxShadow: "none",
    "&.MuiButton-textPrimary": {
      color:  "#069668",
      "&.Mui-disabled": {
        color: theme.palette.primary.main,
      },
    },
    "&.MuiButton-textSecondary": {
      color:  "#069668",
      "&.Mui-disabled": {
        color:  "#069668",
      },
    },
    "&.MuiButton-textError": {
      color: theme.palette.error.main,
      "&.Mui-disabled": {
        color: theme.palette.error.main,
      },
    },
    "&.MuiButton-textWarning": {
      color: theme.palette.warning.main,
      "&.Mui-disabled": {
        color: theme.palette.warning.main,
      },
    },
    "&.MuiButton-textSuccess": {
      color: theme.palette.success.main,
      "&.Mui-disabled": {
        color: theme.palette.success.main,
      },
    },
  },

  // Subtle variant (custom)
  "&.subtle": {
    border: `1px solid ${theme.palette.grey[100]}`,
    boxShadow: `0 1px 3px ${alpha(theme.palette.common.black, 0.05)}`,
    backgroundColor: theme.palette.secondary.light,
    "&:hover": {
      backgroundColor: theme.palette.secondary.light,
      boxShadow: `0 1px 3px ${alpha(theme.palette.common.black, 0.1)}`,
    },
    "&:focus": {
      backgroundColor: theme.palette.secondary.light,
      boxShadow: "none",
    },
    "&.subtle-primary": {
      color: theme.palette.primary.main,
      "&.Mui-disabled": {
        color: theme.palette.primary.main,
      },
    },
    "&.subtle-secondary": {
      color: theme.palette.common.black,
      "&.Mui-disabled": {
        color: theme.palette.common.black,
      },
    },
    "&.subtle-error": {
      color: theme.palette.error.main,
      "&.Mui-disabled": {
        color: theme.palette.error.main,
      },
    },
    "&.subtle-warning": {
      color: theme.palette.warning.main,
      "&.Mui-disabled": {
        color: theme.palette.warning.main,
      },
    },
    "&.subtle-success": {
      color: theme.palette.success.main,
      "&.Mui-disabled": {
        color: theme.palette.success.main,
      },
    },
  },

  // Link variant (custom)
  "&.link": {
    borderColor: "transparent",
    boxShadow: "none",
    paddingLeft: 0,
    paddingRight: 0,
    minWidth: "initial",
    backgroundColor: "transparent",
    "& .MuiButton-startIcon": {
      marginLeft: 0,
    },
    "& .MuiButton-endIcon": {
      marginRight: 0,
    },
    "&:hover": {
      opacity: 0.6,
      backgroundColor: "transparent",
      boxShadow: "none",
    },
    "&:focus": {
      backgroundColor: "transparent",
      boxShadow: "none",
    },
    "&.link-primary": {
      color:  "#069668",
      "&.Mui-disabled": {
        color:  "#069668",
        borderColor: "transparent",
        boxShadow: "none",
      },
    },
    "&.link-secondary": {
      color: theme.palette.common.black,
      "&.Mui-disabled": {
        color: theme.palette.common.black,
        borderColor: "transparent",
        boxShadow: "none",
      },
    },
    "&.link-error": {
      color: theme.palette.error.main,
      "&.Mui-disabled": {
        color: theme.palette.error.main,
        borderColor: "transparent",
        boxShadow: "none",
      },
    },
    "&.link-warning": {
      color: theme.palette.warning.main,
      "&.Mui-disabled": {
        color: theme.palette.warning.main,
        borderColor: "transparent",
        boxShadow: "none",
      },
    },
    "&.link-success": {
      color: theme.palette.success.main,
      "&.Mui-disabled": {
        color: theme.palette.success.main,
        borderColor: "transparent",
        boxShadow: "none",
      },
    },
  },

  // Size variants
  "&.MuiButton-sizeSmall": {
    padding: theme.spacing(0.375, 2),
    gap: theme.spacing(0.75),
    "& .MuiButton-startIcon, & .MuiButton-endIcon": {
      "& > *:first-of-type": {
        fontSize: theme.spacing(1.75),
      },
    },
    "&.link": {
      padding: theme.spacing(0.375, 0),
    },
  },

  // Tiny size (mapped to small in MUI but with custom styling)
  "&.tiny": {
    padding: theme.spacing(0, 1.5),
    gap: theme.spacing(0.5),
    "& .MuiButton-startIcon, & .MuiButton-endIcon": {
      "& > *:first-of-type": {
        fontSize: theme.spacing(1.5),
      },
    },
    "&.link": {
      padding: 0,
    },
  },

  // Pill variant
  "&.pill": {
    borderRadius: theme.spacing(3.125),
  },

  // Full width
  "&.MuiButton-fullWidth": {
    width: "100%",
  },
}));
