import { useState, useCallback } from 'react';

export type ActionStatus = 'idle' | 'pending' | 'success' | 'error';

// Small reusable hook to manage async action/form status
export const useFormStatus = () => {
  const [status, setStatus] = useState<ActionStatus>('idle');
  const [error, setError] = useState<Error | null>(null);

  const run = useCallback(async <T,>(fn: () => Promise<T>): Promise<T> => {
    setStatus('pending');
    setError(null);
    try {
      const res = await fn();
      setStatus('success');
      return res;
    } catch (err) {
      const e = err instanceof Error ? err : new Error(String(err));
      setError(e);
      setStatus('error');
      throw err;
    }
  }, []);

  const reset = useCallback(() => {
    setStatus('idle');
    setError(null);
  }, []);

  return { status, error, run, reset } as const;
};
