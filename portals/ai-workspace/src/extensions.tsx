/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 * Licensed under the Apache License, Version 2.0.
 */

import React, { createContext, useContext } from 'react';

export type AIWorkspaceExtension = {
  id: string;
  path: string;
  label: string;
  icon?: React.ReactNode;
  element: React.ReactNode;
};

const ExtensionsContext = createContext<readonly AIWorkspaceExtension[]>([]);

export function ExtensionsProvider({
  extensions,
  children,
}: {
  extensions: readonly AIWorkspaceExtension[];
  children: React.ReactNode;
}) {
  return (
    <ExtensionsContext.Provider value={extensions}>
      {children}
    </ExtensionsContext.Provider>
  );
}

export function useAIWorkspaceExtensions(): readonly AIWorkspaceExtension[] {
  return useContext(ExtensionsContext);
}
