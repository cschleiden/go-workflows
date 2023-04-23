export interface WorkflowInstance {
  instance_id: string;
  parent_instance: string;
}

export interface WorkflowInstanceRef {
  instance: WorkflowInstance;

  created_at: string;
  completed_at?: string;

  state: number;
}

export type WorkflowInstanceInfo = WorkflowInstanceRef & {
  history: HistoryEvent<any>[];
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

export type WorkflowInstanceTree = WorkflowInstanceRef & {
  workflow_name: string;
  children: WorkflowInstanceTree[];
};
