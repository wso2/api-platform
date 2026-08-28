/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { Box } from '@wso2/oxygen-ui';

import type { Project } from '../../../../api/resources/projects';
import { ProjectCard } from '../../../../components/cards/ProjectCard';

type ProjectsGridProps = {
  projects: Project[];
  orgHandle: string;
  onOpen: (project: Project) => void;
  onDelete?: (project: Project) => void;
};

/**
 * Renders exactly the projects it is given. Paging lives on the page, which
 * owns the request — slicing here as well would page an already-paged response.
 */
export function ProjectsGrid({
  projects,
  orgHandle,
  onOpen,
  onDelete,
}: ProjectsGridProps) {
  return (
    <Box
      sx={{
        display: 'grid',
        gap: 2.5,
        gridTemplateColumns: {
          xs: '1fr',
          sm: 'repeat(2, 1fr)',
          md: 'repeat(3, 1fr)',
          lg: 'repeat(4, 1fr)',
        },
        // Allow cards to shrink so long text does not widen the grid.
        '& > *': { minWidth: 0 },
      }}
    >
      {projects.map((project) => (
        <ProjectCard
          key={project.id}
          onDelete={onDelete}
          onOpen={onOpen}
          orgHandle={orgHandle}
          project={project}
        />
      ))}
    </Box>
  );
}
