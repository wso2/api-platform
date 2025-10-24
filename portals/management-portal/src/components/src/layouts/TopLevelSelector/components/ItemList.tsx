import React from 'react';
import { Box, List, ListItem, ListItemButton, ListItemText, Typography } from '@mui/material';
import type { LevelItem } from '../utils';

interface ItemListProps {
    title: string;
    items: LevelItem[];
    selectedItemId?: string;
    onSelect: (item: LevelItem) => void;
}

/**
 * List component for displaying items in the TopLevelSelector popover
 */
export const ItemList: React.FC<ItemListProps> = ({
    title,
    items,
    selectedItemId,
    onSelect,
}) => (
    <Box display="flex" flexDirection="column">
        <Typography variant="body2" color="text.secondary">
            {title}
        </Typography>
        <List>
            {items.map((item) => (
                <ListItem disablePadding key={item.id}>
                    <ListItemButton
                        onClick={(e) => {
                            e.stopPropagation();
                            e.preventDefault();
                            onSelect(item);
                        }}
                        selected={item.id === selectedItemId}
                        disableRipple
                    >
                        <ListItemText primary={item.label} />
                    </ListItemButton>
                </ListItem>
            ))}
        </List>
    </Box>
); 