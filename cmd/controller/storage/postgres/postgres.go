/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	_ "github.com/lib/pq"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
)

type storageDriver struct {
	ctx context.Context
	db  *sql.DB
}

type sqlRow interface {
	Scan(dest ...any) error
}

type unmarshalFn[T any] func(row sqlRow) (T, error)

type tableIterator[T any] struct {
	ctx context.Context

	statement *sql.Stmt
	offset    int

	unmarshal unmarshalFn[T]

	iterator storage.Iterator[T]
}

func newIterator[T any](ctx context.Context, statement *sql.Stmt, unmarshal unmarshalFn[T]) (storage.Iterator[T], error) {
	iterator := &tableIterator[T]{
		ctx: ctx,

		statement: statement,
		offset:    0,

		unmarshal: unmarshal,
	}

	objects, err := iterator.retrieveRows()
	if err != nil {
		logger.Debugf("unable to retrieve rows, %s", err.Error())
		return nil, err
	}

	iterator.iterator = storage.NewDefaultIterator[T](objects)
	return iterator, err
}

func (iterator *tableIterator[T]) retrieveRows() ([]T, error) {
	rows, err := iterator.statement.QueryContext(iterator.ctx, iterator.offset)
	if err != nil {
		return nil, err
	}

	objects := make([]T, 0)
	for rows.Next() {
		obj, err := iterator.unmarshal(rows)
		if err != nil {
			return nil, err
		}

		objects = append(objects, obj)

		iterator.offset++
	}

	return objects, nil
}

func (iterator *tableIterator[T]) Next() bool {
	if !iterator.Next() {
		objects, err := iterator.retrieveRows()
		if err != nil {
			logger.Debugf("unable to retrieve rows, %s", err.Error())
			return false
		}

		if len(objects) == 0 {
			return false
		}

		iterator.iterator = storage.NewDefaultIterator[T](objects)
	}

	return true
}

func (iterator *tableIterator[T]) Value() T {
	return iterator.Value()
}

const (
	selectAgents = `SELECT id, state, hostname, address, version, max_sessions, gpus, 
			( SELECT ARRAY (
				SELECT ( SELECT keyvalue FROM key_values WHERE id = agent_labels.key_value_id ) FROM agent_labels WHERE agent_id = agents.id
			) ) labels, 
			( SELECT ARRAY (
				SELECT ( SELECT keyvalue FROM key_values WHERE id = agent_taints.key_value_id ) FROM agent_taints WHERE agent_id = agents.id
			) ) taints, 
			( SELECT ARRAY (
				SELECT row(id, state, address, version, persistent, gpus) FROM sessions tab WHERE tab.agent_id = agents.id 
			) ) sessions
		FROM agents`
	selectSessions       = "SELECT id, state, address, version, persistent, gpus FROM sessions"
	selectQueuedSessions = "SELECT id, requirements FROM sessions WHERE state = 'queued'"

	orderBy     = " ORDER BY created_at ASC"
	offsetLimit = " OFFSET $1 LIMIT "
)

func selectAgentsWhere(where string) string {
	return fmt.Sprint(selectAgents, " WHERE ", where, orderBy)
}

func selectAgentsIterator(limit int) string {
	return fmt.Sprint(selectAgents, orderBy, offsetLimit, limit)
}

func selectAgentsIteratorWhere(where string, limit int) string {
	return fmt.Sprint(selectAgents, " WHERE ", where, orderBy, offsetLimit, limit)
}

func unmarshalAgent(row sqlRow) (restapi.Agent, error) {
	var state string
	var gpus []byte
	var labels pq.StringArray
	var taints pq.StringArray
	var sessions pq.StringArray

	agent := restapi.Agent{
		Labels:   map[string]string{},
		Taints:   map[string]string{},
		Sessions: make([]restapi.Session, 0),
	}

	err := row.Scan(&agent.Id, &state, &agent.Hostname, &agent.Address, &agent.Version, &agent.MaxSessions, &gpus, &labels, &taints, &sessions)
	if err != nil {
		return restapi.Agent{}, err
	}

	agent.State = stringToAgentState(state)

	err = json.Unmarshal(gpus, &agent.Gpus)
	if err != nil {
		return restapi.Agent{}, err
	}

	for _, label := range labels {
		err = json.Unmarshal([]byte(label), &agent.Labels)
		if err != nil {
			return restapi.Agent{}, err
		}
	}

	for _, taint := range taints {
		err = json.Unmarshal([]byte(taint), &agent.Taints)
		if err != nil {
			return restapi.Agent{}, err
		}
	}

	// TODO: sessions

	return agent, nil
}

