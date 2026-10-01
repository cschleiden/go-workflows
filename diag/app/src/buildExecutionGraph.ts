import { RawNodeDatum } from "react-d3-tree/lib/types/types/common";
import { HistoryEvent } from "./client";

export type NodeKind = "Activity" | "SubWorkflow" | "Timer" | "Signal" | "SideEffect";
export type NodeState = "Pending" | "Completed" | "Failed" | "Canceled";
export type EndState =
  | "Pending"
  | "Completed"
  | "Failed"
  | "ContinuedAsNew"
  | "Canceled"
  | "Terminated";

export interface ExecutionNode {
  kind: NodeKind;
  name: string;
  state: NodeState;
  eventId: string;
  scheduleEventId?: number;
}

export type ExecutionGraphNode = RawNodeDatum & {
  nodeType: "start" | "step" | "end";
  items?: ExecutionNode[];
  endState?: EndState;
  children?: ExecutionGraphNode[];
};

const creatingEvents: Record<string, NodeKind> = {
  ActivityScheduled: "Activity",
  SubWorkflowScheduled: "SubWorkflow",
  TimerScheduled: "Timer",
  SignalReceived: "Signal",
  SideEffectResult: "SideEffect",
};

const outcomeEvents: Record<string, NodeState> = {
  ActivityCompleted: "Completed",
  ActivityFailed: "Failed",
  SubWorkflowCompleted: "Completed",
  SubWorkflowFailed: "Failed",
  SubWorkflowCancellationRequested: "Canceled",
  TimerFired: "Completed",
  TimerCanceled: "Canceled",
};

// Builds a linear chain Start -> Step 1 -> ... -> End from the history.
// A batch is the set of events between two consecutive WorkflowTaskStarted
// events: nodes created in the same batch ran in parallel, batches are
// sequential. Outcome events are matched to nodes via schedule_event_id.
export function buildExecutionGraph(
  history: HistoryEvent<any>[]
): ExecutionGraphNode {
  const batches: ExecutionNode[][] = [];
  const bySchedule = new Map<number, ExecutionNode>();
  let current: ExecutionNode[] = [];
  let workflowName = "";
  let endState: EndState = "Pending";

  for (const event of history) {
    if (event.type === "WorkflowTaskStarted") {
      if (current.length > 0) {
        batches.push(current);
      }
      current = [];
      continue;
    }

    const kind = creatingEvents[event.type];
    if (kind) {
      const node: ExecutionNode = {
        kind,
        name: event.attributes?.name || "",
        state: kind === "Signal" || kind === "SideEffect" ? "Completed" : "Pending",
        eventId: event.id,
        scheduleEventId: event.schedule_event_id,
      };
      current.push(node);
      if (event.schedule_event_id) {
        bySchedule.set(event.schedule_event_id, node);
      }
      continue;
    }

    const outcome = outcomeEvents[event.type];
    if (outcome && event.schedule_event_id) {
      const node = bySchedule.get(event.schedule_event_id);
      if (node) {
        node.state = outcome;
      }
      continue;
    }

    switch (event.type) {
      case "WorkflowExecutionStarted":
        workflowName = event.attributes?.name || "";
        break;
      case "WorkflowExecutionFinished":
        endState = event.attributes?.error ? "Failed" : "Completed";
        break;
      case "WorkflowExecutionContinuedAsNew":
        endState = "ContinuedAsNew";
        break;
      case "WorkflowExecutionCanceled":
        endState = "Canceled";
        break;
      case "WorkflowExecutionTerminated":
        endState = "Terminated";
        break;
    }
  }
  if (current.length > 0) {
    batches.push(current);
  }

  let tail: ExecutionGraphNode = { name: "End", nodeType: "end", endState };
  for (let i = batches.length - 1; i >= 0; i--) {
    tail = {
      name: `Step ${i + 1}`,
      nodeType: "step",
      items: batches[i],
      children: [tail],
    };
  }

  return { name: workflowName, nodeType: "start", children: [tail] };
}
