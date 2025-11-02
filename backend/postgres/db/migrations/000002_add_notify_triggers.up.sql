-- Add triggers for LISTEN/NOTIFY support on workflow tasks

-- Create function to notify when pending events are inserted
CREATE OR REPLACE FUNCTION notify_pending_event() RETURNS TRIGGER AS $$
BEGIN
  -- Notify on the workflow_tasks channel with the queue name as payload
  -- This SELECT is efficient due to the unique index on (instance_id, execution_id)
  PERFORM pg_notify('workflow_tasks', (
    SELECT queue FROM instances WHERE instance_id = NEW.instance_id AND execution_id = NEW.execution_id LIMIT 1
  )::text);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger on pending_events table
CREATE TRIGGER pending_events_notify
AFTER INSERT ON pending_events
FOR EACH ROW
EXECUTE FUNCTION notify_pending_event();

-- Create function to notify when activities are inserted
CREATE OR REPLACE FUNCTION notify_activity() RETURNS TRIGGER AS $$
BEGIN
  -- Notify on the activity_tasks channel with the queue name as payload
  PERFORM pg_notify('activity_tasks', NEW.queue);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger on activities table
CREATE TRIGGER activities_notify
AFTER INSERT ON activities
FOR EACH ROW
EXECUTE FUNCTION notify_activity();
