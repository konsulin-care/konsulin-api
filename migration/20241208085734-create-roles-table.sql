
-- +migrate Up
CREATE TYPE action_type AS ENUM ('read', 'update', 'create', 'delete');

CREATE TABLE roles (
    id SERIAL PRIMARY KEY,                  
    name VARCHAR(255) NOT NULL UNIQUE,      
    created_at TIMESTAMP NOT NULL DEFAULT NOW(), 
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(), 
    deleted_at TIMESTAMP NULL               
);

CREATE TABLE role_permissions (
    id SERIAL PRIMARY KEY,                 
    role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    resource VARCHAR(255) NOT NULL,        
    actions action_type[] NOT NULL,        
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP NULL              
);

-- +migrate Down
DROP TABLE IF EXISTS role_permissions;

DROP TABLE IF EXISTS roles;

DROP TYPE IF EXISTS action_type;