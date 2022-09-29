CREATE SCHEMA IF NOT EXISTS gwf;

CREATE TABLE IF NOT EXISTS gwf.instances (
  id BIGSERIAL PRIMARY KEY,
  instance_id TEXT NOT NULL,
  execution_id TEXT NOT NULL,
  parent_instance_id TEXT NULL,
  parent_schedule_event_id BIGINT NULL,
  metadata JSONB NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ NULL,
  locked_until TIMESTAMPTZ NULL,
  sticky_until TIMESTAMPTZ NULL,
  worker TEXT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_instances_instance_id ON gwf.instances (instance_id);
CREATE INDEX IF NOT EXISTS idx_instances_locked_until_completed_at ON gwf.instances (completed_at, locked_until, sticky_until, worker);
CREATE INDEX IF NOT EXISTS idx_instances_parent_instance_id ON gwf.instances (parent_instance_id);

CREATE TABLE IF NOT EXISTS gwf.pending_events (
  id BIGSERIAL PRIMARY KEY,
  event_id TEXT NOT NULL,
  sequence_id BIGINT NOT NULL, -- Not used, but keep for now for query compat
  instance_id TEXT NOT NULL,
  event_type INT NOT NULL,
  timestamp TIMESTAMPTZ NOT NULL,
  schedule_event_id BIGINT NOT NULL,
  attributes JSONB NOT NULL,
  visible_at TIMESTAMPTZ NULL
);
CREATE INDEX IF NOT EXISTS idx_pending_events_instance_id ON gwf.pending_events (instance_id);
CREATE INDEX IF NOT EXISTS idx_pending_events_instance_id_visible_at_schedule_event_id ON gwf.pending_events (instance_id, visible_at, schedule_event_id);

CREATE TABLE IF NOT EXISTS gwf.history (
  id BIGSERIAL PRIMARY KEY,
  event_id TEXT NOT NULL,
  sequence_id BIGINT NOT NULL,
  instance_id TEXT NOT NULL,
  event_type INT NOT NULL,
  timestamp TIMESTAMPTZ NOT NULL,
  schedule_event_id BIGINT NOT NULL,
  attributes JSONB NOT NULL,
  visible_at TIMESTAMPTZ NULL -- Is this required?
);
CREATE INDEX IF NOT EXISTS idx_history_instance_id ON gwf.history (instance_id);
CREATE INDEX IF NOT EXISTS idx_history_instance_id_sequence_id ON gwf.history (instance_id, sequence_id);

CREATE TABLE IF NOT EXISTS gwf.activities (
  id BIGSERIAL PRIMARY KEY,
  activity_id TEXT NOT NULL,
  instance_id TEXT NOT NULL,
  execution_id TEXT NOT NULL,
  event_type INT NOT NULL,
  timestamp TIMESTAMPTZ NOT NULL,
  schedule_event_id BIGINT NOT NULL,
  attributes JSONB NOT NULL,
  visible_at TIMESTAMPTZ NULL,
  locked_until TIMESTAMPTZ NULL,
  worker TEXT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_activities_instance_id ON gwf.activities (instance_id, activity_id, execution_id, worker);
CREATE INDEX IF NOT EXISTS idx_activities_locked_until ON gwf.activities (locked_until);