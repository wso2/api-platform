import { styled } from '@mui/material/styles';
import TextField, { type TextFieldProps } from '@mui/material/TextField';
import FormControl from '@mui/material/FormControl';
import type { FormControlProps } from '@mui/material/FormControl';
import { alpha, type Theme } from '@mui/material';
import type { TextInputProps } from './TextInput';
import { type ComponentType } from 'react';

type CustomTextFieldProps = TextFieldProps & {
  customSize?: 'small' | 'medium' | 'large';
};

const getSize = (size: CustomTextFieldProps['customSize'], theme: Theme) => {
  switch (size) {
    case 'small':
      return {
        '& .MuiOutlinedInput-root': {
          minHeight: theme.spacing(4),
        },
        '& .MuiOutlinedInput-input': {
          padding: theme.spacing(1, 1.5),
        },
      };
    case 'large':
      return {
        '& .MuiOutlinedInput-root': {
          minHeight: theme.spacing(7),
          borderRadius: 12,
        },
        '& .MuiOutlinedInput-input': {
          padding: theme.spacing(2, 2),
        },
      };
    case 'medium':
    default:
      return {
        '& .MuiOutlinedInput-root': {
          minHeight: theme.spacing(5),
        },
        '& .MuiOutlinedInput-input': {
          padding: theme.spacing(1.5, 2),
        },
      };
  }
};

