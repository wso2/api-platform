import { Box } from '@wso2/oxygen-ui';
import { Boxes } from '@wso2/oxygen-ui-icons-react';

/**
 * Neutral icon tile for an API — same treatment as the gateway card's icon
 * tile so the console's cards read as one family. Shared by the API card
 * (grid view) and the API list rows.
 */
export function KindIconTile({ size = 36 }: { size?: number }) {
  return (
    <Box
      sx={{
        alignItems: 'center',
        bgcolor: 'action.hover',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 1.5,
        color: 'text.secondary',
        display: 'flex',
        flexShrink: 0,
        height: size,
        justifyContent: 'center',
        width: size,
      }}
    >
      <Boxes size={size >= 44 ? 22 : 20} />
    </Box>
  );
}
