
-- +migrate Up
CREATE TYPE transaction_session_type AS ENUM (
    'online',
    'offline'
);

CREATE TYPE transaction_payment_status AS ENUM (
    'pending',
    'completed',
    'failed',
    'refunded',
    'partial_refund'
);

CREATE TYPE transaction_refund_status AS ENUM (
    'none',
    'pending',
    'processing',
    'refunded',
    'partial_refund',
    'failed',
    'cancelled'
);


CREATE TABLE transactions (
    id VARCHAR(255) PRIMARY KEY,
    patient_id VARCHAR(255) NOT NULL,
    practitioner_id VARCHAR(255) NOT NULL,
    payment_link TEXT,
    status_payment transaction_payment_status NOT NULL,
    amount NUMERIC(10, 2) NOT NULL,
    currency VARCHAR(10) DEFAULT 'IDR', 
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    length_minutes_per_session INT,
    session_total INT,
    session_type transaction_session_type NOT NULL,
    notes TEXT,
    refund_status transaction_refund_status DEFAULT 'none',
    refund_amount NUMERIC(10, 2) DEFAULT 0.00,
    audit_log JSONB DEFAULT '[]'
);


CREATE INDEX idx_transactions_patient_id ON transactions(patient_id);
CREATE INDEX idx_transactions_practitioner_id ON transactions(practitioner_id);
CREATE INDEX idx_transactions_status_payment ON transactions(status_payment);
CREATE INDEX idx_transactions_refund_status ON transactions(refund_status);

-- +migrate Down
DROP TABLE IF EXISTS transactions;

DROP TYPE IF EXISTS transaction_session_type;

DROP TYPE IF EXISTS transaction_payment_status;

DROP TYPE IF EXISTS transaction_refund_status;
