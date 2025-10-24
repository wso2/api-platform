import {
  Box,
  CircularProgress,
  FormHelperText,
  InputAdornment,
  Popper,
  TextField,
  type TooltipProps,
  Typography,
  Autocomplete as MUIAutocomplete,
  type AutocompleteProps,
  type AutocompleteRenderInputParams,
} from '@mui/material';
import React, { useMemo } from 'react';
import clsx from 'clsx';
import { IconButton } from '../IconButton';
import { Button } from '../Button';
import { Tooltip } from '../Tooltip';
import { StyledSelect } from './Select.styled';
import Question from '../../Icons/generated/Question';
import Close from '../../Icons/generated/Close';
import ChevronDown from '../../Icons/generated/ChevronDown';
import { AddIcon } from '../../Icons/generated';
import Info from '../../Icons/generated/Info';

interface ISelectProps<T>
  extends Omit<
    AutocompleteProps<T, false, boolean, false>,
    'renderInput' | 'onChange' | 'renderOption'
  > {
  name: string;
  label?: string;
  labelId: string;
  tooltip?: React.ReactNode;
  tooltipPlacement?: TooltipProps['placement'];
  helperText?: React.ReactNode;
  loadingText?: string;
  error?: boolean;
  optional?: boolean;
  addBtnText?: string;
  onAddClick?: () => void;
  getOptionLabel: (val: string | T) => string;
  getOptionIcon?: (val: T) => string;
  onChange: (val: T | null) => void;
  value: T | null;
  getOptionValue?: (val: T) => string | number;
  InputProps?: Partial<AutocompleteRenderInputParams['InputProps']>;
  startIcon?: React.ReactNode;
  info?: React.ReactNode;
  actions?: React.ReactNode;
  renderOption?: (optionVal: T) => React.ReactNode;
  enableOverflow?: boolean;
  getOptionDisabled?: (option: T) => boolean;
  testId: string;
  isClearable?: boolean;
  isLoading?: boolean;
  getOptionSelected?: (option: T, value: T) => boolean;
  placeholder?: string;
}

