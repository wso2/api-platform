import React from 'react';
import { Box, Typography } from '@mui/material';
import { type LevelItem } from '../utils';
import { IconButton } from '../../../components';
import { ChevronDownIcon } from '../../../Icons';


interface SelectorContentProps {
    selectedItem: LevelItem;
    onOpen: (event: React.MouseEvent<HTMLButtonElement>) => void;
    disableMenu?: boolean;
}

/**
 * Content component for the TopLevelSelector showing the selected item and dropdown button
 */
export const SelectorContent: React.FC<SelectorContentProps> = ({
    selectedItem,
    onOpen,
    disableMenu = false,
}) => (
    <Box display="flex" alignItems="center" gap={1} marginRight={5}>
        <Typography
            variant="body1"
            fontSize={14}
            fontWeight={450}
            color="text.primary"
        >
            {selectedItem.label}
        </Typography>
        {!disableMenu && <IconButton
            size="tiny"
            disableRipple
            onClick={onOpen}
            aria-label="Open selector menu"
        >
            <ChevronDownIcon />
        </IconButton>}
    </Box>
); 