-- Drop tables in reverse order to respect foreign key constraints
DROP TABLE IF EXISTS user_organizations;
DROP TABLE IF EXISTS organizations;
DROP TABLE IF EXISTS users;
