DROP TABLE IF EXISTS `users`;
CREATE TABLE `users`
(
    `id`              int(0) NOT NULL DEFAULT 0,
    `name`            varchar(255) NULL,
    `age`             int(0) NULL DEFAULT NULL,
    `login_count`     bigint(0) NULL DEFAULT 0,
    `additional_info` varchar(255) NULL,
    `create_time`     datetime(0) NULL DEFAULT NULL,
    `info`            text CHARACTER,
    `num`             double NULL DEFAULT 0,
    PRIMARY KEY (`id`) USING BTREE
);