import { useState } from 'react';
import { Box, TablePagination } from '@wso2/oxygen-ui';

import type { Project } from '../../types/domain';
import { ProjectCard } from './ProjectCard';

type ProjectsGridProps = {
  projects: Project[];
  orgHandle: string;
  onOpen: (project: Project) => void;
  onDelete?: (project: Project) => void;
};

export function ProjectsGrid({
  projects,
  orgHandle,
  onOpen,
  onDelete,
}: ProjectsGridProps) {
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(12);
  const paged = projects.slice(
    page * rowsPerPage,
    page * rowsPerPage + rowsPerPage
  );

  return (
    <>
      <Box
        sx={{
          display: 'grid',
          gap: 2.5,
          gridTemplateColumns: 'repeat(auto-fill, minmax(330px, 1fr))',
        }}
      >
        {paged.map((project) => (
          <ProjectCard
            key={project.id}
            onDelete={onDelete}
            onOpen={onOpen}
            orgHandle={orgHandle}
            project={project}
          />
        ))}
      </Box>
      {projects.length > rowsPerPage && (
        <TablePagination
          component="div"
          count={projects.length}
          onPageChange={(_event, nextPage) => setPage(nextPage)}
          onRowsPerPageChange={(event) => {
            setRowsPerPage(parseInt(event.target.value, 10));
            setPage(0);
          }}
          page={page}
          rowsPerPage={rowsPerPage}
          rowsPerPageOptions={[12, 24, 48]}
          sx={{ mt: 2 }}
        />
      )}
    </>
  );
}
