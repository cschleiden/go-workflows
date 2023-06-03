CREATE TABLE IF NOT EXISTS `instances` (
  `id` BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `instance_id` NVARCHAR(128) NOT NULL,
  `execution_id` NVARCHAR(128) NOT NULL,
  `parent_instance_id` NVARCHAR(128) NULL,
  `parent_execution_id` NVARCHAR(128) NULL,
  `parent_schedule_event_id` BIGINT NULL,
  `metadata` BLOB NULL,
  `state` INT NOT NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `completed_at` DATETIME NULL,
  `locked_until` DATETIME NULL,
  `sticky_until` DATETIME NULL,
  `worker` NVARCHAR(64) NULL,

  UNIQUE INDEX `idx_instances_instance_id_execution_id` (`instance_id`, `execution_id`),
  INDEX `idx_instances_locked_until_completed_at` (`completed_at`, `locked_until`, `sticky_until`, `worker`),
  INDEX `idx_instances_parent_instance_id_parent_execution_id` (`parent_instance_id`, `parent_execution_id`)
);


CREATE TABLE IF NOT EXISTS `pending_events` (
  `id` BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `event_id` NVARCHAR(128) NOT NULL,
  `sequence_id` BIGINT NOT NULL, -- Not used, but keep for now for query compat
  `instance_id` NVARCHAR(128) NOT NULL,
  `execution_id` NVARCHAR(128) NOT NULL,
  `event_type` INT NOT NULL,
  `timestamp` DATETIME NOT NULL,
  `schedule_event_id` BIGINT NOT NULL,
  `attributes` BLOB NOT NULL,
  `visible_at` DATETIME NULL,

  INDEX `idx_pending_events_inid_exid` (`instance_id`, `execution_id`),
  INDEX `idx_pending_events_inid_exid_visible_at_schedule_event_id` (`instance_id`, `execution_id`, `visible_at`, `schedule_event_id`)
);


CREATE TABLE IF NOT EXISTS `history` (
  `id` BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `event_id` NVARCHAR(64) NOT NULL,
  `sequence_id` BIGINT NOT NULL,
  `instance_id` NVARCHAR(128) NOT NULL,
  `execution_id` NVARCHAR(128) NOT NULL,
  `event_type` INT NOT NULL,
  `timestamp` DATETIME NOT NULL,
  `schedule_event_id` BIGINT NOT NULL,
  `attributes` BLOB NOT NULL,
  `visible_at` DATETIME NULL, -- Is this required?

  INDEX `idx_history_instance_id_execution_id` (`instance_id`, `execution_id`),
  INDEX `idx_history_instance_id_execution_id_sequence_id` (`instance_id`, `execution_id`, `sequence_id`)
);


CREATE TABLE IF NOT EXISTS `activities` (
  `id` BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `activity_id` NVARCHAR(64) NOT NULL,
  `instance_id` NVARCHAR(128) NOT NULL,
  `execution_id` NVARCHAR(128) NOT NULL,
  `event_type` INT NOT NULL,
  `timestamp` DATETIME NOT NULL,
  `schedule_event_id` BIGINT NOT NULL,
  `attributes` BLOB NOT NULL,
  `visible_at` DATETIME NULL,
  `locked_until` DATETIME NULL,
  `worker` NVARCHAR(64) NULL,

  UNIQUE INDEX `idx_activities_instance_id_execution_id_activity_id_worker` (`instance_id`, `execution_id`, `activity_id`, `worker`),
  INDEX `idx_activities_locked_until` (`locked_until`)
);