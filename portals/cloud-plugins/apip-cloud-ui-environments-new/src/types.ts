export type Environment = {
  id: string;
  name: string;
  critical: boolean;
  createdAt: string;
};

export type CreateEnvironmentInput = Pick<Environment, 'name' | 'critical'>;
