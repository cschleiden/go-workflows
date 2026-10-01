import { useEffect, useMemo, useRef, useState } from "react";
import Tree from "react-d3-tree";
import { Point } from "react-d3-tree/lib/types/types/common";
import { HistoryEvent } from "./client";
import {
  buildExecutionGraph,
  EndState,
  ExecutionGraphNode,
  ExecutionNode,
  NodeState,
} from "./buildExecutionGraph";

const ITEM_HEIGHT = 40;
const NODE_WIDTH = 200;

const stateColors: Record<NodeState | EndState, string> = {
  Pending: "#6c757d",
  Completed: "#198754",
  Failed: "#dc3545",
  Canceled: "#ffc107",
  ContinuedAsNew: "#0dcaf0",
  Terminated: "#dc3545",
};

function highlightEvents(node: ExecutionNode) {
  const elements: HTMLElement[] = node.scheduleEventId
    ? Array.from(
        document.querySelectorAll<HTMLElement>(
          `.schedule-event-${node.scheduleEventId}`
        )
      )
    : [document.getElementById(`event-${node.eventId}`)].filter(
        (e): e is HTMLElement => !!e
      );

  if (elements.length === 0) {
    return;
  }

  elements[0].scrollIntoView({ behavior: "smooth", block: "center" });
  elements.forEach((e) => {
    e.style.outline = "3px solid #0d6efd";
    e.style.outlineOffset = "-3px";
  });
  setTimeout(() => {
    elements.forEach((e) => {
      e.style.outline = "";
      e.style.outlineOffset = "";
    });
  }, 2000);
}

const Pill: React.FC<{ node: ExecutionNode }> = ({ node }) => (
  <div
    onClick={() => highlightEvents(node)}
    title={`${node.kind}: ${node.name}`}
    style={{
      cursor: "pointer",
      background: stateColors[node.state],
      color: node.state === "Canceled" ? "#212529" : "#fff",
      borderRadius: "16px",
      padding: "2px 10px",
      height: ITEM_HEIGHT - 6,
      marginBottom: 6,
      overflow: "hidden",
      whiteSpace: "nowrap",
      textOverflow: "ellipsis",
      fontSize: "12px",
      lineHeight: "15px",
    }}
  >
    <b>{node.name || <i>unnamed</i>}</b>
    <br />
    <small>
      {node.kind} · {node.state}
    </small>
  </div>
);

export const ExecutionGraph: React.FC<{ history: HistoryEvent<any>[] }> = ({
  history,
}) => {
  const data = useMemo(() => buildExecutionGraph(history), [history]);
  const containerRef = useRef<HTMLDivElement>(null);
  const [translate, setTranslate] = useState<Point>({ x: 0, y: 0 });

  useEffect(() => {
    if (containerRef.current) {
      const dimensions = containerRef.current.getBoundingClientRect();
      setTranslate({ x: 60, y: dimensions.height / 2 });
    }
  }, [data]);

  return (
    <div style={{ width: "100%", height: "400px" }} ref={containerRef}>
      <Tree
        orientation="horizontal"
        translate={translate}
        collapsible={false}
        zoomable={true}
        data={data}
        nodeSize={{ x: NODE_WIDTH + 60, y: 100 }}
        renderCustomNodeElement={(nodeData) => {
          const n = nodeData.nodeDatum as unknown as ExecutionGraphNode;

          if (n.nodeType === "step") {
            const items = n.items || [];
            const height = items.length * ITEM_HEIGHT + 20;
            return (
              <g>
                <foreignObject
                  width={NODE_WIDTH}
                  height={height}
                  x={-NODE_WIDTH / 2}
                  y={-height / 2}
                >
                  <div
                    style={{
                      border: "1px solid #dee2e6",
                      borderRadius: "8px",
                      background: "#fff",
                      padding: "4px 6px 0 6px",
                      height: "100%",
                    }}
                  >
                    <div className="text-secondary" style={{ fontSize: "10px" }}>
                      {n.name}
                    </div>
                    {items.map((item) => (
                      <Pill key={item.eventId} node={item} />
                    ))}
                  </div>
                </foreignObject>
              </g>
            );
          }

          const color =
            n.nodeType === "end" ? stateColors[n.endState || "Pending"] : "#0dcaf0";
          const label =
            n.nodeType === "end" ? `End (${n.endState})` : n.name || "Start";
          return (
            <g>
              <circle r={15} fill={color} stroke="#495057" strokeWidth={1} />
              <foreignObject width={160} height={40} x={-80} y={20}>
                <div style={{ textAlign: "center", fontSize: "12px" }}>
                  <code>{label}</code>
                </div>
              </foreignObject>
            </g>
          );
        }}
      />
    </div>
  );
};
