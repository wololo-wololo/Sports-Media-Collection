-- Winner of an event session (pole sitter for qualifying), as reported by
-- the series data source or entered by hand.
ALTER TABLE `event_sessions` ADD COLUMN `winner` varchar(255);
