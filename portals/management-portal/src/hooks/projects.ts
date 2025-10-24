import { useCallback } from "react";
import { getApiConfig } from "./apiConfig";

export type Project = {
  id: string;
  name: string;
  description: string;
  organizationId: string;
  createdAt: string;
  updatedAt: string;
};

type ProjectListResponse = {
  count: number;
  list: Project[];
};

const DEFAULT_PROJECT_NAME = "AI APIs";
const DEFAULT_PROJECT_DESCRIPTION = "Initial project created via management portal.";

export const useProjectsApi = () => {
  const createProject = useCallback(
    async (
      name: string = DEFAULT_PROJECT_NAME,
      description: string = DEFAULT_PROJECT_DESCRIPTION
    ): Promise<Project> => {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(`${baseUrl}/api/v1/projects`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ name, description }),
      });

      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(
          `Failed to create project: ${response.status} ${response.statusText} ${errorBody}`
        );
      }

      const data: Project = await response.json();
      return data;
    },
    []
  );

  const fetchProject = useCallback(async (projectId: string): Promise<Project> => {
    const { token, baseUrl } = getApiConfig();

    const response = await fetch(`${baseUrl}/api/v1/projects/${projectId}`, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    if (!response.ok) {
      const errorBody = await response.text();
      throw new Error(
        `Failed to fetch project ${projectId}: ${response.status} ${response.statusText} ${errorBody}`
      );
    }

    const data: Project = await response.json();
    return data;
  }, []);

  const fetchProjects = useCallback(
    async (): Promise<Project[]> => {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(`${baseUrl}/api/v1/projects`, {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (response.status === 404) {
        return [];
      }

      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(
          `Failed to fetch projects: ${response.status} ${response.statusText} ${errorBody}`
        );
      }

      const data = await response.json();
      if (Array.isArray(data)) {
        return data;
      }
      if (data && Array.isArray((data as ProjectListResponse).list)) {
        return (data as ProjectListResponse).list;
      }
      return [];
    },
    []
  );

  const deleteProject = useCallback(async (projectId: string): Promise<void> => {
    const { token, baseUrl } = getApiConfig();

    const response = await fetch(`${baseUrl}/api/v1/projects/${projectId}`, {
      method: "DELETE",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    if (!response.ok) {
      const errorBody = await response.text();
      throw new Error(
        `Failed to delete project ${projectId}: ${response.status} ${response.statusText} ${errorBody}`
      );
    }
  }, []);

  return {
    createProject,
    fetchProject,
    fetchProjects,
    deleteProject,
  };
};
