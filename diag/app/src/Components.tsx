import React from "react";
import { Badge } from "react-bootstrap";
import { Color } from "react-bootstrap/esm/types";
import { WorkflowInstance as Instance } from "./client";

export function decodePayload(payload: string): string {
  try {
    const decoded = atob(payload);
    
    // Try to parse as JSON and pretty-print if valid
    try {
      JSON.parse(decoded);
      // If parsing succeeds, pretty-print the JSON
      return JSON.stringify(JSON.parse(decoded), null, 2);
    } catch {
      // If not valid JSON, return as-is
      return decoded;
    }
  } catch {
    return payload;
  }
}

export function decodePayloads(payload: { [key: string]: any }): any {
  const r: any = {};

  for (const key of Object.keys(payload)) {
    switch (key) {
      case "inputs":
        r[key] = payload[key].map((p: any) => decodePayload(p));
        break;

      case "error":
        r[key] = decodePayloads(payload[key]);
        break;

      case "result":
        r[key] = decodePayload(payload[key]);
        break;

      case "stacktrace":
        r[key] = ("" + payload[key]).replaceAll("\t", "    ").split("\n");
        break;

      default:
        r[key] = payload[key];
    }
  }

  return r;
}

export function formatAttributesForDisplay(attributes: { [key: string]: any }): string {
  const decoded = decodePayloads(attributes);
  
  // Custom replacer function to handle already-pretty-printed JSON strings in inputs
  const replacer = (key: string, value: any) => {
    if (key === "inputs" && Array.isArray(value)) {
      // For inputs, we want to parse the pretty-printed JSON strings back to objects
      // so they display nicely in the final JSON
      return value.map(input => {
        try {
          return JSON.parse(input);
        } catch {
          return input;
        }
      });
    }
    return value;
  };
  
  return JSON.stringify(decoded, replacer, 2);
}

export const Payload: React.FC<{ payloads: string[] }> = ({ payloads }) => {
  return (
    <div className="bg-dark text-light rounded p-2">
      {payloads.map((p, idx) => (
        <pre className="mb-0" key={idx}>
          {p}
        </pre>
      ))}
    </div>
  );
};

export const WorkflowInstance: React.FC<{ instance: Instance }> = ({
  instance,
}) => {
  return (
    <div>
      <code>{instance.instance_id}</code>
      <br />
      <small>{instance.execution_id}</small>
    </div>
  );
};

export const WorkflowInstanceState: React.FC<{ state: number }> = ({
  state,
}) => {
  if (state === 0) {
    return <Badge bg="info">Active</Badge>;
  } else if (state === 1) {
    return (
      <Badge bg="light" text="dark">
        ContinuedAsNew
      </Badge>
    );
  } else {
    return <Badge bg="success">Completed</Badge>;
  }
};

export const EventType: React.FC<{ type: string }> = ({ type }) => {
  const [textColor, bgColor] = eventColor(type);

  return (
    <Badge text={textColor} bg={bgColor}>
      <code
        style={{
          color: "inherit",
        }}
      >
        {type}
      </code>
    </Badge>
  );
};

export const ScheduleEventID: React.FC<{ id: number }> = ({ id }) => {
  const [textColor, bgColor] = scheduleEventIDColor(id);

  return (
    <Badge
      className="ms-2"
      pill
      text={textColor}
      bg={""}
      style={{
        background: bgColor,
        fontWeight: "bold",
      }}
    >
      <code
        style={{
          color: "inherit",
        }}
      >
        {id}
      </code>
    </Badge>
  );
};

function eventColor(event: string): [Color, string] {
  switch (event) {
    case "SubWorkflowScheduled":
    case "SubWorkflowCancellationRequested":
    case "SubWorkflowCompleted":
    case "SubWorkflowFailed":
      return ["light", "success"];

    case "ActivityScheduled":
    case "ActivityCompleted":
    case "ActivityFailed":
      return ["dark", "warning"];

    case "TimerScheduled":
    case "TimerFired":
    case "TimerCanceled":
      return ["light", "primary"];

    case "SignalReceived":
      return ["light", "dark"];

    case "SideEffectResult":
      return ["dark", "secondary"];

    case "WorkflowTaskStarted":
      return ["dark", "light"];

    case "WorkflowExecutionStarted":
      return ["dark", "info"];
    case "WorkflowExecutionFinished":
      return ["light", "success"];
    case "WorkflowExecutionContinuedAsNew":
      return ["dark", "light"];

    default:
      return ["dark", "light"];
  }
}

function scheduleEventIDColor(id: number): [Color, string] {
  // Default bootstrap theme colors
  const colors: [Color, string][] = [
    ["light", "#0d6efd"],
    ["light", "#6f42c1"],
    ["light", "#d63384"],
    ["light", "#ffc107"],
    ["light", "#dc3545"],
    ["light", "#6610f2"],
    ["light", "#fd7e14"],
    ["light", "#198754"],
    ["light", "#20c997"],
    ["light", "#0dcaf0"],
  ];

  return colors[(id - 1) % colors.length];
}
