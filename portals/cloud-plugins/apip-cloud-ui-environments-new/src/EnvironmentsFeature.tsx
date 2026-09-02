import { useState, type FC } from 'react';
import EnvironmentForm from './EnvironmentForm';
import EnvironmentsList from './EnvironmentsList';
import type { AIWorkspaceHostPort } from './hostPort';

export type EnvironmentsFeatureProps = { port: AIWorkspaceHostPort };

const EnvironmentsFeature: FC<EnvironmentsFeatureProps> = ({ port }) => {
  const readOnly = Boolean(port.projectHandle);
  const [view, setView] = useState<'list' | 'create'>('list');

  if (!readOnly && view === 'create') {
    return <EnvironmentForm onBack={() => setView('list')} notify={port.notify} />;
  }

  return <EnvironmentsList readOnly={readOnly} onCreateClick={() => setView('create')} notify={port.notify} />;
};

export default EnvironmentsFeature;
