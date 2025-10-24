export type StepKey = "gateway" | "apis" | "test";
export type GwType = "hybrid" | "cloud";
export type GwMode = "cards" | "form" | "list";

export interface GatewayRecord {
  id: string;
  type: GwType;
  displayName: string;
  name: string;
  host?: string;
  description?: string;
  createdAt: Date;
  isActive?: boolean; // becomes true after copying the curl command
}
