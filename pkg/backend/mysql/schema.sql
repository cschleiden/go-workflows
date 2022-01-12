CREATE TABLE IF NOT EXISTS `instances` (
  `id` NVARCHAR(32) NOT NULL,
  `execution_id` NVARCHAR(32) NOT NULL,
  `parent_instance_id` NVARCHAR(32) NOT NULL,
  `parent_event_id` INT NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `completed_at` DATETIME NULL,
  `locked_until` DATETIME NULL,
  `locked_by` NVARCHAR(32) NULL,

  PRIMARY KEY(`id`, `execution_id`),
  INDEX `idx_instances_execution_id` (`execution_id`)
);


CREATE TABLE IF NOT EXISTS `pending_events` (
  `id` NVARCHAR(32) NOT NULL PRIMARY KEY,
  `instance_id` NVARCHAR(32) NOT NULL,
  `event_type` INT NOT NULL,
  `event_id` INT NOT NULL,
  `attributes` BLOB NOT NULL,
  `visible_at` DATETIME NULL,

  INDEX `idx_pending_events_instance_id_visible_at` (`instance_id`, `visible_at`)
);


CREATE TABLE IF NOT EXISTS `history` (
  `id` NVARCHAR(32) NOT NULL PRIMARY KEY,
  `instance_id` NVARCHAR(32) NOT NULL,
  `event_type` INT NOT NULL,
  `event_id` INT NOT NULL,
  `attributes` BLOB NOT NULL,
  `visible_at` DATETIME NULL, -- Is this required?

  INDEX `idx_history_instance_id` (`instance_id`)
);


CREATE TABLE IF NOT EXISTS `activities` (
  `id` NVARCHAR(32) NOT NULL PRIMARY KEY,
  `instance_id` NVARCHAR(32) NOT NULL,
  `execution_id` NVARCHAR(32) NOT NULL,
  `event_type` INT NOT NULL,
  `event_id` INT NOT NULL,
  `attributes` BLOB NOT NULL,
  `visible_at` DATETIME NULL,
  `locked_until` DATETIME NULL,
  `locked_by` NVARCHAR(32) NULL,
  INDEX `idx_activities_instance_id` (`instance_id`)
);