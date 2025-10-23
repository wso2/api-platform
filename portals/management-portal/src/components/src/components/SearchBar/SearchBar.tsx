import React from 'react';
import { StyledSearchBar } from './SearchBar.styled';
import { SimpleSelect } from '../SimpleSelect';
import { SelectMenuItem } from '../SimpleSelect/SelectMenuItem/SelectMenuItem';
import { Box, InputBase } from '@mui/material';
import clsx from 'clsx';
import Search from '../../Icons/generated/Search';

export interface SearchBarProps {
  onChange: (v: string) => void;
  placeholder?: string;
  iconPlacement?: 'left' | 'right';
  size?: 'small' | 'medium';
  color?: 'secondary';
  keyDown?: React.KeyboardEventHandler<HTMLInputElement | HTMLTextAreaElement>;
  testId: string;
  onFilterChange?: (value: string) => void;
  filterValue?: string;
  filterItems?: { value: number; label: string }[];
  bordered?: boolean;
  inputValue?: string;
}

/**
 * SearchBar component
 * @component
 */
export const SearchBar = React.forwardRef<HTMLDivElement, SearchBarProps>(
  (
    {
      onChange,
      onFilterChange,
      filterValue,
      filterItems,
      testId,
      placeholder,
      iconPlacement = 'left',
      size,
      color,
      keyDown,
      bordered,
      inputValue,
      ...restProps
    },
    ref
  ) => {
    const handleOnChange = (
      e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>
    ) => {
      onChange(e.target.value);
    };

    const isFilter = filterItems && filterItems.length > 0;

    const getEndAdornment = () => {
      if (isFilter) {
        return (
          <div className="filterWrap">
            <SimpleSelect
              testId={`${testId}-filter`}
              value={filterValue}
              isSearchBarItem={true}
              onChange={(
                event:
                  | React.ChangeEvent<HTMLInputElement>
                  | (Event & { target: { value: unknown; name: string } })
              ) => {
                onFilterChange?.(event.target.value as string);
              }}
              resetStyles
              anchorOrigin={{
                vertical: 'bottom',
                horizontal: 'right',
              }}
              transformOrigin={{
                vertical: 'top',
                horizontal: 'right',
              }}
            >
              {filterItems?.map((item) => (
                <SelectMenuItem
                  key={item.value}
                  testId={`search-bar-filter-${item.value}`}
                  value={item.value}
                >
                  {item.label}
                </SelectMenuItem>
              ))}
            </SimpleSelect>
          </div>
        );
      }
      if (iconPlacement === 'right') {
        return (
          <Box className="searchIcon">
            <Search fontSize="small" />
          </Box>
        );
      }
    };

    return (
      <StyledSearchBar
        data-cyid={`${testId}-search-bar`}
        className="search"
        ref={ref}
        size={size}
        color={color}
        bordered={bordered}
        {...restProps}
      >
        <div className="search">
          <InputBase
            startAdornment={
              iconPlacement === 'left' && (
                <div className="searchIcon">
                  <Search fontSize="small" />
                </div>
              )
            }
            endAdornment={getEndAdornment()}
            placeholder={placeholder}
            inputProps={{ 'aria-label': 'search' }}
            onChange={handleOnChange}
            onKeyDown={keyDown}
            value={inputValue}
            data-cyid={`${testId}-search-bar-input`}
            className={clsx('inputRoot', {
              inputRootSecondary: color === 'secondary',
              inputRootBordered: bordered,
              inputRootFilter: isFilter,
            })}
            classes={{
              input: 'input',
            }}
          />
        </div>
      </StyledSearchBar>
    );
  }
);

SearchBar.displayName = 'SearchBar';
