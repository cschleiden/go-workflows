-- Drop column from instances and activities
ALTER TABLE `instances` DROP COLUMN `queue`;
ALTER TABLE `activities` DROP COLUMN `queue`;

-- Update index
DROP INDEX IF EXISTS `idx_instances_locked_until_completed_at_queue`;
CREATE INDEX `idx_instances_locked_until_completed_at` ON `instances` (`completed_at`, `locked_until`, `sticky_until`, `worker`);

DROP INDEX IF EXISTS `idx_activities_instance_id_execution_id_activity_id_worker_queue`;
CREATE INDEX `idx_activities_instance_id_execution_id_worker` ON `activities` (`instance_id`, `execution_id`, `id`, `worker`);