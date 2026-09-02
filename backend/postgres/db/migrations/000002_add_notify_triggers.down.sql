-- Remove LISTEN/NOTIFY triggers

DROP TRIGGER IF EXISTS pending_events_notify ON pending_events;
DROP FUNCTION IF EXISTS notify_pending_event();

DROP TRIGGER IF EXISTS activities_notify ON activities;
DROP FUNCTION IF EXISTS notify_activity();
