ALTER TABLE `instances` ADD COLUMN `queue` NVARCHAR(128) DEFAULT '';

-- Update index
DROP INDEX `idx_instances_locked_until_completed_at` ON `instances`;
CREATE INDEX `idx_instances_locked_until_completed_at_queue` ON `instances` (`completed_at`, `locked_until`, `sticky_until`, `worker`, `queue`);

ALTER TABLE `activities` ADD COLUMN `queue` NVARCHAR(128) DEFAULT '';

-- Update index
DROP INDEX `idx_activities_locked_until` ON `activities`;
CREATE INDEX `idx_activities_locked_until_queue` ON `activities` (`locked_until`, `queue`);