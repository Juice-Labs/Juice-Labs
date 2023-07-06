/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
)

const (
	CreateUuidExtension = "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\""

	CreateGpuType = "DO $$ " +
		"BEGIN " +
		"IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'gpu') THEN " +
		"CREATE TYPE Gpu AS (" +
		"name text," +
		"vendorId smallint," +
		"deviceId smallint," +
		"vram bigint" +
		");" +
		"END IF; " +
		"END; $$"

	CreateKeyValueType = "DO $$ " +
		"BEGIN " +
		"IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'keyvalue') THEN " +
		"CREATE TYPE keyvalue AS (" +
		"key text," +
		"value text" +
		");" +
		"END IF; " +
		"END; $$"

	CreateAgentsTable = "CREATE TABLE IF NOT EXISTS agents (" +
		"id uuid NOT NULL PRIMARY KEY DEFAULT uuid_generate_v4()," +
		"state smallint NOT NULL," +
		"version text NOT NULL," +
		"hostname text NOT NULL," +
		"address text NOT NULL," +
		"max_sessions int NOT NULL," +
		"gpus gpu[] NOT NULL," +
		"tags keyvalue[] NOT NULL," +
		"taints keyvalue[] NOT NULL," +
		"sessions uuid[]," +
		"lastUpdated timestamp NOT NULL," +
		"data text NOT NULL" +
		")"

	CreateSessionsTable = "CREATE TABLE IF NOT EXISTS sessions (" +
		"id uuid NOT NULL PRIMARY KEY DEFAULT uuid_generate_v4()," +
		"agent_id uuid REFERENCES agents (id)," +
		"state smallint NOT NULL," +
		"address text," +
		"version text NOT NULL," +
		"gpus gpu[]," +
		"lastUpdated timestamp NOT NULL," +
		"data text NOT NULL" +
		")"

	InsertIntoAgentsTable = "INSERT INTO agents (" +
		"state, version, hostname, address, max_sessions, gpus, tags, taints, sessions, lastUpdated, data" +
		") VALUES (" +
		"$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11" +
		") RETURNING id"

	UpdateAgent = "UPDATE agents SET (" +
		"state, sessions, lastUpdated, data" +
		") = (" +
		"$1, $2, $3, $4" +
		")" +
		"WHERE id = $5"

	SelectActiveAgents = "SELECT data FROM agents WHERE state = 2"

	SelectAgentsUpdatedSince = "SELECT data FROM agents WHERE lastUpdated > $1"

	InsertIntoSessionsTable = "INSERT INTO sessions (" +
		"agent_id, state, address, version, gpus, lastUpdated, data" +
		") VALUES (" +
		"$1, $2, $3, $4, $5, $6, $7" +
		") RETURNING id"

	UpdateSession = "UPDATE sessions SET (" +
		"agent_id, state, address, gpus, lastUpdated, data" +
		") = (" +
		"$1, $2, $3, $4, $5, $6" +
		")" +
		"WHERE id = $7"

	SelectSessionById = "SELECT data FROM sessions WHERE id = $1"

	SelectSessionsNotClosedUpdatedSince = "SELECT data FROM sessions " +
		"WHERE state < 4 AND lastUpdated > $1"
)

