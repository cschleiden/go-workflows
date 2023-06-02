import { useEffect, useRef, useState } from "react";
import { Alert } from "react-bootstrap";
import Tree from "react-d3-tree";
import { Point } from "react-d3-tree/lib/types/types/common";
import useFetch from "react-fetch-hook";
import { Link } from "react-router-dom";
import { WorkflowInstanceState } from "./Components";
import { WorkflowInstanceTree } from "./client";

function useCenteredTree(
  data: unknown
): [Point, React.RefObject<HTMLDivElement>] {
  const [translate, setTranslate] = useState<Point>({ x: 0, y: 0 });
  const treeContainerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (treeContainerRef.current) {
      const dimensions = treeContainerRef.current.getBoundingClientRect();

      setTranslate({
        x: dimensions.width / 2,
        y: 20, // Approximate the root node
      });
    }
  }, [data]);

  return [translate, treeContainerRef];
}

export const InstanceTree: React.FC<{
  instanceId: string;
  executionId: string;
}> = ({ instanceId, executionId }) => {
  const {
    isLoading,
    data: instanceTree,
    error,
  } = useFetch<WorkflowInstanceTree>(
    document.location.pathname +
      "api/" +
      instanceId +
      "/" +
      executionId +
      "/tree"
  );

  const [translate, containerRef] = useCenteredTree(instanceTree);

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error || !instanceTree) {
    return (
      <div>
        <Alert variant="danger">
          Could not load tree for instance with id <code>{instanceId}</code>
        </Alert>
      </div>
    );
  }

  return (
    <div
      style={{
        width: "100%",
        height: "500px",
      }}
      ref={containerRef}
    >
      <Tree
        orientation="vertical"
        translate={translate}
        collapsible={false}
        zoomable={false}
        data={mapTree(instanceTree)}
        hasInteractiveNodes={true}
        centeringTransitionDuration={1000}
        transitionDuration={1000}
        renderCustomNodeElement={(nodeData) => {
          const x = nodeData.nodeDatum as unknown as DisplayableTree;

          return (
            <g>
              <circle
                r={15}
                fill={x.instance.instance_id === instanceId ? "green" : "black"}
              ></circle>
              <foreignObject width={200} height={50} x={20} y={-25}>
                <div>
                  <Link
                    to={`/${x.instance.instance_id}/${x.instance.execution_id}`}
                  >
                    {x.name}
                  </Link>
                  <br />
                  <WorkflowInstanceState state={x.state} />
                </div>
              </foreignObject>
            </g>
          );
        }}
      />
    </div>
  );
};

type DisplayableTree = WorkflowInstanceTree & {
  name: string;
  children: DisplayableTree[];
};

function mapTree(tree: WorkflowInstanceTree): DisplayableTree {
  return {
    ...tree,
    children: tree.children?.map(mapTree) || [],
    name: tree.workflow_name,
  };
}
