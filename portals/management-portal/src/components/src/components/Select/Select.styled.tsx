import { alpha, Box, type BoxProps, styled } from '@mui/material';
import type { ComponentType } from 'react';

export interface StyledSelectProps extends BoxProps {
  disabled?: boolean;
}

export const StyledSelect: ComponentType<StyledSelectProps> = styled(Box)(
  ({ theme }) => ({
    '.selectLabel': {
      display: 'block',
      color: theme.palette.secondary.dark,
      marginBottom: theme.spacing(0.5),
    },

    '.selectRoot': {
      '& .MuiAutocomplete-inputRoot[class*="MuiOutlinedInput-root"]': {
        '& .MuiAutocomplete-endAdornment': {
          right: theme.spacing(0.8),
        },
      },
      '& .MuiTextField-root': {
        '& .MuiInputBase-root': {
          height: theme.spacing(5),
          backgroundColor: theme.palette.common.white,
          paddingTop: theme.spacing(0.4),
          paddingBottom: theme.spacing(0.35),

          // Default box shadow for normal state
          boxShadow: `0px 1px 2px -1px ${alpha(
            theme.palette.common.black,
            0.08
          )}, 0px -3px 9px 0px ${alpha(
            theme.palette.common.black,
            0.04
          )} inset`,

          '& .MuiOutlinedInput-notchedOutline': {
            borderColor: theme.palette.grey[100],
          },
          '&.Mui-focused': {
            boxShadow: `0 -3px 9px 0 ${alpha(
              theme.palette.common.black,
              0.04
            )} inset, 0 0 0 2px ${theme.palette.primary.light}`,
            '& .MuiOutlinedInput-notchedOutline': {
              borderColor: theme.palette.primary.light,
            },
            '&.Mui-error .MuiOutlinedInput-notchedOutline': {
              borderColor: theme.palette.error.main,
            },
          },
          '&:hover': {
            '& .MuiOutlinedInput-notchedOutline': {
              borderColor: theme.palette.primary.light,
            },
            '&.Mui-error .MuiOutlinedInput-notchedOutline': {
              borderColor: theme.palette.error.main,
            },
            '&.Mui-focused': {
              '& .MuiOutlinedInput-notchedOutline': {
                borderColor: theme.palette.primary.light,
              },
            },
          },
          '&.MuiInputBase-marginDense': {
            height: theme.spacing(4),
          },
          '&.Mui-error': {
            backgroundColor: theme.palette.error.light,
            borderColor: theme.palette.error.main,
            boxShadow: 'transparent',
            '& .MuiOutlinedInput-notchedOutline': {
              borderColor: theme.palette.error.main,
            },
            '&.Mui-focused': {
              backgroundColor: theme.palette.error.light,
              boxShadow: 'none',
            },
            '&:hover': {
              boxShadow: 'none',
            },
          },
          '&.Mui-disabled': {
            backgroundColor: theme.palette.secondary.light,
            '& .MuiAutocomplete-endAdornment': {
              opacity: 0.5,
            },
            '&:hover': {
              '& .MuiOutlinedInput-notchedOutline': {
                borderColor: theme.palette.grey[100],
              },
              '&.Mui-error .MuiOutlinedInput-notchedOutline': {
                borderColor: theme.palette.error.main,
              },
              '&.Mui-focused': {
                '& .MuiOutlinedInput-notchedOutline': {
                  borderColor: theme.palette.grey[100],
                },
              },
            },
          },
        },
      },
      '& .MuiAutocomplete-endAdornment': {
        display: 'flex',
        alignItems: 'center',
        top: `calc(50% - ${theme.spacing(2)}px)`,
      },
      '& .MuiAutocomplete-popupIndicator': {
        padding: theme.spacing(0.75),
        marginRight: 0,
        color: alpha(theme.palette.common.black, 0.87),
        '&:hover': {
          backgroundColor: 'transparent',
        },
      },
    },
    '.formLabel': {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      marginBottom: theme.spacing(0.5),
      gap: theme.spacing(1),
      flexWrap: 'nowrap',
    },
    '.formLabelAction': {
      marginLeft: 'auto',
      display: 'flex',
      alignItems: 'center',
    },
    '.formLabelInfo': {
      display: 'flex',
      alignItems: 'center',
    },
    '.formOptional': {
      color: theme.palette.text.secondary,
      fontSize: theme.spacing(1.4),
    },
    '.formLabelTooltip': {
      display: 'flex',
      alignItems: 'center',
      color: theme.palette.text.secondary,
    },
    '.inputGroup': {
      position: 'relative',
    },
    '.tooltipIcon': {
      color: theme.palette.secondary.main,
      cursor: 'help',
      fontSize: theme.spacing(1.75),
      display: 'flex',
      alignItems: 'center',
    },
    '.listbox': {
      paddingTop: 0,
      paddingBottom: 0,
      '& .MuiAutocomplete-option[aria-selected="true"],& .MuiAutocomplete-option[data-focus="true"]':
        {
          backgroundColor: theme.palette.secondary.light,
        },
    },
    '.option': {
      padding: 0,
    },
    '.focused': {
      backgroundColor: theme.palette.secondary.light,
    },
    '.listItemContent': {
      width: '10%',
      padding: theme.spacing(1, 1.5),
      display: 'flex',
      alignItems: 'center',
    },
    '.listItemImgWrap': {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      width: theme.spacing(2.5),
      height: theme.spacing(2.5),
      marginRight: theme.spacing(1),
      flexDirection: 'row',
      flexShrink: 0,
      overflow: 'hidden',
    },
    '.listItemImg': {
      width: theme.spacing(2.5),
      height: theme.spacing(2.5),
      display: 'block',
      objectFit: 'cover',
      borderRadius: theme.spacing(0.25),
    },
    '.createButton': {
      justifyContent: 'flex-start',
      '&:hover': {
        backgroundColor: 'none !important',
      },
      '&.MuiButton-root:hover': {
        backgroundColor: 'none !important',
      },
    },
    '.popupIcon': {
      display: 'flex',
      '&:hover': {
        backgroundColor: 'transparent',
      },
    },
    '.clearIcon': {
      fontSize: theme.spacing(1.5),
      display: 'flex',
      paddingRight: theme.spacing(1),
      borderRight: `1px solid ${theme.palette.grey[100]}`,
    },
    '.clearIndicator': {
      '&:hover': {
        backgroundColor: 'transparent',
      },
    },
    '.loadingTextLoader': {
      marginRight: theme.spacing(1),
    },
    '.startAdornment': {
      paddingLeft: theme.spacing(0.5),
      '& svg': {
        fontSize: theme.spacing(1.8),
      },
    },
    '.selectInfoIcon': {
      display: 'flex',
      alignItems: 'center',
      fontSize: theme.spacing(1.75),
    },
  })
);
