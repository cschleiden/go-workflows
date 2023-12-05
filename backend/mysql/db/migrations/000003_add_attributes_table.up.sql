CREATE TABLE IF NOT EXISTS `attributes` (
  `id` BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  `event_id` NVARCHAR(128) NOT NULL,
  `instance_id` NVARCHAR(128) NOT NULL,
  `execution_id` NVARCHAR(128) NOT NULL,
  `data` MEDIUMBLOB NOT NULL,

  UNIQUE INDEX `idx_attributes_instance_id_execution_id_event_id` (`instance_id`, `execution_id`, `event_id`),
  INDEX `idx_attributes_event_id` (`event_id`)
);

-- Move activity attributes to attributes table
INSERT IGNORE INTO `attributes` (`event_id`, `instance_id`, `execution_id`, `data`) SELECT `activity_id`, `instance_id`, `execution_id`, `attributes` FROM `activities`;
ALTER TABLE `activities` DROP COLUMN `attributes`;

-- Move history attributes to attributes table
INSERT IGNORE INTO `attributes` (`event_id`, `instance_id`, `execution_id`, `data`) SELECT `event_id`, `instance_id`, `execution_id`, `attributes` FROM `history`;
ALTER TABLE `history` DROP COLUMN `attributes`;

-- Move pending_events attributes to attributes table
INSERT IGNORE INTO `attributes` (`event_id`, `instance_id`, `execution_id`, `data`) SELECT `event_id`, `instance_id`, `execution_id`, `attributes` FROM `pending_events`;
ALTER TABLE `pending_events` DROP COLUMN `attributes`;