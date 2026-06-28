CREATE DATABASE IF NOT EXISTS egoadmin_dtm
/*!40100 DEFAULT CHARACTER SET utf8mb4 */
;

USE egoadmin_dtm;

CREATE TABLE IF NOT EXISTS trans_global (
  `id` bigint(22) NOT NULL AUTO_INCREMENT,
  `gid` varchar(128) NOT NULL COMMENT 'global transaction id',
  `trans_type` varchar(45) NOT NULL COMMENT 'transaction type: saga | xa | tcc | msg',
  `status` varchar(12) NOT NULL COMMENT 'transaction status: prepared | submitted | aborting | succeed | failed',
  `query_prepared` varchar(1024) NOT NULL COMMENT 'url to check for msg|workflow',
  `protocol` varchar(45) NOT NULL COMMENT 'protocol: http | grpc | json-rpc',
  `create_time` datetime DEFAULT NULL,
  `update_time` datetime DEFAULT NULL,
  `finish_time` datetime DEFAULT NULL,
  `rollback_time` datetime DEFAULT NULL,
  `options` varchar(1024) DEFAULT '' COMMENT 'options for transaction like: TimeoutToFail, RequestTimeout',
  `custom_data` varchar(1024) DEFAULT '' COMMENT 'custom data for transaction',
  `next_cron_interval` int(11) DEFAULT NULL COMMENT 'next cron interval. for use of cron job',
  `next_cron_time` datetime DEFAULT NULL COMMENT 'next time to process this trans. for use of cron job',
  `owner` varchar(128) NOT NULL DEFAULT '' COMMENT 'who is locking this trans',
  `ext_data` text COMMENT 'extra data for this trans. currently used in workflow pattern',
  `result` varchar(1024) DEFAULT '' COMMENT 'result for transaction',
  `rollback_reason` varchar(1024) DEFAULT '' COMMENT 'rollback reason for transaction',
  PRIMARY KEY (`id`),
  UNIQUE KEY `gid` (`gid`),
  KEY `owner` (`owner`),
  KEY `status_next_cron_time` (`status`, `next_cron_time`) COMMENT 'cron job will use this index to query trans'
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE IF NOT EXISTS trans_branch_op (
  `id` bigint(22) NOT NULL AUTO_INCREMENT,
  `gid` varchar(128) NOT NULL COMMENT 'global transaction id',
  `url` varchar(1024) NOT NULL COMMENT 'the url of this op',
  `data` text COMMENT 'request body, deprecated',
  `bin_data` blob COMMENT 'request body',
  `branch_id` varchar(128) NOT NULL COMMENT 'transaction branch ID',
  `op` varchar(45) NOT NULL COMMENT 'transaction operation type like: action | compensate | try | confirm | cancel',
  `status` varchar(45) NOT NULL COMMENT 'transaction op status: prepared | succeed | failed',
  `finish_time` datetime DEFAULT NULL,
  `rollback_time` datetime DEFAULT NULL,
  `create_time` datetime DEFAULT NULL,
  `update_time` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `gid_uniq` (`gid`, `branch_id`, `op`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE IF NOT EXISTS kv (
  `id` bigint(22) NOT NULL AUTO_INCREMENT,
  `cat` varchar(45) NOT NULL COMMENT 'the category of this data',
  `k` varchar(128) NOT NULL,
  `v` text,
  `version` bigint(22) DEFAULT 1,
  `create_time` datetime DEFAULT NULL,
  `update_time` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_k` (`cat`, `k`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
