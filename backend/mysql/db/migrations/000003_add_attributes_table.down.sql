ALTER TABLE `activities` ADD COLUMN `attributes` MEDIUMBLOB NULL;
UPDATE `activities` SET `attributes` = `attributes`.`data` FROM `attributes` WHERE `activities`.`event_id` = `attributes`.`event_id` AND `activities`.`instance_id` = `attributes`.`instance_id` AND `activities`.`execution_id` = `attributes`.`execution_id`;
ALTER TABLE `activities` MODIFY COLUMN `attributes` MEDIUMBLOB NOT NULL;

ALTER TABLE `history` ADD COLUMN `attributes` MEDIUMBLOB NULL;
UPDATE `history` SET `attributes` = `attributes`.`data` FROM `attributes` WHERE `history`.`event_id` = `attributes`.`event_id` AND `history`.`instance_id` = `attributes`.`instance_id` AND `history`.`execution_id` = `attributes`.`execution_id`;
ALTER TABLE `history` MODIFY COLUMN `attributes` MEDIUMBLOB NOT NULL;

ALTER TABLE `pending_events` ADD COLUMN `attributes` MEDIUMBLOB NULL;
UPDATE `pending_events` SET `attributes` = `attributes`.`data` FROM `attributes` WHERE `pending_events`.`event_id` = `attributes`.`event_id` AND `pending_events`.`instance_id` = `attributes`.`instance_id` AND `pending_events`.`execution_id` = `attributes`.`execution_id`;
ALTER TABLE `pending_events` MODIFY COLUMN `attributes` MEDIUMBLOB NOT NULL;


-- Drop attributes table
DROP TABLE `attributes`;