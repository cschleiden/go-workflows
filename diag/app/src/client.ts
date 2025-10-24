export interface WorkflowInstance {
  instance_id: string;
  parent?: WorkflowInstance;
  execution_id: string;
}

export interface WorkflowInstanceRef {
  instance: WorkflowInstance;

  created_at: string;
  completed_at?: string;

  state: number;
  queue: string;
}

export type WorkflowInstanceInfo = WorkflowInstanceRef & {
  history?: HistoryEvent<any>[];
};

export interface HistoryEvent<TAttributes> {
  id: string;
  sequence_id: number;
  type: string;
  timestamp: string;
  schedule_event_id?: number;
  attributes: TAttributes;
  visible_at?: string;
}

export interface ExecutionStartedAttributes {
  name: string;
  inputs: string[];
}

export interface ExecutionCompletedAttributes {
  result: string;
  error: string;
}

export interface ExecutionContinuedAsNewAttributes {
  result: string;
}

export type WorkflowInstanceTree = WorkflowInstanceRef & {
  workflow_name: string;
  error?: boolean;
  children: WorkflowInstanceTree[];
};

export interface Stats {
  ActiveWorkflowInstances: number;
  PendingActivityTasks: Record<string, number>;
  PendingWorkflowTasks: Record<string, number>;
}
