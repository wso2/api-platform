import React, { useRef } from 'react';
import {
  StyledAutofocusField,
  StyledExpandableSearch,
} from './ExpandableSearch.styled';
import { InputBase } from '@mui/material';
import { IconButton } from '../../IconButton';
import Close from '../../../Icons/generated/Close';
import Search from '../../../Icons/generated/Search';

export interface AutofocusFieldProps {
  onChange: (data: any) => void;
  onClearClick: () => void;
  onBlur: (data: any) => void;
  searchQuery: string;
  inputReference: React.RefObject<HTMLInputElement | null>;
  size?: 'small' | 'medium';
  placeholder?: string;
  testId: string;
}

/**
 * SearchBar component
 * @component
 */
export const AutofocusField = React.forwardRef<
  HTMLDivElement,
  AutofocusFieldProps
>(({ ...props }, ref) => {
  return (
    <StyledAutofocusField
      ref={ref}
      size={props.size}
      onChange={props.onChange}
      onBlur={props.onBlur}
      className="search"
    >
      <InputBase
        inputRef={props.inputReference}
        value={props.searchQuery}
        endAdornment={
          props.searchQuery && (
            <IconButton
              onMouseDown={(e: React.MouseEvent<HTMLButtonElement>) => {
                props.onClearClick();
                e.preventDefault();
              }}
              color="secondary"
              size="tiny"
              data-testid="search-clear-icon"
              testId={`${props.testId}-clear`}
              variant="link"
            >
              <Close fontSize="inherit" />
            </IconButton>
          )
        }
        onChange={(e) => props.onChange(e.target.value)}
        onBlur={() => props.onBlur(props.searchQuery)}
        placeholder={props.placeholder || 'Search...'}
        className={`inputExpandable input${props.size ? props.size.charAt(0).toUpperCase() + props.size.slice(1) : ''}`}
        data-testid={props.testId}
        aria-label="text-field"
        data-cyid={`${props.testId}-search-field`}
        fullWidth
      />
    </StyledAutofocusField>
  );
});

AutofocusField.displayName = 'AutofocusField';

export interface ExpandableSearchProps {
  searchString: string;
  setSearchString: (value: string) => void;
  direction?: 'left' | 'right';
  placeholder?: string;
  testId: string;
  size?: 'small' | 'medium';
}

export const ExpandableSearch = React.forwardRef<
  HTMLDivElement,
  ExpandableSearchProps
>(({ ...props }, ref) => {
  const inputReference = useRef<HTMLInputElement>(null);
  const [isSearchShow, setSearchShow] = React.useState(false);

  const {
    searchString,
    setSearchString,
    direction = 'left',
    placeholder,
    testId,
    size = 'medium',
  } = props;

  const handleSearchFieldChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchString(e.target.value);
  };

  const handleSearchFieldBlur = (
    e: React.FocusEvent<HTMLInputElement | HTMLTextAreaElement>
  ) => {
    if (e.target.value === '') {
      setSearchShow(false);
    }
  };

  const onClearClick = () => {
    if (searchString === '') {
      setSearchShow(false);
    } else {
      setSearchString('');
    }
    inputReference?.current?.focus();
  };

  const onSearchClick = () => {
    setSearchShow(true);
    setTimeout(() => {
      inputReference?.current?.focus();
    }, 100);
  };

  return (
    <StyledExpandableSearch
      ref={ref}
      data-cyid={`${testId}-expandable-search`}
      direction={direction}
      isOpen={isSearchShow}
    >
      <div
        className={`expandableSearchCont ${isSearchShow ? 'expandableSearchContOpen' : ''}`}
      >
        {(direction === 'left' || (direction === 'right' && isSearchShow)) && (
          <IconButton
            onClick={onSearchClick}
            size="small"
            data-testid="search-icon"
            testId="search-icon"
            color="secondary"
            variant="text"
            disabled={isSearchShow}
            className="searchIconButton"
          >
            <Search fontSize="inherit" />
          </IconButton>
        )}

        <div
          className={`expandableSearchWrap ${isSearchShow ? 'expandableSearchWrapShow' : ''}`}
        >
          <InputBase
            inputRef={inputReference}
            value={searchString}
            onChange={handleSearchFieldChange}
            onBlur={handleSearchFieldBlur}
            placeholder={placeholder || 'Search...'}
            className={`inputExpandable input${size ? size.charAt(0).toUpperCase() + size.slice(1) : ''}`}
            data-testid={`${testId}-search-input`}
            aria-label="text-field"
            data-cyid={`${testId}-search-field`}
            fullWidth
            endAdornment={
              (isSearchShow || searchString) && (
                <IconButton
                  onMouseDown={(e: React.MouseEvent<HTMLButtonElement>) => {
                    onClearClick();
                    e.preventDefault();
                  }}
                  color="secondary"
                  size="small"
                  data-testid="search-clear-icon"
                  testId={`${testId}-clear`}
                  variant="link"
                  className="clearIconButton"
                >
                  <Close fontSize="inherit" />
                </IconButton>
              )
            }
          />
        </div>

        {direction === 'right' && !isSearchShow && (
          <IconButton
            onClick={onSearchClick}
            size="small"
            data-testid="search-icon"
            testId="search-icon"
            color="secondary"
            variant="text"
            disabled={isSearchShow}
            className="searchIconButton"
          >
            <Search fontSize="inherit" />
          </IconButton>
        )}
      </div>
    </StyledExpandableSearch>
  );
});

ExpandableSearch.displayName = 'ExpandableSearch';
