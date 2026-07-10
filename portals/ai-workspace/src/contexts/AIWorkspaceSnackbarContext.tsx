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

import { ReactNode, createContext, useCallback, useContext, useMemo, useState } from 'react';
import { Snackbar } from '@wso2/oxygen-ui';

type SnackbarContent = ReactNode;

export type AIWorkspaceSnackbarOptions = {
  autoHideDuration?: number;
};

type AIWorkspaceSnackbarContextValue = {
  enqueueSnackbar: (content: SnackbarContent, options?: AIWorkspaceSnackbarOptions) => void;
  closeSnackbar: () => void;
};

const AIWorkspaceSnackbarContext = createContext<AIWorkspaceSnackbarContextValue | null>(null);

export function AIWorkspaceSnackbarProvider({ children }: { children: ReactNode }) {
  const [snackbarState, setSnackbarState] = useState<{
    open: boolean;
    content: SnackbarContent;
    autoHideDuration: number;
    key: number;
  }>({
    open: false,
    content: null,
    autoHideDuration: 3500,
    key: 0,
  });

  const enqueueSnackbar = useCallback(
    (content: SnackbarContent, options?: AIWorkspaceSnackbarOptions) => {
      setSnackbarState({
        open: true,
        content,
        autoHideDuration: options?.autoHideDuration ?? 3500,
        key: Date.now(),
      });
    },
    []
  );

  const closeSnackbar = useCallback(() => {
    setSnackbarState((prev) => ({ ...prev, open: false }));
  }, []);

  const value = useMemo(
    () => ({
      enqueueSnackbar,
      closeSnackbar,
    }),
    [enqueueSnackbar, closeSnackbar]
  );

  return (
    <AIWorkspaceSnackbarContext.Provider value={value}>
      {children}
      <Snackbar
        key={snackbarState.key}
        open={snackbarState.open}
        autoHideDuration={snackbarState.autoHideDuration}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        onClose={(_, reason) => {
          if (reason === 'clickaway') return;
          closeSnackbar();
        }}
        message={snackbarState.content}
        slotProps={{
          content: {
            sx: {
              bgcolor: 'transparent',
              backgroundColor: 'transparent',
              boxShadow: 'none',
              border: 'none',
              p: 0,
            },
          },
        }}
        sx={{
          bottom: { xs: 72, sm: 64 },
          '& .MuiSnackbarContent-root': {
            bgcolor: 'transparent',
            backgroundColor: 'transparent',
            boxShadow: 'none',
            border: 'none',
            p: 0,
          },
          '& .MuiPaper-root': {
            bgcolor: 'transparent',
            backgroundColor: 'transparent',
            boxShadow: 'none',
            border: 'none',
          },
          '& .MuiSnackbarContent-message': {
            p: 0,
            m: 0,
          },
        }}
      />
    </AIWorkspaceSnackbarContext.Provider>
  );
}

export function useAIWorkspaceSnackbarContext(): AIWorkspaceSnackbarContextValue {
  const context = useContext(AIWorkspaceSnackbarContext);
  if (!context) {
    throw new Error(
      'useAIWorkspaceSnackbarContext must be used within AIWorkspaceSnackbarProvider'
    );
  }
  return context;
}
