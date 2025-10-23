import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useProjectsApi, type Project } from "../hooks/projects";
import { useOrganization } from "./OrganizationContext";

type ProjectContextValue = {
  projects: Project[];
  selectedProject: Project | null;
  loading: boolean;
  error: string | null;
  projectsLoaded: boolean;
  refreshProjects: () => Promise<Project[]>;
  createProject: (name?: string, description?: string) => Promise<Project>;
  fetchProjectById: (projectId: string) => Promise<Project>;
  deleteProject: (projectId: string) => Promise<void>;
  setSelectedProject: (project: Project | null) => void;
};

const ProjectContext = createContext<ProjectContextValue | undefined>(undefined);

type ProjectProviderProps = {
  children: ReactNode;
};

export const ProjectProvider = ({ children }: ProjectProviderProps) => {
  const {
    createProject: createProjectRequest,
    fetchProject,
    fetchProjects,
    deleteProject: deleteProjectRequest,
  } = useProjectsApi();
  const { organization, loading: organizationLoading } = useOrganization();

  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null);
  const [projectsLoaded, setProjectsLoaded] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const fetchedOrgIdRef = useRef<string | null>(null);
  const lastSetProjectIdRef = useRef<string | null>(null);

  const storageKey = useMemo(() => {
    if (!organization?.id) {
      return null;
    }
    return `selectedProjectId:${organization.id}`;
  }, [organization?.id]);

  const persistSelectedProjectId = useCallback(
    (projectId: string | null) => {
      if (!storageKey || typeof window === "undefined") {
        return;
      }

      if (projectId) {
        window.localStorage.setItem(storageKey, projectId);
      } else {
        window.localStorage.removeItem(storageKey);
      }
    },
    [storageKey]
  );

  const readStoredProjectId = useCallback(() => {
    if (!storageKey || typeof window === "undefined") {
      return null;
    }

    return window.localStorage.getItem(storageKey);
  }, [storageKey]);

  useEffect(() => {
    if (!storageKey) {
      setSelectedProjectId(null);
      lastSetProjectIdRef.current = null;
      return;
    }

    const storedId = readStoredProjectId();
    setSelectedProjectId(storedId);
    lastSetProjectIdRef.current = storedId;
  }, [readStoredProjectId, storageKey]);

  useEffect(() => {
    if (organizationLoading) {
      return;
    }

    if (!organization?.id) {
      setProjects([]);
      setSelectedProjectId(null);
      lastSetProjectIdRef.current = null;
      setProjectsLoaded(false);
      setLoading(false);
      setError(null);
      fetchedOrgIdRef.current = null;
      return;
    }

    const storedId = readStoredProjectId();
    setSelectedProjectId(storedId);
    lastSetProjectIdRef.current = storedId;
  }, [organization?.id, organizationLoading, readStoredProjectId]);

  useEffect(() => {
    if (!projectsLoaded) {
      return;
    }

    if (projects.length === 0) {
      if (selectedProjectId) {
        setSelectedProjectId(null);
        persistSelectedProjectId(null);
        lastSetProjectIdRef.current = null;
      }
      return;
    }

    if (selectedProjectId) {
      const exists = projects.some((project) => project.id === selectedProjectId);
      if (!exists) {
        const fallbackId = projects[0]?.id ?? null;
        setSelectedProjectId(fallbackId);
        persistSelectedProjectId(fallbackId);
        lastSetProjectIdRef.current = fallbackId;
      }
    }
  }, [projects, projectsLoaded, selectedProjectId, persistSelectedProjectId]);

  const refreshProjects = useCallback(async () => {
    if (!organization?.id) {
      setProjects([]);
      setSelectedProjectId(null);
      persistSelectedProjectId(null);
      setLoading(false);
      return [];
    }

    setProjectsLoaded(false);
    setLoading(true);
    setError(null);

    try {
      const result = await fetchProjects();
      setProjects(result);
      setProjectsLoaded(true);
      return result;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Unknown error occurred";
      setError(message);
      throw err;
    } finally {
      setLoading(false);
    }
  }, [fetchProjects, organization?.id, persistSelectedProjectId]);

  useEffect(() => {
    if (organizationLoading) {
      return;
    }

    const orgId = organization?.id ?? null;

    if (!orgId) {
      fetchedOrgIdRef.current = null;
      return;
    }

    if (fetchedOrgIdRef.current === orgId) {
      return;
    }

    fetchedOrgIdRef.current = orgId;
    refreshProjects().catch(() => {
      fetchedOrgIdRef.current = null;
    });
  }, [organization?.id, organizationLoading, refreshProjects]);

  const setSelectedProject = useCallback(
    (project: Project | null) => {
      const nextId = project?.id ?? null;
      if (lastSetProjectIdRef.current === nextId) {
        return;
      }
      lastSetProjectIdRef.current = nextId;
      setSelectedProjectId(nextId);
      persistSelectedProjectId(nextId);
    },
    [persistSelectedProjectId]
  );

  useEffect(() => {
    lastSetProjectIdRef.current = selectedProjectId;
  }, [selectedProjectId]);

  const createProject = useCallback(
    async (name?: string, description?: string) => {
      setLoading(true);
      setError(null);

      try {
        const project = await createProjectRequest(name, description);
        setProjects((prev) => [...prev, project]);
        setSelectedProject(project);
        return project;
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Unknown error occurred";
        setError(message);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [createProjectRequest, setSelectedProject]
  );

  const fetchProjectById = useCallback(
    async (projectId: string) => {
      setLoading(true);
      setError(null);

      try {
        const project = await fetchProject(projectId);
        setProjects((prev) => {
          const index = prev.findIndex((item) => item.id === project.id);
          if (index === -1) {
            return [...prev, project];
          }
          const next = [...prev];
          next[index] = project;
          return next;
        });
        setSelectedProject(project);
        return project;
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Unknown error occurred";
        setError(message);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [fetchProject, setSelectedProject]
  );

  const deleteProject = useCallback(
    async (projectId: string) => {
      setLoading(true);
      setError(null);

      try {
        await deleteProjectRequest(projectId);
        setProjects((prev) =>
          prev.filter((project) => project.id !== projectId)
        );
        setSelectedProjectId((prevId) => {
          if (prevId === projectId) {
            persistSelectedProjectId(null);
            return null;
          }
          return prevId;
        });
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Unknown error occurred";
        setError(message);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [deleteProjectRequest, setSelectedProject]
  );

  const selectedProject = useMemo(() => {
    if (!selectedProjectId) {
      return null;
    }
    return projects.find((project) => project.id === selectedProjectId) ?? null;
  }, [projects, selectedProjectId]);

  const value = useMemo<ProjectContextValue>(
    () => ({
      projects,
      selectedProject,
      loading,
      error,
      projectsLoaded,
      refreshProjects,
      createProject,
      fetchProjectById,
      deleteProject,
      setSelectedProject,
    }),
    [
      projects,
      selectedProject,
      loading,
      error,
      projectsLoaded,
      refreshProjects,
      createProject,
      fetchProjectById,
      deleteProject,
      setSelectedProject,
    ]
  );

  return (
    <ProjectContext.Provider value={value}>{children}</ProjectContext.Provider>
  );
};

export const useProjects = () => {
  const context = useContext(ProjectContext);

  if (!context) {
    throw new Error("useProjects must be used within a ProjectProvider");
  }

  return context;
};
