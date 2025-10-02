-- Remove queue column from instances
ALTER TABLE instances DROP COLUMN queue;

-- Update index
DROP INDEX idx_instances_locked_until_completed_at_queue;
CREATE INDEX idx_instances_locked_until_completed_at ON instances (completed_at, locked_until, sticky_until, worker);

-- Update index
DROP INDEX idx_activities_locked_until_queue;

-- Remove queue column from activities
ALTER TABLE activities DROP COLUMN queue;

CREATE INDEX idx_activities_locked_until ON activities (locked_until);