export const StyledTextField: ComponentType<CustomTextFieldProps> = styled(
  TextField,
  {
    shouldForwardProp: (prop) => prop !== 'customSize',
  }
)<CustomTextFieldProps>(({ theme, customSize }) => ({
  ...getSize(customSize, theme),
  borderRadius: theme.spacing(0.625),

  '& .MuiOutlinedInput-root': {
    backgroundColor: theme.palette.background.paper,
    boxShadow: theme.shadows[20],
    borderRadius: theme.shape.borderRadius,
    border: 0,
    '&:active, &:focus': {
      backgroundColor: theme.palette.background.paper,
      boxShadow: theme.shadows[21],
    },
    '&:hover': {
      backgroundColor: theme.palette.background.paper,
      boxShadow: theme.shadows[21],
    },
    '& .MuiOutlinedInput-notchedOutline': {
      border: 0,
      backgroundColor: 'transparent !important',
    },
    '&.Mui-error': {
      backgroundColor: alpha(theme.palette.error.main, 0.05),
      borderRadius: theme.shape.borderRadius,
      boxShadow: theme.shadows[22],
    },
  },

  '& .MuiFormHelperText-root.Mui-error': {
    margin: 0,
    marginTop: theme.spacing(1),
    color: theme.palette.error.main,
    fontSize: theme.typography.caption.fontSize,
  },
  '.root': {
    padding: theme.spacing(0.5, 1.5),
    width: '100%',
    minHeight: theme.spacing(5),
    backgroundColor: theme.palette.common.white,
    border: `1px solid ${theme.palette.grey[100]}`,
    boxShadow: `0 1px 2px -1px ${alpha(
      theme.palette.common.black,
      0.08
    )}, 0 -3px 9px 0 ${alpha(theme.palette.common.black, 0.04)} inset`,
    borderRadius: 5,
    '&$multiline': {
      height: 'auto',
      resize: 'auto',
    },
    '&$multilineReadonly': {
      height: 'auto',
      resize: 'none',
      '& $textarea': {
        height: 'auto',
        resize: 'none',
      },
    },
    '&$multilineResizeIndicator': {
      height: 'auto',
      resize: 'none',
      '& $textarea': {
        height: 'auto',
        resize: 'none',
      },
    },
    '&$rounded': {
      paddingLeft: theme.spacing(2),
    },
    '&:hover': {
      borderColor: theme.palette.grey[200],
    },
  },
  '.rootSmall': {
    minHeight: theme.spacing(4),
  },
  '.rootLarge': {
    minHeight: theme.spacing(7),
    borderRadius: 12,
    padding: theme.spacing(1, 1, 1, 3),
  },
  textInput: (props: TextInputProps) => {
    const typographyVariant =
      props.typography === 'inherit' ? 'body1' : props.typography || 'body1';
    const typography =
      theme.typography[typographyVariant as keyof typeof theme.typography];
    return {
      minHeight: theme.spacing(2.5),
      padding: theme.spacing(0.125, 0),
      fontSize:
        typeof typography === 'object' && typography
          ? typography.fontSize
          : undefined,
      fontWeight:
        typeof typography === 'object' && typography
          ? typography.fontWeight
          : undefined,
      lineHeight:
        typeof typography === 'object' && typography
          ? typography.lineHeight
          : undefined,
    };
  },
  '.textInputDisabled': {
    '&:hover': {
      borderColor: theme.palette.grey[100],
    },
  },
  '.inputAdornedEnd': {
    '& .MuiInputAdornment-root': {
      marginRight: theme.spacing(-1),
    },
  },
  '.inputAdornedEndAlignTop': {
    '& .MuiInputAdornment-root': {
      alignSelf: 'flex-end',
      height: 'auto',
      marginBottom: theme.spacing(0.25),
    },
  },
  '.multiline': {},
  '.multilineReadonly': {},
  '.multilineResizeIndicator': {},
  '.rounded': {
    borderRadius: theme.spacing(2.5),
  },
  '.focused': {
    borderColor: theme.palette.primary.light,
    borderWidth: 1,
    boxShadow: `0 -3px 9px 0 ${alpha(
      theme.palette.common.black,
      0.04
    )} inset, 0 0 0 2px ${theme.palette.primary.light}`,
    '&:hover': {
      borderColor: theme.palette.primary.light,
    },
  },
  '.error': {
    background: theme.palette.error.light,
    borderColor: theme.palette.error.main,
    boxShadow: `0 0 0 1px ${theme.palette.error.light}, inset 0 2px 2px ${alpha(
      theme.palette.error.light,
      0.07
    )}`,
    '&:hover': {
      borderColor: theme.palette.error.main,
    },
  },
  '.readOnlyDefault': {
    boxShadow: `0 0 0 1px ${alpha(
      theme.palette.common.black,
      0.05
    )}, inset 0 2px 2px ${alpha(theme.palette.common.black, 0.05)}`,
    border: 'none',
    backgroundColor: theme.palette.secondary.light,
  },
  '.readOnlyPlain': {
    boxShadow: 'none',
    border: 'none',
    backgroundColor: theme.palette.common.white,
    paddingLeft: 0,
    paddingRight: 0,
  },
  '.inputGroup': {
    position: 'relative',
  },
  '.tooltipIcon': {
    display: 'flex',
    alignItems: 'center',
    color: theme.palette.secondary.main,
    cursor: 'help',
    fontSize: theme.spacing(1.75),
  },
  '.textarea': {
    resize: 'both',
  },
  '.copyToClipboardInput': {
    backgroundColor: theme.palette.secondary.light,
    border: `1px solid ${theme.palette.grey[100]}`,
    boxShadow: 'none',
    paddingRight: theme.spacing(5),
  },
  '.textInputInfoIcon': {
    display: 'flex',
    alignItems: 'center',
    fontSize: theme.spacing(1.75),
  },
}));

export const StyledFormControl: ComponentType<FormControlProps> = styled(
  FormControl
)({
  width: '100%',
});

export const HeadingWrapper: ComponentType<any> = styled('div')(
  ({ theme }: { theme: Theme }) => ({
    display: 'flex',
    width: '100%',
    alignItems: 'center',
    marginBottom: theme.spacing(0.5),

    '.formLabelAction': {
      marginLeft: 'auto',
      display: 'flex',
      alignItems: 'center',
    },
    '.formLabelInfo': {
      marginLeft: theme.spacing(1),
      display: 'flex',
      alignItems: 'center',
    },
    '.formOptional': {
      color: theme.palette.grey[200],
      fontSize: theme.spacing(1.4),
      marginLeft: theme.spacing(1),
    },
    '.formLabelTooltip': {
      marginLeft: theme.spacing(1),
      display: 'flex',
      alignItems: 'center',
    },
    '.tooltipIcon': {
      display: 'flex',
      alignItems: 'center',
      color: theme.palette.secondary.main,
      cursor: 'help',
      fontSize: theme.spacing(1.75),
    },
  })
);
