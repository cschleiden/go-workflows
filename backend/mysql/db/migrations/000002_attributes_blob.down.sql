ALTER TABLE `activities` MODIFY COLUMN `attributes` BLOB NOT NULL;
ALTER TABLE `history` MODIFY COLUMN `attributes` BLOB NOT NULL;
ALTER TABLE `pending_events` MODIFY COLUMN `attributes` BLOB NOT NULL;