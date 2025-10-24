import React, { useState, useCallback } from 'react';
import {
  StyledPopover,
  StyledTopLevelSelector,
} from './TopLevelSelector.styled';
import { Box } from '@mui/material';
import { SelectorHeader, SelectorContent, PopoverContent } from './components';
import { type LevelItem, Level } from './utils';

export interface TopLevelSelectorProps {
  className?: string;
  items: LevelItem[];
  recentItems?: LevelItem[];
  selectedItem: LevelItem;
  level: Level;
  isHighlighted?: boolean;
  disabled?: boolean;
  onSelect: (item: LevelItem) => void;
  onClick: (level: Level) => void;
  onClose?: () => void;
  onCreateNew?: () => void;
}

/**
 * TopLevelSelector component for selecting items at different levels (Organization, Project, Component)
 * @component
 */
export const TopLevelSelector = React.forwardRef<
  HTMLDivElement,
  TopLevelSelectorProps
>(
  (
    {
      items = [],
      selectedItem,
      onSelect,
      isHighlighted = false,
      disabled = false,
      onClick,
      level,
      recentItems = [],
      onClose,
      onCreateNew,
      className,
    },
    ref
  ) => {
    const [search, setSearch] = useState('');
    const [anchorEl, setAnchorEl] = useState<HTMLButtonElement | null>(null);
    const open = Boolean(anchorEl);

    const handleClick = useCallback(() => {
      if (!disabled) {
        onClick?.(level);
      }
    }, [disabled, onClick, level]);

    const handleSelect = useCallback((item: LevelItem) => {
      if (!disabled) {
        onSelect(item);
        setAnchorEl(null);
      }
    }, [disabled, onSelect]);

    const handleOpen = useCallback((event: React.MouseEvent<HTMLButtonElement>) => {
      event.stopPropagation();
      event.preventDefault();
      setAnchorEl(event.currentTarget);
    }, []);

    const handleClose = useCallback(() => {
      setAnchorEl(null);
      setSearch('');
      onClose?.();
    }, [onClose]);

    const handleSearchChange = useCallback((value: string) => {
      setSearch(value);
    }, []);

    const handleCreateNew = useCallback(() => {
      onCreateNew?.();
      setAnchorEl(null);
    }, [onCreateNew]);

    return (
      <StyledTopLevelSelector
        ref={ref}
        onClick={handleClick}
        disabled={disabled}
        variant="outlined"
        isHighlighted={isHighlighted}
        className={className}
        role="button"
        tabIndex={disabled ? -1 : 0}
        aria-label={`${level} selector`}
        aria-expanded={open}
        aria-haspopup="listbox"
      >
        <Box display="flex" flexDirection="column">
          <SelectorHeader level={level} onClose={onClose} />
          <SelectorContent selectedItem={selectedItem} onOpen={handleOpen} disableMenu={items.length === 0} />
        </Box>

        <StyledPopover
          id={`${level}-popover`}
          open={open}
          anchorEl={anchorEl}
          onClose={handleClose}
          anchorOrigin={{
            vertical: 'bottom',
            horizontal: 'left',
          }}
          transformOrigin={{
            vertical: 'top',
            horizontal: 'left',
          }}
          role="listbox"
          aria-label={`${level} options`}
        >
          <PopoverContent
            search={search}
            onSearchChange={handleSearchChange}
            recentItems={recentItems}
            items={items}
            selectedItem={selectedItem}
            onSelect={handleSelect}
            onCreateNew={handleCreateNew}
            level={level}
          />
        </StyledPopover>
      </StyledTopLevelSelector>
    );
  }
);

TopLevelSelector.displayName = 'TopLevelSelector';
