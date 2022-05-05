import React from "react";
import { Badge } from "react-bootstrap";
import { Color } from "react-bootstrap/esm/types";

export function decodePayload(payload: string): string {
  try {
    return atob(payload);
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

      case "result":
        r[key] = decodePayload(payload[key]);
        break;

      default:
        r[key] = payload[key];
    }
  }

  return r;
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

    default:
      return ["dark", "info"];
  }
}
