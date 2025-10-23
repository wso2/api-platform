import React, { useMemo } from 'react';
import { Box, Divider } from '@mui/material';
import { ItemList } from './ItemList';
import { type LevelItem, Level } from '../utils';
import { SearchBar } from '../../../components/SearchBar';
import { Button } from '../../../components';
import { AddIcon } from '../../../Icons';

interface PopoverContentProps {
    search: string;
    onSearchChange: (value: string) => void;
    recentItems: LevelItem[];
    items: LevelItem[];
    selectedItem: LevelItem;
    onSelect: (item: LevelItem) => void;
    onCreateNew?: () => void;
    level: Level;
}

/**
 * Content component for the TopLevelSelector popover containing search, create button, and item lists
 */
export const PopoverContent: React.FC<PopoverContentProps> = ({
    search,
    onSearchChange,
    recentItems,
    items,
    selectedItem,
    onSelect,
    onCreateNew,
    level,
}) => {
    const filteredItems = useMemo(() => {
        if (!search.trim()) return items;
        return items.filter((item) =>
            item.label.toLowerCase().includes(search.toLowerCase())
        );
    }, [items, search]);

    const filteredRecentItems = useMemo(() => {
        if (!search.trim()) return recentItems;
        return recentItems.filter((item) =>
            item.label.toLowerCase().includes(search.toLowerCase())
        );
    }, [recentItems, search]);

    return (
        <Box display="flex" flexDirection="column" gap={1} p={1}>
            <SearchBar
                inputValue={search}
                onChange={onSearchChange}
                testId="top-level-selector-search"
                placeholder="Search"
            />

            {onCreateNew && (
                <Box display="flex" gap={1}>
                    <Button
                        variant="text"
                        startIcon={<AddIcon fontSize="inherit" />}
                        onClick={onCreateNew}
                        disableRipple
                    >
                        Create {level}
                    </Button>
                </Box>
            )}

            {filteredRecentItems.length > 0 && (
                <>
                    <Divider />
                    <ItemList
                        title="Recent"
                        items={filteredRecentItems}
                        onSelect={onSelect}
                    />
                </>
            )}

            {filteredItems.length > 0 && (
                <>
                    <Divider />
                    <ItemList
                        title={`All {level}s`}
                        items={filteredItems}
                        selectedItemId={selectedItem.id}
                        onSelect={onSelect}
                    />
                </>
            )}
        </Box>
    );
}; 