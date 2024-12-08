
-- +migrate Up
CREATE TABLE education_levels (
    id SERIAL PRIMARY KEY,                     
    code VARCHAR(50) NOT NULL,                 
    display VARCHAR(255) NOT NULL,             
    system TEXT NOT NULL,                      
    definition TEXT NOT NULL,                           
    internal_id INTEGER NOT NULL UNIQUE,       
    status VARCHAR(50) NOT NULL,               
    custom_display VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP NULL
);

-- +migrate Down
DROP TABLE IF EXISTS education_levels;
