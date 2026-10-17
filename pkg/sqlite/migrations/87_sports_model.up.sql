-- Sports data model: series > events (season + round) > sessions,
-- with recordings (scenes) attached to an event and optionally a session.

CREATE TABLE `series` (
  `id` integer not null primary key autoincrement,
  `name` varchar(255) not null,
  `short_name` varchar(32),
  `sport` varchar(64) not null default 'Motorsport',
  `color` varchar(16),
  `data_source` varchar(64),
  `created_at` datetime not null,
  `updated_at` datetime not null,
  CHECK (`name` != '')
);
CREATE UNIQUE INDEX `index_series_on_name` on `series` (`name`);

CREATE TABLE `events` (
  `id` integer not null primary key autoincrement,
  `series_id` integer not null,
  `season` integer not null,
  `round` integer,
  `name` varchar(255) not null,
  `circuit` varchar(255),
  `location` varchar(255),
  `country` varchar(255),
  `start_date` date,
  `end_date` date,
  `external_id` varchar(255),
  `created_at` datetime not null,
  `updated_at` datetime not null,
  foreign key(`series_id`) references `series`(`id`) on delete CASCADE,
  CHECK (`name` != '')
);
CREATE INDEX `index_events_on_series_id_season` on `events` (`series_id`, `season`);
CREATE UNIQUE INDEX `index_events_on_series_id_season_round` on `events` (`series_id`, `season`, `round`) WHERE `round` IS NOT NULL;
CREATE UNIQUE INDEX `index_events_on_series_id_external_id` on `events` (`series_id`, `external_id`) WHERE `external_id` IS NOT NULL;

CREATE TABLE `event_sessions` (
  `id` integer not null primary key autoincrement,
  `event_id` integer not null,
  `session_type` varchar(32) not null,
  `name` varchar(255),
  `start_time` datetime,
  `created_at` datetime not null,
  `updated_at` datetime not null,
  foreign key(`event_id`) references `events`(`id`) on delete CASCADE
);
CREATE INDEX `index_event_sessions_on_event_id` on `event_sessions` (`event_id`);

ALTER TABLE `scenes` ADD COLUMN `event_id` integer REFERENCES `events`(`id`) ON DELETE SET NULL;
ALTER TABLE `scenes` ADD COLUMN `session_id` integer REFERENCES `event_sessions`(`id`) ON DELETE SET NULL;
ALTER TABLE `scenes` ADD COLUMN `language` varchar(16);
ALTER TABLE `scenes` ADD COLUMN `feed_type` varchar(32);
ALTER TABLE `scenes` ADD COLUMN `feed_subject` varchar(255);
ALTER TABLE `scenes` ADD COLUMN `source` varchar(32);
ALTER TABLE `scenes` ADD COLUMN `completeness` varchar(32);
ALTER TABLE `scenes` ADD COLUMN `commentators` text;
ALTER TABLE `scenes` ADD COLUMN `sync_offset` real;

CREATE INDEX `index_scenes_on_event_id` on `scenes` (`event_id`);
CREATE INDEX `index_scenes_on_session_id` on `scenes` (`session_id`);
