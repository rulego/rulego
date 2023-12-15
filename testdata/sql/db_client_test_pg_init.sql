DROP TABLE IF EXISTS users;
CREATE TABLE "public"."users"
(
    "id"              int4 NOT NULL,
    "name"            varchar(255) COLLATE "pg_catalog"."default",
    "age"             int4,
    "login_count"     int4                                DEFAULT 0,
    "additional_info" varchar(255) COLLATE "pg_catalog"."default",
    "create_time"     date,
    "info"            text COLLATE "pg_catalog"."default" DEFAULT ''::text,
    "isactive"        bool                                DEFAULT false
);