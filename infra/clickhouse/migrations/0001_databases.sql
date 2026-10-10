-- Create application databases.
-- meta is bootstrapped by the Migrator before migrations run; listed here for documentation.
CREATE DATABASE IF NOT EXISTS raw;
CREATE DATABASE IF NOT EXISTS staging;
CREATE DATABASE IF NOT EXISTS marts
