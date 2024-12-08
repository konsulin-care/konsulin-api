-- +migrate Up
CREATE TABLE users (
    id SERIAL PRIMARY KEY,                             
    email VARCHAR(255) UNIQUE NOT NULL,               
    gender VARCHAR(50),                               
    role_id INTEGER REFERENCES roles(id) ON DELETE SET NULL, 
    address TEXT,                                     
    fullname VARCHAR(255),                            
    username VARCHAR(255) UNIQUE NOT NULL,           
    password VARCHAR(255) NOT NULL,                           
    birth_date VARCHAR(20),                                  
    patient_id VARCHAR(50),                               
    reset_token VARCHAR(255),                         
    whatsapp_otp VARCHAR(10),                         
    whatsapp_number VARCHAR(15),                     
    practitioner_id VARCHAR(50),                         
    profile_picture_name VARCHAR(255),               
    educations TEXT[],                                
    reset_token_expiry TIMESTAMP,                    
    whatsapp_otp_expiry TIMESTAMP,                   
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),     
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),     
    deleted_at TIMESTAMP                             
);


-- +migrate Down
DROP TABLE IF EXISTS users;