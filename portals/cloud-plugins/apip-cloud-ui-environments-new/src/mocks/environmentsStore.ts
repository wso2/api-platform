import type { CreateEnvironmentInput, Environment } from '../types';

let environments: Environment[] = [
  { id: 'development', name: 'Development', critical: false, createdAt: '2023-09-01T00:00:00.000Z' },
  { id: 'production', name: 'Production', critical: true, createdAt: '2023-09-01T00:00:00.000Z' },
];

export function listEnvironments(): Environment[] {
  return environments;
}

export function createEnvironment(input: CreateEnvironmentInput): Environment {
  const environment: Environment = {
    id: `environment-${Date.now()}`,
    ...input,
    createdAt: new Date().toISOString(),
  };
  environments = [...environments, environment];
  return environment;
}

export function deleteEnvironment(id: string): void {
  environments = environments.filter((environment) => environment.id !== id);
}
