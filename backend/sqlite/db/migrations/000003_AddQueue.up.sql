-- Add queue to instances
ALTER TABLE `instances` ADD COLUMN `queue` NVARCHAR(128) DEFAULT '';

-- Update index
DROP INDEX IF EXISTS `idx_instances_locked_until_completed_at` ;
CREATE INDEX `idx_instances_locked_until_completed_at_queue` ON `instances` (`completed_at`, `locked_until`, `sticky_until`, `worker`, `queue`);

-- Add queue to activities
ALTER TABLE `activities` ADD COLUMN `queue` NVARCHAR(128) DEFAULT '';

-- Update index
DROP INDEX IF EXISTS `idx_activities_instance_id_execution_id_worker`;
CREATE INDEX `idx_activities_instance_id_execution_id_worker_queue` ON `activities` (`instance_id`, `execution_id`, `worker`, `queue`);

DROP INDEX IF EXISTS `idx_activities_locked_until`;

CREATE INDEX `idx_activities_locked_until_queue` ON `activities` (`locked_until`, `queue`);
