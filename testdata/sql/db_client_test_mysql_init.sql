DROP TABLE IF EXISTS `users`;
CREATE TABLE `users`
(
    `id`              int NOT NULL ,
    `name`            varchar(255) ,
    `age`             int   DEFAULT 0,
    `login_count`     bigint  DEFAULT 0,
    `additional_info` varchar(1024) ,
    `create_time`     datetime ,
    `info`            text ,
    `num`             double  DEFAULT 0,
    PRIMARY KEY (`id`) USING BTREE
);