func selectSessionsWhere(where string) string {
	return fmt.Sprint(selectSessions, " WHERE ", where, orderBy)
}

func unmarshalSession(row sqlRow) (restapi.Session, error) {
	var session restapi.Session
	var state string
	var gpus string

	err := row.Scan(&session.Id, &state, &session.Address, &session.Version, &session.Persistent, &gpus)
	if err != nil {
		return restapi.Session{}, err
	}

	err = json.Unmarshal([]byte(gpus), &session.Gpus)
	if err != nil {
		return restapi.Session{}, err
	}

	session.State = stringToSessionState(state)

	return session, nil
}

func selectQueuedSessionsWhere(where string) string {
	return fmt.Sprint(selectQueuedSessions, " AND ", where, orderBy)
}

func selectQueuedSessionsIteratorWhere(where string, limit int) string {
	return fmt.Sprint(selectQueuedSessions, " AND ", where, orderBy, offsetLimit, limit)
}

func unmarshalQueuedSession(row sqlRow) (storage.QueuedSession, error) {
	session := storage.QueuedSession{}

	var requirements string
	err := row.Scan(&session.Id, &requirements)
	if err != nil {
		return storage.QueuedSession{}, err
	}

	err = json.Unmarshal([]byte(requirements), &session.Requirements)
	if err != nil {
		return storage.QueuedSession{}, err
	}

	return session, nil
}

func stringToAgentState(str string) int {
	switch str {
	case "active":
		return restapi.AgentActive
	case "disabled":
		return restapi.AgentDisabled
	case "missing":
		return restapi.AgentMissing
	case "closed":
		return restapi.AgentClosed
	}

	logger.Panicf("unable to convert string to AgentState, %s", str)
	return -1
}

func agentStateToString(state int) string {
	switch state {
	case restapi.AgentActive:
		return "active"
	case restapi.AgentDisabled:
		return "disabled"
	case restapi.AgentMissing:
		return "missing"
	case restapi.AgentClosed:
		return "closed"
	}

	logger.Panicf("unable to convert AgentState to string, %d", state)
	return ""
}

func stringToSessionState(str string) int {
	switch str {
	case "queued":
		return restapi.SessionQueued
	case "assigned":
		return restapi.SessionAssigned
	case "active":
		return restapi.SessionActive
	case "closed":
		return restapi.SessionClosed
	case "failed":
		return restapi.SessionFailed
	case "canceling":
		return restapi.SessionCanceling
	case "canceled":
		return restapi.SessionCanceled
	}

	logger.Panicf("unable to convert string to SessionState, %s", str)
	return -1
}

func sessionStateToString(state int) string {
	switch state {
	case restapi.SessionQueued:
		return "queued"
	case restapi.SessionAssigned:
		return "assigned"
	case restapi.SessionActive:
		return "active"
	case restapi.SessionClosed:
		return "closed"
	case restapi.SessionFailed:
		return "failed"
	case restapi.SessionCanceling:
		return "canceling"
	case restapi.SessionCanceled:
		return "canceled"
	}

	logger.Panicf("unable to convert SessionState to string, %d", state)
	return ""
}

func OpenStorage(ctx context.Context, connection string) (storage.Storage, error) {
	db, err := sql.Open("postgres", connection)
	if err != nil {
		return nil, err
	}

	return &storageDriver{
		ctx: ctx,
		db:  db,
	}, nil
}

func (driver *storageDriver) addLog(tx *sql.Tx, operation string, obj any) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(driver.ctx, "INSERT INTO log ("+
		"operation, data"+
		") VALUES ("+
		"$1, $2"+
		")",
		operation, data)
	return err
}

