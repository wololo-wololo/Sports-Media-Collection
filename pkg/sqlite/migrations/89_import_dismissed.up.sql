-- Recordings the user dismissed from the import review list, e.g. files
-- that are not sports recordings.
ALTER TABLE `scenes` ADD COLUMN `import_dismissed` boolean not null default '0';