func appendArrayQuotedBytes(b, v []byte) []byte {
	b = append(b, '"')
	for {
		i := bytes.IndexAny(v, `"\`)
		if i < 0 {
			b = append(b, v...)
			break
		}
		if i > 0 {
			b = append(b, v[:i]...)
		}
		b = append(b, '\\', v[i])
		v = v[i+1:]
	}
	return append(b, '"')
}

func gpuValuerAppend(b []byte, gpu restapi.Gpu) []byte {
	b1 := make([]byte, 0, 64)

	b1 = append(b1, '(')
	b1 = appendArrayQuotedBytes(b1, []byte(gpu.Name))
	b1 = append(b1, ',')
	b1 = fmt.Append(b1, gpu.VendorId)
	b1 = append(b1, ',')
	b1 = fmt.Append(b1, gpu.DeviceId)
	b1 = append(b1, ',')
	b1 = fmt.Append(b1, gpu.Vram)
	b1 = append(b1, ')')

	return appendArrayQuotedBytes(b, b1)
}

type gpuValuer struct {
	Data *[]restapi.Gpu
}

func (valuer gpuValuer) Value() (driver.Value, error) {
	if n := len(*valuer.Data); n > 0 {
		b := make([]byte, 1, 64)
		b[0] = '{'

		b = gpuValuerAppend(b, (*valuer.Data)[0])
		for i := 1; i < n; i++ {
			b = append(b, ',')
			b = gpuValuerAppend(b, (*valuer.Data)[i])
		}

		return string(append(b, '}')), nil
	}

	return "{}", nil
}

type sessionGpuValuer struct {
	Data *[]restapi.SessionGpu
}

func (valuer sessionGpuValuer) Value() (driver.Value, error) {
	if n := len(*valuer.Data); n > 0 {
		b := make([]byte, 1, 64)
		b[0] = '{'

		b = gpuValuerAppend(b, (*valuer.Data)[0].Gpu)
		for i := 1; i < n; i++ {
			b = append(b, ',')
			b = gpuValuerAppend(b, (*valuer.Data)[i].Gpu)
		}

		return string(append(b, '}')), nil
	}

	return "{}", nil
}

type mapStringStringValuer struct {
	Data *map[string]string
}

func mapStringStringValuerAppend(b []byte, key, value string) []byte {
	b1 := make([]byte, 0, 64)

	b1 = append(b1, '(')
	b1 = appendArrayQuotedBytes(b1, []byte(key))
	b1 = append(b1, ',')
	b1 = appendArrayQuotedBytes(b1, []byte(value))
	b1 = append(b1, ')')

	return appendArrayQuotedBytes(b, b1)
}

func (valuer mapStringStringValuer) Value() (driver.Value, error) {
	if n := len(*valuer.Data); n > 0 {
		b := make([]byte, 1, 64)
		b[0] = '{'

		first := true

		for key, value := range *valuer.Data {
			if !first {
				b = append(b, ',')
			}

			b = mapStringStringValuerAppend(b, key, value)

			first = false
		}

		return string(append(b, '}')), nil
	}

	return "{}", nil
}

type sessionUuidValuer struct {
	Data *[]restapi.Session
}

func (valuer sessionUuidValuer) Value() (driver.Value, error) {
	if n := len(*valuer.Data); n > 0 {
		b := make([]byte, 1, 64)
		b[0] = '{'

		b = appendArrayQuotedBytes(b, []byte((*valuer.Data)[0].Id))
		for i := 1; i < n; i++ {
			b = append(b, ',')
			b = appendArrayQuotedBytes(b, []byte((*valuer.Data)[i].Id))
		}

		return string(append(b, '}')), nil
	}

	return "{}", nil
}

type storageDriver struct {
	ctx context.Context
	db  *sql.DB
}

func OpenStorage(ctx context.Context) (storage.Storage, error) {
	db, err := sql.Open("postgres", "user=postgres password='[&yx+c3i89}2<((KcZv4{8mGzNO<' dbname=juice host=controller-dev.cluster-coe6qeujst8t.us-east-2.rds.amazonaws.com sslmode=disable")
	if err != nil {
		return nil, err
	}

	_, err = db.ExecContext(ctx, CreateUuidExtension)
	if err != nil {
		return nil, err
	}

	_, err = db.ExecContext(ctx, CreateGpuType)
	if err != nil {
		return nil, err
	}

	_, err = db.ExecContext(ctx, CreateKeyValueType)
	if err != nil {
		return nil, err
	}

	_, err = db.ExecContext(ctx, CreateAgentsTable)
	if err != nil {
		return nil, err
	}

	_, err = db.ExecContext(ctx, CreateSessionsTable)
	if err != nil {
		return nil, err
	}

	return &storageDriver{
		ctx: ctx,
		db:  db,
	}, nil
}

func (driver *storageDriver) Close() error {
	return driver.db.Close()
}

func (driver *storageDriver) AddAgent(agent storage.Agent) (string, error) {
	var id string

	jsonb, err := json.Marshal(agent)
	if err == nil {
		err = driver.db.QueryRowContext(driver.ctx, InsertIntoAgentsTable,
			agent.State, agent.Version, agent.Hostname, agent.Address,
			agent.MaxSessions, gpuValuer{Data: &agent.Gpus},
			mapStringStringValuer{Data: &agent.Tags},
			mapStringStringValuer{Data: &agent.Taints},
			sessionUuidValuer{Data: &agent.Sessions},
			agent.LastUpdated, jsonb).Scan(&id)
	}

	return id, err
}

func (driver *storageDriver) AddSession(session storage.Session) (string, error) {
	var id string

	jsonb, err := json.Marshal(session)
	if err == nil {
		var agentId any
		agentId = session.AgentId
		if agentId == "" {
			agentId = nil
		}

		err = driver.db.QueryRowContext(driver.ctx, InsertIntoSessionsTable,
			agentId, session.State, session.Address, session.Version,
			sessionGpuValuer{Data: &session.Gpus}, session.LastUpdated, jsonb).Scan(&id)
	}

	return id, err
}

func (driver *storageDriver) GetActiveAgents() ([]storage.Agent, error) {
	rows, err := driver.db.QueryContext(driver.ctx, SelectActiveAgents)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	agents := make([]storage.Agent, 0)
	for rows.Next() {
		var data []byte
		err = rows.Scan(&data)
		if err != nil {
			return nil, err
		}

		var agent storage.Agent
		err = json.Unmarshal(data, &agent)
		if err != nil {
			return nil, err
		}

		agents = append(agents, agent)
	}

	return agents, nil
}

func (driver *storageDriver) UpdateAgentsAndSessions(agents []storage.Agent, sessions []storage.Session) error {
	if len(agents) > 0 || len(sessions) > 0 {
		tx, err := driver.db.BeginTx(driver.ctx, nil)
		if err != nil {
			return err
		}

		for _, agent := range agents {
			jsonb, err := json.Marshal(agent)
			if err != nil {
				return err
			}

			_, err = driver.db.ExecContext(driver.ctx, UpdateAgent,
				agent.State, sessionUuidValuer{Data: &agent.Sessions},
				agent.LastUpdated, jsonb, agent.Id)
			if err != nil {
				return err
			}
		}

		for _, session := range sessions {
			jsonb, err := json.Marshal(session)
			if err != nil {
				return err
			}

			var agentId any
			agentId = session.AgentId
			if agentId == "" {
				agentId = nil
			}

			_, err = driver.db.ExecContext(driver.ctx, UpdateSession,
				agentId, session.State, session.Address,
				sessionGpuValuer{Data: &session.Gpus}, session.LastUpdated,
				jsonb, session.Id)
			if err != nil {
				return err
			}
		}

		return tx.Commit()
	}

	return nil
}

func (driver *storageDriver) GetSessionById(id string) (storage.Session, error) {
	var session storage.Session

	var jsonb []byte
	err := driver.db.QueryRowContext(driver.ctx, SelectSessionById, id).Scan(&jsonb)
	if err != nil {
		return storage.Session{}, err
	}

	err = json.Unmarshal(jsonb, &session)
	return session, err
}

func (driver *storageDriver) GetAgentsAndSessionsUpdatedSince(time time.Time) ([]storage.Agent, []storage.Session, error) {
	rows, err := driver.db.QueryContext(driver.ctx, SelectAgentsUpdatedSince, time)
	if err != nil {
		return nil, nil, err
	}

	defer rows.Close()

	agents := make([]storage.Agent, 0)
	for rows.Next() {
		var data []byte
		err = rows.Scan(&data)
		if err != nil {
			return nil, nil, err
		}

		var agent storage.Agent
		err = json.Unmarshal(data, &agent)
		if err != nil {
			return nil, nil, err
		}

		agents = append(agents, agent)
	}

	rows, err = driver.db.QueryContext(driver.ctx, SelectSessionsNotClosedUpdatedSince, time)
	if err != nil {
		return nil, nil, err
	}

	defer rows.Close()

	sessions := make([]storage.Session, 0)
	for rows.Next() {
		var data []byte
		err = rows.Scan(&data)
		if err != nil {
			return nil, nil, err
		}

		var session storage.Session
		err = json.Unmarshal(data, &session)
		if err != nil {
			return nil, nil, err
		}

		sessions = append(sessions, session)
	}

	return agents, sessions, nil
}
