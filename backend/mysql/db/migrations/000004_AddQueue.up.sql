ALTER TABLE `instances` ADD COLUMN `queue` NVARCHAR(128) DEFAULT '';

-- Update index
DROP INDEX `idx_instances_locked_until_completed_at` ON `instances`;
CREATE INDEX `idx_instances_locked_until_completed_at_queue` ON `instances` (`completed_at`, `locked_until`, `sticky_until`, `worker`, `queue`);

ALTER TABLE `activities` ADD COLUMN `queue` NVARCHAR(128) DEFAULT '';

-- Update index
DROP INDEX `idx_activities_instance_id_execution_id_activity_id_worker` ON `activities`;
CREATE INDEX `idx_activities_instance_id_execution_id_activity_id_worker_queue` ON `activities` (`instance_id`, `execution_id`, `activity_id`, `worker`, `queue`);