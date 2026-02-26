CREATE TABLE `log`
(
    `id`             int unsigned NOT NULL AUTO_INCREMENT,
    `container_name` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
    `trace_id`       varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
    `level`          varchar(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci  DEFAULT NULL,
    `message`        longtext CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci,
    `caller`         varchar(200) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
    `extra`          json                                                          DEFAULT NULL,
    `time`           timestamp    NULL                                             DEFAULT NULL,
    `container_id`   varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci DEFAULT NULL,
    PRIMARY KEY (`id`),
    KEY `container_name` (`container_name`),
    KEY `trace_id` (`trace_id`),
    KEY `container_name_level` (`container_name`, `level`),
    KEY `container_name_time` (`container_name`, `time`),
    FULLTEXT KEY `message` (`message`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_0900_ai_ci;
