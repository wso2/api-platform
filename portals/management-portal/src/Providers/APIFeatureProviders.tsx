import React, { type ReactNode } from "react";
import { ApiProvider } from "../context/ApiContext";
import { CreateComponentBuildpackProvider } from "../context/CreateComponentBuildpackContext";
import { GithubAPICreationProvider } from "../context/GithubAPICreationContext";
import { GithubProjectValidationProvider } from "../context/validationContext";

/**
 * Combines all API-related feature contexts:
 * - ApiProvider (manages APIs)
 * - CreateComponentBuildpackProvider (handles buildpack component creation)
 * - GithubAPICreationProvider (handles GitHub repo import & API creation)
 */
type Props = { children: ReactNode };

export const APIFeatureProviders: React.FC<Props> = ({ children }) => {
  return (
    <ApiProvider>
      <CreateComponentBuildpackProvider>
        <GithubAPICreationProvider>
          <GithubProjectValidationProvider>
            {children}
          </GithubProjectValidationProvider>
        </GithubAPICreationProvider>
      </CreateComponentBuildpackProvider>
    </ApiProvider>
  );
};
