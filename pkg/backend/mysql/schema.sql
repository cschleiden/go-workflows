CREATE TABLE IF NOT EXISTS `instances` (
  `id` int(11) NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `instance_id` NVARCHAR(64) NOT NULL,
  `execution_id` NVARCHAR(64) NOT NULL,
  `parent_instance_id` NVARCHAR(64) NULL,
  `parent_event_id` INT NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `completed_at` DATETIME NULL,
  `locked_until` DATETIME NULL,
  `locked_by` NVARCHAR(64) NULL,

  UNIQUE INDEX `idx_instances_instance_id_execution_id` (`instance_id`, `execution_id`),
  INDEX `idx_instances_locked_until_completed_at` (`locked_until`, `completed_at`)
);


CREATE TABLE IF NOT EXISTS `pending_events` (
  `id` int(11) NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `event_id` NVARCHAR(64) NOT NULL,
  `instance_id` NVARCHAR(64) NOT NULL,
  `event_type` INT NOT NULL,
  `event_id2` INT NOT NULL,
  `attributes` BLOB NOT NULL,
  `visible_at` DATETIME NULL,

  INDEX `idx_pending_events_instance_id_execution_id_visible_at` (`instance_id`, `visible_at`)
);


CREATE TABLE IF NOT EXISTS `history` (
  `id` int(11) NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `event_id` NVARCHAR(64) NOT NULL,
  `instance_id` NVARCHAR(64) NOT NULL,
  `event_type` INT NOT NULL,
  `event_id2` INT NOT NULL,
  `attributes` BLOB NOT NULL,
  `visible_at` DATETIME NULL, -- Is this required?

  INDEX `idx_history_instance_id` (`instance_id`)
);


CREATE TABLE IF NOT EXISTS `activities` (
  `id` int(11) NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `activity_id` NVARCHAR(64) NOT NULL,
  `instance_id` NVARCHAR(64) NOT NULL,
  `execution_id` NVARCHAR(64) NOT NULL,
  `event_type` INT NOT NULL,
  `event_id` INT NOT NULL,
  `attributes` BLOB NOT NULL,
  `visible_at` DATETIME NULL,
  `locked_until` DATETIME NULL,
  `locked_by` NVARCHAR(64) NULL,

  UNIQUE INDEX `idx_activities_instance_id` (`activity_id`, `instance_id`, `execution_id`),
  INDEX `idx_activities_locked_until` (`locked_until`)
);