func (driver *storageDriver) addLogData(tx *sql.Tx, operation string, data []byte) error {
	_, err := tx.ExecContext(driver.ctx, "INSERT INTO log ("+
		"operation, data"+
		") VALUES ("+
		"$1, $2"+
		")",
		operation, data)
	return err
}

func (driver *storageDriver) Close() error {
	return driver.db.Close()
}

func (driver *storageDriver) AggregateData() (storage.AggregatedData, error) {
	return storage.AggregatedData{}, nil
}

func (driver *storageDriver) RegisterAgent(agent restapi.Agent) (string, error) {
	gpus, err := json.Marshal(agent.Gpus)
	if err != nil {
		return "", err
	}

	tx, err := driver.db.BeginTx(driver.ctx, nil)
	if err != nil {
		return "", err
	}

	err = driver.addLog(tx, "RegisterAgent", agent)
	if err != nil {
		return "", errors.Join(err, tx.Rollback())
	}

	var id string
	err = driver.db.QueryRowContext(driver.ctx, "INSERT INTO agents ("+
		"state, hostname, address, version, max_sessions, gpus, vram_available, sessions_available, updated_at"+
		") VALUES ("+
		"$1, $2, $3, $4, $5, $6, $7, $8, now()"+
		") RETURNING id",
		agentStateToString(agent.State), agent.Hostname, agent.Address, agent.Version,
		agent.MaxSessions, gpus, storage.TotalVram(agent.Gpus), agent.MaxSessions).Scan(&id)
	if err != nil {
		return "", errors.Join(err, tx.Rollback())
	}

	for key, value := range agent.Labels {
		var keyValueId uint64
		err = driver.db.QueryRowContext(driver.ctx, "INSERT INTO key_values ("+
			"keyvalue"+
			") VALUES ("+
			"$1"+
			") RETURNING id", fmt.Sprintf(`{"%s":"%s"}`, key, value)).Scan(&keyValueId)
		if err != nil {
			return "", errors.Join(err, tx.Rollback())
		}

		_, err = driver.db.ExecContext(driver.ctx, "INSERT INTO agent_labels ("+
			"agent_id, key_value_id"+
			") VALUES ("+
			"$1, $2"+
			")", id, keyValueId)
		if err != nil {
			return "", errors.Join(err, tx.Rollback())
		}
	}

	for key, value := range agent.Taints {
		var keyValueId uint64
		err = driver.db.QueryRowContext(driver.ctx, "INSERT INTO key_values ("+
			"keyvalue"+
			") VALUES ("+
			"$1, $2"+
			") RETURNING id", fmt.Sprintf(`{"%s":"%s"}`, key, value)).Scan(&keyValueId)
		if err != nil {
			return "", errors.Join(err, tx.Rollback())
		}

		_, err = driver.db.ExecContext(driver.ctx, "INSERT INTO agent_taints ("+
			"agent_id, key_value_id"+
			") VALUES ("+
			"$1, $2"+
			")", id, keyValueId)
		if err != nil {
			return "", errors.Join(err, tx.Rollback())
		}
	}

	return id, tx.Commit()
}

func (driver *storageDriver) GetAgentById(id string) (restapi.Agent, error) {
	return unmarshalAgent(driver.db.QueryRowContext(driver.ctx, selectAgentsWhere("id = $1"), id))
}

