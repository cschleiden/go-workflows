-- MySQL dump 10.13  Distrib 8.0.33, for Linux (x86_64)
--
-- Host: localhost    Database: workflows_1716351619
-- ------------------------------------------------------
-- Server version	8.0.33












--
-- Table structure for table `activities`
--

DROP TABLE IF EXISTS `activities`;


CREATE TABLE `activities` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `activity_id` varchar(64) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL,
  `instance_id` varchar(128) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL,
  `execution_id` varchar(128) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL,
  `event_type` int NOT NULL,
  `timestamp` datetime NOT NULL,
  `schedule_event_id` bigint NOT NULL,
  `visible_at` datetime DEFAULT NULL,
  `locked_until` datetime DEFAULT NULL,
  `worker` varchar(64) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci DEFAULT NULL,
  `queue` varchar(128) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci DEFAULT '',
  PRIMARY KEY (`id`),
  KEY `idx_activities_locked_until` (`locked_until`),
  KEY `idx_activities_instance_id_execution_id_activity_id_worker_queue` (`instance_id`,`execution_id`,`activity_id`,`worker`,`queue`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;


--
-- Dumping data for table `activities`
--

LOCK TABLES `activities` WRITE;


UNLOCK TABLES;

--
-- Table structure for table `attributes`
--

DROP TABLE IF EXISTS `attributes`;


CREATE TABLE `attributes` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `event_id` varchar(128) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL,
  `instance_id` varchar(128) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL,
  `execution_id` varchar(128) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL,
  `data` mediumblob NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_attributes_instance_id_execution_id_event_id` (`instance_id`,`execution_id`,`event_id`),
  KEY `idx_attributes_event_id` (`event_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;


--
-- Dumping data for table `attributes`
--

LOCK TABLES `attributes` WRITE;


UNLOCK TABLES;

--
-- Table structure for table `history`
--

DROP TABLE IF EXISTS `history`;


CREATE TABLE `history` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `event_id` varchar(64) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL,
  `sequence_id` bigint NOT NULL,
  `instance_id` varchar(128) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL,
  `execution_id` varchar(128) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL,
  `event_type` int NOT NULL,
  `timestamp` datetime NOT NULL,
  `schedule_event_id` bigint NOT NULL,
  `visible_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_history_instance_id_execution_id` (`instance_id`,`execution_id`),
  KEY `idx_history_instance_id_execution_id_sequence_id` (`instance_id`,`execution_id`,`sequence_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;


--
-- Dumping data for table `history`
--

LOCK TABLES `history` WRITE;


UNLOCK TABLES;

--
-- Table structure for table `instances`
--

DROP TABLE IF EXISTS `instances`;


CREATE TABLE `instances` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `instance_id` varchar(128) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL,
  `execution_id` varchar(128) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL,
  `parent_instance_id` varchar(128) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci DEFAULT NULL,
  `parent_execution_id` varchar(128) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci DEFAULT NULL,
  `parent_schedule_event_id` bigint DEFAULT NULL,
  `metadata` blob,
  `state` int NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `completed_at` datetime DEFAULT NULL,
  `locked_until` datetime DEFAULT NULL,
  `sticky_until` datetime DEFAULT NULL,
  `worker` varchar(64) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci DEFAULT NULL,
  `queue` varchar(128) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci DEFAULT '',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_instances_instance_id_execution_id` (`instance_id`,`execution_id`),
  KEY `idx_instances_parent_instance_id_parent_execution_id` (`parent_instance_id`,`parent_execution_id`),
  KEY `idx_instances_locked_until_completed_at_queue` (`completed_at`,`locked_until`,`sticky_until`,`worker`,`queue`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;


--
-- Dumping data for table `instances`
--

LOCK TABLES `instances` WRITE;


UNLOCK TABLES;

--
-- Table structure for table `pending_events`
--

DROP TABLE IF EXISTS `pending_events`;


CREATE TABLE `pending_events` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `event_id` varchar(128) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL,
  `sequence_id` bigint NOT NULL,
  `instance_id` varchar(128) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL,
  `execution_id` varchar(128) CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci NOT NULL,
  `event_type` int NOT NULL,
  `timestamp` datetime NOT NULL,
  `schedule_event_id` bigint NOT NULL,
  `visible_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_pending_events_inid_exid` (`instance_id`,`execution_id`),
  KEY `idx_pending_events_inid_exid_visible_at_schedule_event_id` (`instance_id`,`execution_id`,`visible_at`,`schedule_event_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;


--
-- Dumping data for table `pending_events`
--

LOCK TABLES `pending_events` WRITE;


UNLOCK TABLES;

--
-- Table structure for table `schema_migrations`
--

DROP TABLE IF EXISTS `schema_migrations`;


CREATE TABLE `schema_migrations` (
  `version` bigint NOT NULL,
  `dirty` tinyint(1) NOT NULL,
  PRIMARY KEY (`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;


--
-- Dumping data for table `schema_migrations`
--

LOCK TABLES `schema_migrations` WRITE;

INSERT INTO `schema_migrations` VALUES (4,0);

UNLOCK TABLES;










-- Dump completed on 2024-05-22  4:20:20
