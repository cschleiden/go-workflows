CREATE TABLE schema_migrations (version uint64,dirty bool);
CREATE UNIQUE INDEX version_unique ON schema_migrations (version);
CREATE TABLE `instances` (
  `id` TEXT NOT NULL,
  `execution_id` TEXT NOT NULL,
  `parent_instance_id` TEXT NULL,
  `parent_execution_id` TEXT NULL,
  `parent_schedule_event_id` INTEGER NULL,
  `metadata` TEXT NULL,
  `state` INTEGER NOT NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `completed_at` DATETIME NULL,
  `locked_until` DATETIME NULL,
  `sticky_until` DATETIME NULL,
  `worker` TEXT NULL, `queue` NVARCHAR(128) DEFAULT '',
  PRIMARY KEY(`id`, `execution_id`)
);
CREATE INDEX `idx_instances_id_execution_id` ON `instances` (`id`, `execution_id`);
CREATE INDEX `idx_instances_parent_instance_id_parent_execution_id` ON `instances` (`parent_instance_id`, `parent_execution_id`);
CREATE TABLE `pending_events` (
  `id` TEXT,
  `sequence_id` INTEGER NOT NULL, -- not used but keep for now for query compat
  `instance_id` TEXT NOT NULL,
  `execution_id` TEXT NOT NULL,
  `event_type` INTEGER NOT NULL,
  `timestamp` DATETIME NOT NULL,
  `schedule_event_id` INT NOT NULL,
  `visible_at` DATETIME NULL,
  PRIMARY KEY(`id`, `instance_id`)
);
CREATE INDEX `idx_pending_events_instance_id_execution_id_visible_at_schedule_event_id` ON `pending_events` (`instance_id`, `execution_id`, `visible_at`, `schedule_event_id`);
CREATE TABLE `history` (
  `id` TEXT,
  `sequence_id` INTEGER NOT NULL,
  `instance_id` TEXT NOT NULL,
  `execution_id` TEXT NOT NULL,
  `event_type` INTEGER NOT NULL,
  `timestamp` DATETIME NOT NULL,
  `schedule_event_id` INT NOT NULL,
  `visible_at` DATETIME NULL,
  PRIMARY KEY(`id`, `instance_id`)
);
CREATE INDEX `idx_history_instance_sequence_id` ON `history` (`instance_id`, `execution_id`, `sequence_id`);
CREATE TABLE `activities` (
  `id` TEXT PRIMARY KEY,
  `instance_id` TEXT NOT NULL,
  `execution_id` TEXT NOT NULL,
  `event_type` INTEGER NOT NULL,
  `timestamp` DATETIME NOT NULL,
  `schedule_event_id` INT NOT NULL,
  `visible_at` DATETIME NULL,
  `locked_until` DATETIME NULL,
  `worker` TEXT NULL
, `queue` NVARCHAR(128) DEFAULT '');
CREATE INDEX `idx_activities_id_worker` ON `activities` (`id`, `worker`);
CREATE INDEX `idx_activities_locked_until` ON `activities` (`locked_until`);
CREATE TABLE `attributes` (
  `id` TEXT NOT NULL,
  `instance_id` TEXT NOT NULL,
  `execution_id` TEXT NOT NULL,
  `data` BLOB NOT NULL,
  PRIMARY KEY(`id`, `instance_id`, `execution_id`)
);
CREATE INDEX `idx_instances_locked_until_completed_at_queue` ON `instances` (`completed_at`, `locked_until`, `sticky_until`, `worker`, `queue`);
CREATE INDEX `idx_activities_instance_id_execution_id_worker_queue` ON `activities` (`instance_id`, `execution_id`, `worker`, `queue`);