function SelectComponent<T>(props: ISelectProps<T>) {
  const {
    name,
    label,
    labelId,
    options,
    error,
    getOptionLabel,
    getOptionIcon,
    helperText,
    loadingText,
    placeholder,
    disabled,
    tooltip,
    optional,
    tooltipPlacement = 'right',
    size = 'medium',
    onAddClick,
    addBtnText,
    value,
    onChange,
    getOptionValue = getOptionLabel,
    InputProps,
    getOptionSelected,
    startIcon,
    onBlur,
    info,
    actions,
    renderOption,
    enableOverflow,
    getOptionDisabled,
    testId,
    isClearable,
    isLoading,
  } = props;

  const CreateAction: T = Symbol.for('selectCreateAction') as any;

  const classes = {
    selectRoot: 'selectRoot',
    listbox: 'listbox',
    option: 'option',
    clearIndicator: 'clearIndicator',
    loadingIcon: 'loadingIcon',
    createButton: 'createButton',
    listItemContent: 'listItemContent',
    listItemImgWrap: 'listItemImgWrap',
    listItemImg: 'listItemImg',
    startAdornment: 'startAdornment',
    selectInfoIcon: 'selectInfoIcon',
    loadingTextLoader: 'loadingTextLoader',
    formLabel: 'formLabel',
    formLabelInfo: 'formLabelInfo',
    formLabelTooltip: 'formLabelTooltip',
    formOptional: 'formOptional',
    formLabelAction: 'formLabelAction',
    tooltipIcon: 'tooltipIcon',
    popupIcon: 'popupIcon',
  };

  const toolTip = tooltip && (
    <Tooltip
      title={typeof tooltip === 'string' ? tooltip : ''}
      placement={tooltipPlacement as any}
      disabled={!tooltip}
    >
      <Box className={classes.tooltipIcon}>
        <Box className={classes.selectInfoIcon}>
          <Question fontSize="inherit" />
        </Box>
      </Box>
    </Tooltip>
  );

  const updateOptions = useMemo(() => {
    const updateValues = options ? options.slice() : [];
    if (addBtnText && onAddClick) {
      updateValues.unshift(CreateAction);
    }
    return updateValues;
  }, [options, addBtnText, onAddClick, CreateAction]);

  return (
    <StyledSelect data-testid={testId}>
      {(label || toolTip || info || optional || actions) && (
        <Box className={classes.formLabel}>
          <Box display="flex" alignItems="center" gap={1}>
            {label && (
              <Typography component="h6" variant="body1">
                {label}
              </Typography>
            )}
            {info && <Box className={classes.formLabelInfo}>{info}</Box>}
            {toolTip && (
              <Box className={classes.formLabelTooltip}>{toolTip}</Box>
            )}
            {optional && (
              <Typography variant="body2" className={classes.formOptional}>
                (Optional)
              </Typography>
            )}
          </Box>
          {actions && (
            <Box
              sx={{ ml: 'auto', display: 'flex', alignItems: 'center' }}
              className={classes.formLabelAction}
            >
              {actions}
            </Box>
          )}
        </Box>
      )}
      <MUIAutocomplete<T, false, boolean, false>
        classes={{
          root: classes.selectRoot,
          listbox: classes.listbox,
          option: classes.option,
          clearIndicator: classes.clearIndicator,
          endAdornment: clsx({
            [classes.loadingIcon]: isLoading,
          }),
        }}
        clearIcon={
          <IconButton
            size="small"
            testId="selector-clear"
            variant="text"
            disableRipple={true}
            disableFocusRipple={true}
            disableTouchRipple={true}
          >
            <Close fontSize="inherit" color="secondary" />
          </IconButton>
        }
        id={labelId}
        data-cyid={`${testId}-select`}
        data-testid={testId}
        size={size}
        disabled={disabled || isLoading}
        disableClearable={!isClearable}
        options={updateOptions}
        value={value}
        slots={{
          popper: enableOverflow
            ? (popoverProps) => (
                <Popper
                  {...popoverProps}
                  style={{
                    ...popoverProps.style,
                    minWidth: popoverProps.style?.width,
                    width: 'auto',
                    zIndex: 1300,
                  }}
                  placement="bottom-start"
                />
              )
            : undefined,
        }}
        getOptionLabel={(optionVal) => {
          if (CreateAction === optionVal) {
            return '';
          }
          return getOptionLabel(optionVal);
        }}
        popupIcon={
          <Box className={classes.popupIcon}>
            {isLoading ? (
              <CircularProgress size={16} />
            ) : (
              <IconButton
                size="small"
                testId="selector-dropdown"
                variant="text"
                className={classes.popupIcon}
                disableRipple={true}
                disableFocusRipple={true}
                disableTouchRipple={true}
              >
                <ChevronDown fontSize="inherit" color="secondary" />
              </IconButton>
            )}
          </Box>
        }
        onChange={(_: any, val) => {
          if (val === null) {
            if (isClearable) {
              onChange(null);
            }
            return;
          }
          if (CreateAction === val) {
            if (onAddClick) {
              onAddClick();
            }
            return;
          }
          onChange(val as T);
        }}
        onBlur={onBlur}
        isOptionEqualToValue={(optionVal, val) => {
          if (CreateAction === optionVal) {
            return false;
          }
          if (getOptionSelected) {
            return getOptionSelected(optionVal, val);
          }
          return getOptionValue(optionVal) === getOptionValue(val);
        }}
        getOptionDisabled={getOptionDisabled}
        renderOption={(renderProps, optionVal) => {
          const labelVal = getOptionLabel(optionVal);
          const itemIcon = getOptionIcon && getOptionIcon(optionVal);

          if (CreateAction === optionVal) {
            return (
              <li {...renderProps} key="create-action">
                <Button
                  fullWidth
                  onClick={onAddClick}
                  variant="text"
                  className={classes.createButton}
                  startIcon={<AddIcon />}
                  testId={`${testId}-create-button`}
                >
                  {addBtnText}
                </Button>
              </li>
            );
          }

          return (
            <li {...renderProps} key={labelVal}>
              <Box className={classes.listItemContent}>
                {renderOption ? (
                  renderOption(optionVal)
                ) : (
                  <Box
                    className={classes.listItemImgWrap}
                    display="flex"
                    flexDirection="row"
                    alignItems="center"
                    gap={1}
                  >
                    {itemIcon && (
                      <Box className={classes.listItemImgWrap}>
                        <img
                          className={classes.listItemImg}
                          src={itemIcon}
                          alt={labelVal}
                          height={10}
                          width={10}
                        />
                      </Box>
                    )}
                    <Typography variant="body2" noWrap>
                      {labelVal}
                    </Typography>
                  </Box>
                )}
              </Box>
            </li>
          );
        }}
        renderInput={({ InputProps: _InputProps, ...params }) => (
          <TextField
            InputProps={{
              ..._InputProps,
              ...InputProps,
              startAdornment: startIcon && (
                <InputAdornment
                  className={classes.startAdornment}
                  position="start"
                >
                  {startIcon}
                </InputAdornment>
              ),
            }}
            name={name}
            {...params}
            variant="outlined"
            error={error}
            disabled={disabled || isLoading}
            placeholder={placeholder}
            size={size}
          />
        )}
      />
      {helperText && (
        <FormHelperText error={error}>
          <Box display="flex" alignItems="center">
            <Box className={classes.selectInfoIcon}>
              <Info fontSize="inherit" />
            </Box>
            <Box ml={1}>{helperText}</Box>
          </Box>
        </FormHelperText>
      )}
      {loadingText && (
        <FormHelperText>
          <Box display="flex" alignItems="center">
            <CircularProgress size={12} className={classes.loadingTextLoader} />
            <Box ml={1}>{loadingText}</Box>
          </Box>
        </FormHelperText>
      )}
    </StyledSelect>
  );
}

export const Select = React.forwardRef<any, ISelectProps<any>>(
  SelectComponent
) as <T>(
  props: ISelectProps<T> & { ref?: React.Ref<any> }
) => React.ReactElement & {
  displayName?: string;
};

(Select as any).displayName = 'Select';
