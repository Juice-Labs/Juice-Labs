create type agent_state as enum ('active', 'inactive', 'deleted');

CREATE TABLE agents (
    id BIGSERIAL PRIMARY KEY,
    state agent_state NOT NULL,
    version varchar(255) NOT NULL,
    hostname varchar(255) NOT NULL,
    address inet NOT NULL,
    max_sessions int NOT NULL,
    gpus jsonb NOT NULL,
    created_at TIMESTAMP default now(),
    updated_at TIMESTAMP
);

create table tags (
    id BIGSERIAL PRIMARY KEY,
    name varchar(255) NOT NULL,
    created_at TIMESTAMP default now(),
    updated_at TIMESTAMP
);

create table taints (
    id BIGSERIAL PRIMARY KEY,
    key varchar(255) NOT NULL,
    value varchar(255) NOT NULL,
    created_at TIMESTAMP default now(),
    updated_at TIMESTAMP
);

create table agent_tags (
    agent_id BIGINT NOT NULL,
    tag_id BIGINT NOT NULL,
    created_at TIMESTAMP default now(),
    PRIMARY KEY (agent_id, tag_id),
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

create table agent_taints (
    agent_id BIGINT NOT NULL,
    taint_id BIGINT NOT NULL,
    created_at TIMESTAMP default now(),
    PRIMARY KEY (agent_id, taint_id),
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (taint_id) REFERENCES taints(id) ON DELETE CASCADE
);

create type session_state as enum ('queued', 'assigned', 'active', 'inactive', 'closed');

create table sessions (
    id BIGSERIAL PRIMARY KEY,
    agent_id BIGINT NOT NULL,
    state session_state NOT NULL,
    address inet NOT NULL,
    version varchar(255) NOT NULL,
    gpu jsonb NOT NULL,
    created_at TIMESTAMP default now(),
    updated_at TIMESTAMP,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);