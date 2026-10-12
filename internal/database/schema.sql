-- Esquema API de tarjetas.

CREATE TABLE cards (
    id UUID PRIMARY KEY,
    holder_name TEXT NOT NULL,
    pan_hash BYTEA NOT NULL,
    last4 CHAR(4) NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expiration_month VARCHAR(2) NOT NULL,
    expiration_year VARCHAR(4) NOT NULL,
    CONSTRAINT cards_last4_digits_chk CHECK (last4 ~ '^[0-9]{4}$'),
    CONSTRAINT cards_expiration_month_chk CHECK (expiration_month ~ '^(0[1-9]|1[0-2])$'),
    CONSTRAINT cards_expiration_year_chk CHECK (expiration_year ~ '^[0-9]{4}$')
);

CREATE INDEX cards_last4_idx ON cards (last4);
CREATE INDEX cards_issued_at_id_idx ON cards (issued_at DESC, id DESC);

CREATE TABLE idempotency_keys (
    operation TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    request_hash BYTEA NOT NULL,
    card_id UUID REFERENCES cards (id) ON DELETE SET NULL,
    response_status SMALLINT,
    response_body JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT idempotency_keys_pkey PRIMARY KEY (operation, idempotency_key),
    CONSTRAINT idempotency_keys_key_length_chk
        CHECK (length(idempotency_key) BETWEEN 1 AND 255),
    CONSTRAINT idempotency_keys_request_hash_length_chk
        CHECK (octet_length(request_hash) = 32),
    CONSTRAINT idempotency_keys_response_status_chk
        CHECK (response_status IS NULL OR response_status BETWEEN 100 AND 599),
    CONSTRAINT idempotency_keys_expiration_chk CHECK (expires_at > created_at),
    CONSTRAINT idempotency_keys_response_pair_chk
        CHECK ((response_status IS NULL) = (response_body IS NULL))
);

CREATE INDEX idempotency_keys_expires_at_idx ON idempotency_keys (expires_at);
