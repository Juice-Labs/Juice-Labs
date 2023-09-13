create type connection_exit_status as enum (
    'unknown',
    'success',
    'failure',
    'canceled'
);

create table connections (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id uuid,
    exit_status connection_exit_status NOT NULL,
    pid bigint NOT NULL,
    process_name text NOT NULL,
    created_at TIMESTAMP DEFAULT now(),
    updated_at TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

alter table
    sessions drop column exit_status;
    