func (driver *storageDriver) UpdateAgent(update restapi.AgentUpdate) error {
	var gpusData []byte
	err := driver.db.QueryRowContext(driver.ctx, "SELECT gpus FROM agents WHERE id = $1", update.Id).Scan(&gpusData)
	if err != nil {
		return err
	}

	var gpus []restapi.Gpu
	err = json.Unmarshal(gpusData, &gpus)
	if err != nil {
		return err
	}

	for index, metrics := range update.Gpus {
		gpus[index].Metrics = metrics
	}

	gpusData, err = json.Marshal(gpus)
	if err != nil {
		return err
	}

	tx, err := driver.db.BeginTx(driver.ctx, nil)
	if err != nil {
		return errors.Join(err, tx.Rollback())
	}

	err = driver.addLog(tx, "UpdateAgent", update)
	if err != nil {
		return errors.Join(err, tx.Rollback())
	}

	// Check if any of the sessions are being closed
	closedSessions := ""
	closedSessionsCount := 0
	for key, value := range update.Sessions {
		if value.State >= restapi.SessionClosed {
			closedSessions = fmt.Sprint(closedSessions, ", ", key)
			closedSessionsCount++
		}
	}

	if closedSessionsCount > 0 {
		_, err = driver.db.ExecContext(driver.ctx, `UPDATE agents SET vram_available = (
				SELECT SUM(vram_required) FROM sessions WHERE id = ANY(ARRAY[$1])
			), sessions_available = sessions_available + $2, state = $3, gpus = $4, updated_at = now() WHERE id = $5`,
			closedSessions[2:], closedSessionsCount, update.State, update.Gpus, update.Id)
	} else {
		_, err = driver.db.ExecContext(driver.ctx, "UPDATE agents SET state = $1, gpus = $2, updated_at = now() WHERE id = $3", agentStateToString(update.State), gpusData, update.Id)
	}

	if err != nil {
		return errors.Join(err, tx.Rollback())
	}

	for id, sessionUpdate := range update.Sessions {
		_, err = driver.db.ExecContext(driver.ctx, "UPDATE sessions SET state = $1 WHERE id = $2", sessionStateToString(sessionUpdate.State), id)
		if err != nil {
			return errors.Join(err, tx.Rollback())
		}
	}

	return tx.Commit()
}

func (driver *storageDriver) RequestSession(sessionRequirements restapi.SessionRequirements) (string, error) {
	requirements, err := json.Marshal(sessionRequirements)
	if err != nil {
		return "", err
	}

	tx, err := driver.db.BeginTx(driver.ctx, nil)
	if err != nil {
		return "", errors.Join(err, tx.Rollback())
	}

	err = driver.addLogData(tx, "RequestSession", requirements)
	if err != nil {
		return "", errors.Join(err, tx.Rollback())
	}

	var id string
	err = driver.db.QueryRowContext(driver.ctx, "INSERT INTO sessions ("+
		"state, version, persistent, requirements, vram_required, updated_at"+
		") VALUES ("+
		"$1, $2, $3, $4, $5, now()"+
		") RETURNING id",
		sessionStateToString(restapi.SessionQueued), sessionRequirements.Version,
		sessionRequirements.Persistent, storage.TotalVramRequired(sessionRequirements), requirements).Scan(&id)
	if err != nil {
		return "", errors.Join(err, tx.Rollback())
	}

	for key, value := range sessionRequirements.MatchLabels {
		var keyValueId uint64
		err = driver.db.QueryRowContext(driver.ctx, "INSERT INTO key_values ("+
			"keyvalue"+
			") VALUES ("+
			"$1, $2"+
			") RETURNING id", fmt.Sprintf(`{"%s":"%s"}`, key, value)).Scan(&keyValueId)
		if err != nil {
			return "", errors.Join(err, tx.Rollback())
		}

		_, err = driver.db.ExecContext(driver.ctx, "INSERT INTO session_match_labels ("+
			"agent_id, key_value_id"+
			") VALUES ("+
			"$1, $2"+
			") RETURNING id", id, keyValueId)
		if err != nil {
			return "", errors.Join(err, tx.Rollback())
		}
	}

	for key, value := range sessionRequirements.Tolerates {
		var keyValueId uint64
		err = driver.db.QueryRowContext(driver.ctx, "INSERT INTO key_values ("+
			"keyvalue"+
			") VALUES ("+
			"$1, $2"+
			") RETURNING id", fmt.Sprintf(`{"%s":"%s"}`, key, value)).Scan(&keyValueId)
		if err != nil {
			return "", errors.Join(err, tx.Rollback())
		}

		_, err = driver.db.ExecContext(driver.ctx, "INSERT INTO session_tolerates ("+
			"agent_id, key_value_id"+
			") VALUES ("+
			"$1, $2"+
			") RETURNING id", id, keyValueId)
		if err != nil {
			return "", errors.Join(err, tx.Rollback())
		}
	}

	return id, tx.Commit()
}

