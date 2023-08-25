-- Create Pool table
CREATE TABLE pools (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    pool_name VARCHAR(255) NOT NULL,
    max_agents INT DEFAULT 0
);

create type permission as enum (
    'create_session',
    'register_agent',
    'admin'
);

-- Create Permissions table
CREATE TABLE permissions (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id text NOT NULL,
    pool_id UUID NOT NULL,
    permission permission NOT NULL,
    FOREIGN KEY (pool_id) REFERENCES pools(id) ON DELETE CASCADE
);


DO $$ BEGIN
    ALTER TABLE sessions
    DROP COLUMN IF EXISTS pool_id;
EXCEPTION
    WHEN OTHERS THEN -- ignore errors
END $$;

-- Modify Agents table
DO $$ BEGIN
    ALTER TABLE agents
    DROP COLUMN IF EXISTS pool_id;
EXCEPTION
    WHEN OTHERS THEN -- ignore errors
END $$;

-- Modify Sessions table
ALTER TABLE sessions
ADD COLUMN pool_id UUID,
ADD FOREIGN KEY (pool_id) REFERENCES pools(id) ON DELETE CASCADE;

-- Modify Agents table
ALTER TABLE agents
ADD COLUMN pool_id UUID,
ADD FOREIGN KEY (pool_id) REFERENCES pools(id) ON DELETE CASCADE;
