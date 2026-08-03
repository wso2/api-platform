import { Box } from '@wso2/oxygen-ui';

import type { Api } from '../../types/domain';
import { ApiCard } from './ApiCard';

type ApiCardGridProps = {
  components: Api[];
  onOpen: (component: Api) => void;
  onDelete?: (component: Api) => void;
};

/** Auto-fill card grid, same density as the gateways page. */
export function ApiCardGrid({
  components,
  onOpen,
  onDelete,
}: ApiCardGridProps) {
  return (
    <Box
      sx={{
        display: 'grid',
        gap: 2.5,
        gridTemplateColumns: 'repeat(auto-fill, minmax(360px, 1fr))',
      }}
    >
      {components.map((component) => (
        <ApiCard
          component={component}
          key={component.id}
          onDelete={onDelete}
          onOpen={onOpen}
        />
      ))}
    </Box>
  );
}
