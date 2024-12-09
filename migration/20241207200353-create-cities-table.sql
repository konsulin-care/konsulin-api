
-- +migrate Up
CREATE TABLE cities (
    id SERIAL PRIMARY KEY,      
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP NULL
);
-- +migrate Down
DROP TABLE IF EXISTS cities;
