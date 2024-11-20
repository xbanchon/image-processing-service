CREATE TABLE IF NOT EXISTS images(
    id bigserial PRIMARY KEY,
    url text NOT NULL,
    filename VARCHAR(255) NOT NULL,
    user_id bigint REFERENCES users,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    updated_at timestamp(0) with time zone NOT NULL DEFAULT NOW()
);