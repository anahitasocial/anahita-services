-- The PHP application's schema, without a single row: what a site running
-- it has before legacy/upgrade.sql. Taken with mysqldump --no-data from a
-- copy of anahita.io's production database (2022-09-06), with nothing of the
-- server it came from. check_schema.sh upgrades it and compares the result
-- with schema/init.sql.


/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!50503 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;
DROP TABLE IF EXISTS `jos_components`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `jos_components` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(50) NOT NULL DEFAULT '',
  `parent` int unsigned NOT NULL DEFAULT '0',
  `option` varchar(50) NOT NULL DEFAULT '',
  `ordering` int NOT NULL DEFAULT '0',
  `iscore` tinyint NOT NULL DEFAULT '0',
  `meta` text,
  `enabled` tinyint NOT NULL DEFAULT '1',
  PRIMARY KEY (`id`),
  KEY `parent_option` (`parent`,`option`(32))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `jos_connect_sessions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `jos_connect_sessions` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `component` varchar(255) NOT NULL,
  `owner_id` bigint NOT NULL,
  `owner_type` varchar(100) NOT NULL,
  `token_key` text,
  `token_secret` text,
  `api` varchar(100) NOT NULL,
  `update_status` tinyint(1) NOT NULL DEFAULT '0',
  `profile_id` varchar(30) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `owner_id_and_profile_id_and_service_id` (`owner_id`,`profile_id`,`api`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `jos_edges`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `jos_edges` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `type` varchar(255) CHARACTER SET latin1 COLLATE latin1_swedish_ci NOT NULL,
  `node_a_id` bigint unsigned NOT NULL,
  `node_a_type` varchar(255) CHARACTER SET latin1 COLLATE latin1_swedish_ci NOT NULL,
  `node_b_id` bigint unsigned NOT NULL,
  `node_b_type` varchar(255) CHARACTER SET latin1 COLLATE latin1_swedish_ci NOT NULL,
  `meta` text,
  `created_on` datetime DEFAULT NULL,
  `created_by` bigint unsigned DEFAULT NULL,
  `modified_on` datetime DEFAULT NULL,
  `modified_by` bigint unsigned DEFAULT NULL,
  `ordering` int unsigned DEFAULT NULL,
  `start_date` datetime DEFAULT NULL,
  `end_date` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uc_edge` (`type`,`node_a_id`,`node_a_type`,`node_b_id`,`node_b_type`),
  KEY `type` (`type`),
  KEY `node_a_id` (`node_a_id`),
  KEY `node_b_id` (`node_b_id`),
  KEY `start_date` (`start_date`),
  KEY `end_date` (`end_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `jos_migrator_versions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `jos_migrator_versions` (
  `id` int NOT NULL AUTO_INCREMENT,
  `component` varchar(255) CHARACTER SET latin1 COLLATE latin1_swedish_ci NOT NULL,
  `version` text CHARACTER SET latin1 COLLATE latin1_swedish_ci NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `component` (`component`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `jos_nodes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `jos_nodes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `type` varchar(255) CHARACTER SET latin1 COLLATE latin1_swedish_ci NOT NULL,
  `component` varchar(100) CHARACTER SET latin1 COLLATE latin1_swedish_ci NOT NULL,
  `name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  `alias` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
  `body` mediumtext CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci,
  `start_date` datetime DEFAULT NULL,
  `end_date` datetime DEFAULT NULL,
  `owner_type` varchar(255) CHARACTER SET latin1 COLLATE latin1_swedish_ci DEFAULT NULL,
  `owner_id` bigint unsigned DEFAULT NULL,
  `comment_status` tinyint(1) DEFAULT NULL,
  `comment_count` int unsigned DEFAULT NULL,
  `last_comment_id` bigint unsigned DEFAULT NULL,
  `last_comment_by` bigint unsigned DEFAULT NULL,
  `last_comment_on` datetime DEFAULT NULL,
  `ordering` int DEFAULT NULL,
  `enabled` tinyint(1) NOT NULL DEFAULT '0',
  `verified` tinyint(1) NOT NULL DEFAULT '0',
  `pinned` tinyint(1) NOT NULL DEFAULT '0',
  `is_default` tinyint(1) NOT NULL DEFAULT '0',
  `meta` text,
  `hits` int unsigned DEFAULT NULL,
  `parent_id` bigint unsigned DEFAULT NULL,
  `parent_type` varchar(255) CHARACTER SET latin1 COLLATE latin1_swedish_ci DEFAULT NULL,
  `geo_latitude` float(10,6) DEFAULT NULL,
  `geo_longitude` float(10,6) DEFAULT NULL,
  `geo_address` varchar(255) DEFAULT NULL,
  `geo_city` varchar(50) DEFAULT NULL,
  `geo_state_province` varchar(50) DEFAULT NULL,
  `geo_country` varchar(30) DEFAULT NULL,
  `geo_postalcode` varchar(15) DEFAULT NULL,
  `created_on` datetime DEFAULT NULL,
  `created_by` bigint unsigned DEFAULT NULL,
  `modified_on` datetime DEFAULT NULL,
  `modified_by` bigint unsigned DEFAULT NULL,
  `filename` varchar(255) CHARACTER SET latin1 COLLATE latin1_swedish_ci DEFAULT NULL,
  `filesize` int DEFAULT NULL,
  `cover_filename` varchar(255) DEFAULT NULL,
  `cover_filesize` int DEFAULT NULL,
  `cover_mimetype` varchar(100) DEFAULT NULL,
  `status` text,
  `status_update_time` datetime DEFAULT NULL,
  `follower_count` int unsigned DEFAULT NULL,
  `leader_count` int unsigned DEFAULT NULL,
  `mutual_count` int unsigned DEFAULT NULL,
  `leader_ids` mediumtext CHARACTER SET latin1 COLLATE latin1_swedish_ci,
  `follower_ids` mediumtext CHARACTER SET latin1 COLLATE latin1_swedish_ci,
  `mutual_ids` mediumtext CHARACTER SET latin1 COLLATE latin1_swedish_ci,
  `blocker_ids` text CHARACTER SET latin1 COLLATE latin1_swedish_ci,
  `allow_follow_request` tinyint unsigned DEFAULT NULL,
  `follow_requester_ids` mediumtext CHARACTER SET latin1 COLLATE latin1_swedish_ci,
  `subscriber_count` int unsigned DEFAULT NULL,
  `subscriber_ids` mediumtext CHARACTER SET latin1 COLLATE latin1_swedish_ci,
  `administrating_ids` text CHARACTER SET latin1 COLLATE latin1_swedish_ci,
  `notification_ids` text CHARACTER SET latin1 COLLATE latin1_swedish_ci,
  `new_notification_ids` text CHARACTER SET latin1 COLLATE latin1_swedish_ci,
  `vote_up_count` int unsigned DEFAULT NULL,
  `vote_down_count` int unsigned DEFAULT NULL,
  `voter_up_ids` mediumtext CHARACTER SET latin1 COLLATE latin1_swedish_ci,
  `voter_down_ids` mediumtext CHARACTER SET latin1 COLLATE latin1_swedish_ci,
  `shared_owner_count` int unsigned DEFAULT NULL,
  `shared_owner_ids` mediumtext CHARACTER SET latin1 COLLATE latin1_swedish_ci,
  `administrator_ids` text CHARACTER SET latin1 COLLATE latin1_swedish_ci,
  `blocked_ids` text CHARACTER SET latin1 COLLATE latin1_swedish_ci,
  `excerpt` text CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci,
  `mimetype` varchar(100) CHARACTER SET latin1 COLLATE latin1_swedish_ci DEFAULT NULL,
  `story_subject_id` bigint unsigned DEFAULT NULL,
  `story_object_type` varchar(255) CHARACTER SET latin1 COLLATE latin1_swedish_ci DEFAULT NULL,
  `story_object_id` bigint unsigned DEFAULT NULL,
  `story_target_id` bigint unsigned DEFAULT NULL,
  `story_comment_id` int unsigned DEFAULT NULL,
  `timezone` int DEFAULT NULL,
  `language` varchar(50) DEFAULT NULL,
  `access` text CHARACTER SET latin1 COLLATE latin1_swedish_ci,
  `permissions` text CHARACTER SET latin1 COLLATE latin1_swedish_ci,
  PRIMARY KEY (`id`),
  KEY `last_comment_by` (`last_comment_by`),
  KEY `created_by` (`created_by`),
  KEY `modified_by` (`modified_by`),
  KEY `type` (`type`),
  KEY `component` (`component`),
  KEY `owner_id` (`owner_id`),
  KEY `parent_id` (`parent_id`),
  KEY `story_target_id` (`story_target_id`),
  KEY `story_object_id` (`story_object_id`),
  KEY `story_subject_id` (`story_subject_id`),
  KEY `story_comment_id` (`story_comment_id`),
  KEY `start_date` (`start_date`),
  KEY `end_date` (`end_date`),
  KEY `type_enabled` (`type`,`enabled`),
  KEY `type_modifed_on` (`type`,`modified_on`),
  KEY `type_created_on` (`type`,`created_on`),
  KEY `type_status_update_time` (`type`,`status_update_time`),
  KEY `type_default` (`type`,`is_default`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `jos_people_people`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `jos_people_people` (
  `people_person_id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `node_id` bigint unsigned NOT NULL,
  `email` varchar(255) DEFAULT NULL,
  `username` varchar(255) DEFAULT NULL,
  `password` varchar(255) DEFAULT NULL,
  `usertype` varchar(50) DEFAULT NULL,
  `gender` varchar(50) DEFAULT NULL,
  `given_name` varchar(255) DEFAULT NULL,
  `family_name` varchar(255) DEFAULT NULL,
  `network_presence` tinyint NOT NULL DEFAULT '0',
  `last_visit_date` datetime DEFAULT NULL,
  `time_zone` int DEFAULT NULL,
  `language` varchar(100) DEFAULT NULL,
  `activation_code` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`people_person_id`),
  UNIQUE KEY `people_person_id` (`people_person_id`),
  UNIQUE KEY `node_id` (`node_id`),
  UNIQUE KEY `username` (`username`),
  UNIQUE KEY `email` (`email`),
  KEY `usertype` (`usertype`),
  KEY `last_visit_date` (`last_visit_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `jos_plugins`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `jos_plugins` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(100) NOT NULL DEFAULT '',
  `element` varchar(100) NOT NULL DEFAULT '',
  `folder` varchar(100) NOT NULL DEFAULT '',
  `ordering` int NOT NULL DEFAULT '0',
  `enabled` tinyint NOT NULL DEFAULT '0',
  `iscore` tinyint NOT NULL DEFAULT '0',
  `meta` text,
  PRIMARY KEY (`id`),
  KEY `idx_folder` (`enabled`,`folder`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `jos_sessions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `jos_sessions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `session_id` char(64) NOT NULL,
  `node_id` bigint NOT NULL DEFAULT '0',
  `username` varchar(255) DEFAULT NULL,
  `usertype` varchar(255) DEFAULT NULL,
  `time` int DEFAULT '0',
  `guest` tinyint DEFAULT '1',
  `meta` longtext,
  PRIMARY KEY (`id`),
  UNIQUE KEY `id` (`id`),
  UNIQUE KEY `session_id` (`session_id`),
  KEY `whosonline` (`guest`,`usertype`,`username`),
  KEY `node_id` (`node_id`),
  KEY `username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `jos_todos_todos`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `jos_todos_todos` (
  `todos_todo_id` bigint NOT NULL AUTO_INCREMENT,
  `node_id` bigint NOT NULL,
  `open_status_change_time` datetime NOT NULL DEFAULT '0000-00-00 00:00:00',
  `open_status_change_by` bigint DEFAULT NULL,
  PRIMARY KEY (`todos_todo_id`),
  UNIQUE KEY `node_id` (`node_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

