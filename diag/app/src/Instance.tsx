import { Accordion, Alert, Card } from "react-bootstrap";
import { Link, useParams } from "react-router-dom";
import {
  EventType,
  Payload,
  ScheduleEventID,
  WorkflowInstance,
  WorkflowInstanceState,
  decodePayload,
  decodePayloads,
} from "./Components";
import { formatAttributesForDisplay } from "./utils";
import {
  ExecutionCompletedAttributes,
  ExecutionStartedAttributes,
  HistoryEvent,
  WorkflowInstanceInfo,
} from "./client";

import useFetch from "react-fetch-hook";
import { InstanceTree } from "./InstanceTree";

function Instance() {
  let params = useParams();

  const instanceId = params.instanceId;
  const executionId = params.executionId;

  const {
    isLoading,
    data: instance,
    error,
  } = useFetch<WorkflowInstanceInfo>(
    document.location.pathname + "api/" + instanceId + "/" + executionId
  );

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error || !instance) {
    return (
      <div>
        <Alert variant="danger">
          Workflow instance with id <code>{instanceId}</code> not found
        </Alert>
      </div>
    );
  }

  const startedEvent = instance.history?.find(
    (e) => e.type === "WorkflowExecutionStarted"
  ) as HistoryEvent<ExecutionStartedAttributes> | undefined;

  const workflowName = startedEvent?.attributes.name;
  const inputs = startedEvent?.attributes.inputs;

  let wfResult: string | undefined;
  let wfError: {} | undefined;
  const finishedEvent = instance.history?.find(
    (e) =>
      e.type === "WorkflowExecutionFinished" ||
      e.type === "WorkflowExecutionContinuedAsNew"
  ) as HistoryEvent<ExecutionCompletedAttributes>; // | ExecutionContinuedAsNewAttributes
  if (finishedEvent) {
    wfResult = finishedEvent.attributes.result;
    wfError = finishedEvent.attributes.error;
  }

  return (
    <div>
      <div className="d-flex align-items-center">
        <h2>
          Workflow: <code>{workflowName || <i>Pending</i>}</code>
        </h2>
      </div>

      <dl className="row">
        <dt className="col-sm-4">InstanceID</dt>
        <dd className="col-sm-8">
          <code>{instance.instance.instance_id}</code>
        </dd>

        <dt className="col-sm-4">ExecutionID</dt>
        <dd className="col-sm-8">
          <code>{instance.instance.execution_id}</code>
        </dd>

        {!!instance.instance.parent && (
          <>
            <dt className="col-sm-4">Parent InstanceID</dt>
            <dd className="col-sm-8">
              {instance.instance.parent && (
                <Link
                  to={`/${instance.instance.parent.instance_id}/${instance.instance.parent.execution_id}`}
                >
                  <WorkflowInstance instance={instance.instance.parent} />
                </Link>
              )}
            </dd>
          </>
        )}

        <dt className="col-sm-4">State</dt>
        <dd className="col-sm-8">
          <WorkflowInstanceState state={instance.state} />
        </dd>

        <dt className="col-sm-4">Created at</dt>
        <dd className="col-sm-8">{instance.created_at}</dd>

        <dt className="col-sm-4">Completed at</dt>
        <dd className="col-sm-8">
          {!instance.completed_at ? <i>pending</i> : instance.completed_at}
        </dd>

        <dt className="col-sm-4">Queue</dt>
        <dd className="col-sm-8">{instance.queue}</dd>
      </dl>

      <Card>
        <Card.Header as="h5">Input</Card.Header>
        <Card.Body>
          {inputs && (
            <Payload payloads={inputs?.map((i) => decodePayload(i))} />
          )}
        </Card.Body>
      </Card>

      <Card className="mt-3">
        <Card.Header as="h5">Result</Card.Header>
        <Card.Body>
          {wfResult && <Payload payloads={[decodePayload(wfResult)]} />}
          {wfError && (
            <Payload
              payloads={[JSON.stringify(decodePayloads(wfError), undefined, 2)]}
            />
          )}
        </Card.Body>
      </Card>

      <h2 className="mt-4">Workflow Graph</h2>
      <InstanceTree
        instanceId={instance.instance.instance_id}
        executionId={instance.instance.execution_id}
      />

      <h2 className="mt-3">History</h2>
      <Accordion alwaysOpen>
        {instance.history?.map((event, idx) => (
          <Accordion.Item
            eventKey={`${idx}`}
            key={event.id}
            className={
              event.schedule_event_id
                ? `schedule-event-${event.schedule_event_id}`
                : ""
            }
          >
            <Accordion.Header>
              <h5 className={"d-flex flex-grow-1 align-items-center pe-3"}>
                <div className="text-secondary" style={{ width: "50px" }}>
                  #{event.sequence_id}
                </div>
                <div className="flex-grow-1">
                  <EventType type={event.type} />

                  {!!event.schedule_event_id && (
                    <ScheduleEventID id={event.schedule_event_id!} />
                  )}
                </div>
                {event.type === "WorkflowExecutionContinuedAsNew" &&
                  event.attributes?.continued_execution_id && (
                    <div className="flex-grow-1 text-secondary small">
                      <Link
                        to={`/${instanceId}/${event.attributes.continued_execution_id}`}
                      >
                        Continued execution
                      </Link>
                    </div>
                  )}
                {event.type !== "WorkflowExecutionStarted" && (
                  <div className="flex-grow-1">
                    <code>{event.attributes?.name}</code>
                  </div>
                )}
                <div>{event.timestamp}</div>
              </h5>
            </Accordion.Header>
            <Accordion.Body>
              <dl>
                <dt>Event ID</dt>
                <dd>{event.id}</dd>
                <dt>Schedule Event ID</dt>
                <dd>
                  {!event.schedule_event_id ? (
                    <i>none</i>
                  ) : (
                    event.schedule_event_id
                  )}
                </dd>
                {event.visible_at && (
                  <>
                    <dt>Visible At</dt>
                    <dd>{event.visible_at}</dd>
                  </>
                )}
                <dt>Attributes</dt>
                <dd>
                  <Payload
                    payloads={[
                      formatAttributesForDisplay(event.attributes)
                    ]}
                  />
                </dd>
              </dl>
            </Accordion.Body>
          </Accordion.Item>
        ))}
      </Accordion>
    </div>
  );
}

export default Instance;
