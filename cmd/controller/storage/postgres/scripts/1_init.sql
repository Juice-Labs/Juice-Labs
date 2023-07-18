create type agent_state as enum (
    'active',
    'disabled',
    'missing',
    'closed'
);
create type session_state as enum (
    'queued',
    'assigned',
    'active',
    'canceling',
    'closed',
    'failed',
    'canceled'
);

create extension "uuid-ossp";

create table log (
    created_at TIMESTAMP DEFAULT now(),
    operation text NOT NULL,
    data jsonb NOT NULL
);

create index on log (created_at);

create table agents (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    state agent_state NOT NULL,
    hostname text NOT NULL,
    address text NOT NULL,
    version text NOT NULL,
    max_sessions int NOT NULL,
    gpus jsonb NOT NULL,
    vram_available bigint NOT NULL,
    sessions_available int NOT NULL,
    created_at TIMESTAMP DEFAULT now(),
    updated_at TIMESTAMP
);

create index on agents (state, vram_available, sessions_available);
create index on agents (created_at);
create index on agents (state, created_at);
create index on agents (state, updated_at);

create table sessions (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id uuid,
    state session_state NOT NULL,
    address text,
    version text NOT NULL,
    persistent boolean NOT NULL,
    gpus jsonb,
    vram_required bigint NOT NULL,
    requirements jsonb NOT NULL,
    created_at TIMESTAMP DEFAULT now(),
    updated_at TIMESTAMP,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

create index on sessions (agent_id);
create index on sessions (created_at);
create index on sessions (state, created_at);

create table key_values (
    id BIGSERIAL PRIMARY KEY,
    key text NOT NULL,
    value text NOT NULL
);

create unique index on key_values (key, value);

create table agent_labels (
    agent_id uuid NOT NULL,
    key_value_id bigint NOT NULL,
    PRIMARY KEY (agent_id, key_value_id),
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (key_value_id) REFERENCES key_values(id) ON DELETE RESTRICT ON UPDATE RESTRICT
);

create table agent_taints (
    agent_id uuid NOT NULL,
    key_value_id bigint NOT NULL,
    PRIMARY KEY (agent_id, key_value_id),
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (key_value_id) REFERENCES key_values(id) ON DELETE RESTRICT ON UPDATE RESTRICT
);

create table session_match_labels (
    session_id uuid NOT NULL,
    key_value_id bigint NOT NULL,
    PRIMARY KEY (session_id, key_value_id),
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (key_value_id) REFERENCES key_values(id) ON DELETE RESTRICT ON UPDATE RESTRICT
);

create table session_tolerates (
    session_id uuid NOT NULL,
    key_value_id bigint NOT NULL,
    PRIMARY KEY (session_id, key_value_id),
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (key_value_id) REFERENCES key_values(id) ON DELETE RESTRICT ON UPDATE RESTRICT
);