func (driver *storageDriver) AssignSession(sessionId string, agentId string, gpus []restapi.SessionGpu) error {
	gpusData, err := json.Marshal(gpus)
	if err != nil {
		return err
	}

	dataMap := map[string]string{
		"SessionId": sessionId,
		"AgentId":   agentId,
		"Gpus":      string(gpusData),
	}
	data, err := json.Marshal(dataMap)
	if err != nil {
		return err
	}

	tx, err := driver.db.BeginTx(driver.ctx, nil)
	if err != nil {
		return errors.Join(err, tx.Rollback())
	}

	err = driver.addLogData(tx, "AssignSession", data)
	if err != nil {
		return errors.Join(err, tx.Rollback())
	}

	_, err = driver.db.ExecContext(driver.ctx, `UPDATE agents SET vram_available = vram_available - (
			SELECT vram_required FROM sessions WHERE id = $1
		), sessions_available = sessions_available - 1, updated_at = now() WHERE id = $2`, sessionId, agentId)
	if err != nil {
		return errors.Join(err, tx.Rollback())
	}

	_, err = driver.db.ExecContext(driver.ctx, `UPDATE sessions SET agent_id = $1, state = 'assigned', address = (
			SELECT address FROM agents WHERE id = $1
		), gpus = $2, updated_at = now() WHERE id = $3`, agentId, gpusData, agentId)
	if err != nil {
		return errors.Join(err, tx.Rollback())
	}

	return tx.Commit()
}

func (driver *storageDriver) GetSessionById(id string) (restapi.Session, error) {
	return unmarshalSession(driver.db.QueryRowContext(driver.ctx, selectSessionsWhere("id = $1"), id))
}

func (driver *storageDriver) GetQueuedSessionById(id string) (storage.QueuedSession, error) {
	return unmarshalQueuedSession(driver.db.QueryRowContext(driver.ctx, selectQueuedSessionsWhere("id = $1"), id))
}

func (driver *storageDriver) GetAgents() (storage.Iterator[restapi.Agent], error) {
	statement, err := driver.db.PrepareContext(driver.ctx, selectAgentsIterator(20))
	if err != nil {
		return nil, err
	}

	return newIterator(driver.ctx, statement, unmarshalAgent)
}

func (driver *storageDriver) GetAvailableAgentsMatching(totalAvailableVramAtLeast uint64) (storage.Iterator[restapi.Agent], error) {
	statement, err := driver.db.PrepareContext(driver.ctx, selectAgentsIteratorWhere(
		fmt.Sprint("state = 'active' AND vram_available >= ", totalAvailableVramAtLeast, " AND sessions_available > 0"), 20))
	if err != nil {
		return nil, err
	}

	return newIterator(driver.ctx, statement, unmarshalAgent)
}

func (driver *storageDriver) GetQueuedSessionsIterator() (storage.Iterator[storage.QueuedSession], error) {
	statement, err := driver.db.PrepareContext(driver.ctx, selectQueuedSessionsIteratorWhere("state = 'queued'", 20))
	if err != nil {
		return nil, err
	}

	return newIterator(driver.ctx, statement, unmarshalQueuedSession)
}

func (driver *storageDriver) SetAgentsMissingIfNotUpdatedFor(duration time.Duration) error {
	_, err := driver.db.ExecContext(driver.ctx, "UPDATE agents SET state = 'missing', updated_at = now() WHERE state = 'active' AND updated_at <= now()-make_interval(secs=>$1)", duration.Seconds())
	return err
}

func (driver *storageDriver) RemoveMissingAgentsIfNotUpdatedFor(duration time.Duration) error {
	_, err := driver.db.ExecContext(driver.ctx, "DELETE FROM agents WHERE state = 'missing' AND updated_at <= now()-make_interval(secs=>$1)", duration.Seconds())
	return err
}
