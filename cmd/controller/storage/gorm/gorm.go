package gorm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage"
	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage/gorm/models"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	uuid "github.com/satori/go.uuid"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	glogger "gorm.io/gorm/logger"
)

type gormDriver struct {
	db *gorm.DB
}

func mapError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = errors.Join(storage.ErrNotFound, err)
	}
	return err
}

func restAgentFromAgent(dbAgent models.Agent) (restapi.Agent, error) {
	agent := restapi.Agent{
		Id:       dbAgent.UUID.String(),
		State:    dbAgent.State.String(),
		Hostname: dbAgent.Hostname,
		Address:  dbAgent.Address,
		Version:  dbAgent.Version,
		Labels:   make(map[string]string),
		Taints:   make(map[string]string),
		Sessions: []restapi.Session{},
	}

	for _, taint := range dbAgent.Taints {
		agent.Taints[taint.Key] = taint.Value
	}

	for _, label := range dbAgent.Labels {
		agent.Labels[label.Key] = label.Value
	}

	if err := json.Unmarshal(dbAgent.Gpus, &agent.Gpus); err != nil {
		return restapi.Agent{}, err
	}

	for _, dbSession := range dbAgent.Sessions {
		session := restapi.Session{
			Id:         dbSession.UUID.String(),
			State:      dbSession.State.String(),
			Address:    dbSession.Address,
			Version:    dbSession.Version,
			Persistent: dbSession.Persistent,
		}

		if err := json.Unmarshal(dbSession.GPUs, &session.Gpus); err != nil {
			continue
		}

		for _, dbConnection := range dbSession.Connections {
			session.Connections = append(session.Connections, restapi.Connection{
				Id:          dbConnection.UUID.String(),
				ExitStatus:  dbConnection.ExitStatus.String(),
				Pid:         int64(dbConnection.Pid),
				ProcessName: dbConnection.ProcessName,
			})
		}

		agent.Sessions = append(agent.Sessions, session)
	}

	return agent, nil
}

func restSessionFromSession(dbSession models.Session) (restapi.Session, error) {
	session := restapi.Session{
		Id:         dbSession.UUID.String(),
		State:      dbSession.State.String(),
		Address:    dbSession.Address,
		Version:    dbSession.Version,
		Persistent: dbSession.Persistent,
	}

	if err := json.Unmarshal(dbSession.GPUs, &session.Gpus); err != nil {
		return restapi.Session{}, nil
	}

	for _, dbConnection := range dbSession.Connections {
		session.Connections = append(session.Connections, restapi.Connection{
			Id:          dbConnection.UUID.String(),
			ExitStatus:  dbConnection.ExitStatus.String(),
			Pid:         int64(dbConnection.Pid),
			ProcessName: dbConnection.ProcessName,
		})
	}
	return session, nil
}

func OpenStorage(ctx context.Context, driver string, dsn string) (storage.Storage, error) {

	var db *gorm.DB
	var err error

	config := &gorm.Config{
		Logger: NewLogger(glogger.Config{
			LogLevel: glogger.Warn,
		}),
	}

	switch driver {
	case "sqlite":
		db, err = gorm.Open(sqlite.Open(dsn), config)
	case "postgres":
		db, err = gorm.Open(postgres.Open(dsn), config)
	default:
		err = fmt.Errorf("invalid GORM driver specified, %s", driver)
	}

	if err != nil {
		return nil, mapError(err)
	}

	err = db.AutoMigrate(
		&models.Connection{},
		&models.Session{},
		&models.KeyValue{},
		&models.Agent{},
	)

	if err != nil {
		return nil, mapError(err)
	}

	return &gormDriver{
		db: db,
	}, err
}

func (g *gormDriver) Close() error {
	return nil
}

func (g *gormDriver) AggregateData() (storage.AggregatedData, error) {
	panic("not implemented") // TODO: Implement
}

func (g *gormDriver) RegisterAgent(agent restapi.Agent) (string, error) {
	gpus, err := json.Marshal(agent.Gpus)

	if err != nil {
		return "", err
	}

	labels := []models.KeyValue{}
	for k, v := range agent.Labels {
		labels = append(labels, models.KeyValue{Key: k, Value: v})
	}

	taints := []models.KeyValue{}
	for k, v := range agent.Taints {
		taints = append(taints, models.KeyValue{Key: k, Value: v})
	}

	dbAgent := models.Agent{
		UUID:          uuid.NewV4(),
		State:         models.AgentStateFromString(agent.State),
		Hostname:      agent.Hostname,
		Address:       agent.Address,
		Version:       agent.Version,
		Gpus:          gpus,
		VramAvailable: storage.TotalVram(agent.Gpus),

		Labels: labels,
		Taints: taints,
	}

	if err = g.db.Create(&dbAgent).Error; err != nil {
		return "", mapError(err)
	}

	return dbAgent.UUID.String(), nil
}

func (g *gormDriver) GetAgentById(id string) (restapi.Agent, error) {

	dbAgent := models.Agent{
		UUID: uuid.FromStringOrNil(id),
	}

	result := g.db.Preload("Labels").Preload("Taints").Preload("Sessions").Where(&dbAgent, "UUID").First(&dbAgent)

	if err := result.Error; err != nil {

		return restapi.Agent{}, mapError(err)
	}

	return restAgentFromAgent(dbAgent)
}

func (g *gormDriver) UpdateAgent(update restapi.AgentUpdate) error {
	err := g.db.Transaction(func(tx *gorm.DB) error {
		var err error
		dbAgent := models.Agent{
			UUID: uuid.FromStringOrNil(update.Id),
		}

		result := tx.Where(&dbAgent, "UUID").First(&dbAgent)
		if result.Error != nil {
			return result.Error
		}

		dbAgent.State = models.AgentStateFromString(update.State)

		// Update GPU metrics

		var gpus []restapi.Gpu
		err = json.Unmarshal(dbAgent.Gpus, &gpus)
		if err != nil {
			return err
		}

		for index, metrics := range update.Gpus {
			gpus[index].Metrics = metrics
		}

		dbAgent.Gpus, err = json.Marshal(gpus)
		if err != nil {
			return err
		}

		// Update Sessions

		for id, sessionUpdate := range update.SessionsUpdate {
			dbSession := models.Session{
				UUID: uuid.FromStringOrNil(id),
			}

			result = tx.Where(&dbSession, "UUID").First(&dbSession)
			if result.Error != nil {
				return mapError(result.Error)
			}

			dbSession.State = models.SessionStateFromString(sessionUpdate.State)

			for _, connectionUpdate := range sessionUpdate.Connections {
				dbConnection := models.Connection{
					UUID:        uuid.FromStringOrNil(connectionUpdate.Id),
					Session:     dbSession,
					Pid:         uint64(connectionUpdate.Pid),
					ProcessName: connectionUpdate.ProcessName,
					ExitStatus:  models.ExitStatusFromString(connectionUpdate.ExitStatus),
				}
				dbSession.Connections = append(dbSession.Connections, dbConnection)
			}

			tx.Updates(dbSession)
		}

		tx.Updates(dbAgent)

		return nil
	})

	return mapError(err)
}

func (g *gormDriver) RequestSession(sessionRequirements restapi.SessionRequirements) (string, error) {

	var dbSession *models.Session
	err := g.db.Transaction(func(tx *gorm.DB) error {
		requirements, err := json.Marshal(sessionRequirements)
		if err != nil {
			return err
		}

		labels := []models.KeyValue{}
		for k, v := range sessionRequirements.MatchLabels {
			labels = append(labels, models.KeyValue{Key: k, Value: v})
		}

		tolerates := []models.KeyValue{}
		for k, v := range sessionRequirements.Tolerates {
			tolerates = append(tolerates, models.KeyValue{Key: k, Value: v})
		}

		dbSession = &models.Session{
			UUID:         uuid.NewV4(),
			Agent:        nil,
			Version:      sessionRequirements.Version,
			State:        models.SessionStateQueued,
			Requirements: requirements,
			VramRequired: storage.TotalVramRequired(sessionRequirements),

			Labels:    labels,
			Tolerates: tolerates,
		}

		tx.Create(dbSession)

		return nil
	})

	if dbSession != nil {
		return dbSession.UUID.String(), nil
	}

	return "", mapError(err)
}

func (g *gormDriver) AssignSession(sessionId string, agentId string, gpus []restapi.SessionGpu) error {
	err := g.db.Transaction(func(tx *gorm.DB) error {

		gpusData, err := json.Marshal(gpus)
		if err != nil {
			return err
		}

		dbAgent := models.Agent{
			UUID: uuid.FromStringOrNil(agentId),
		}

		dbSession := models.Session{
			UUID: uuid.FromStringOrNil(sessionId),
		}

		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(&dbAgent, "UUID").First(&dbAgent)
		if result.Error != nil {
			return result.Error
		}

		result = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(&dbSession, "UUID").First(&dbSession)
		if result.Error != nil {
			return result.Error
		}

		dbSession.GPUs = gpusData
		dbSession.Agent = &dbAgent
		// TODO why?
		dbSession.Address = dbAgent.Address
		dbSession.State = models.SessionStateAssigned
		dbAgent.VramAvailable -= dbSession.VramRequired

		tx.Updates(&dbSession)
		tx.Updates(&dbAgent)

		return nil
	})

	return mapError(err)
}

func (g *gormDriver) CancelSession(sessionId string) error {
	err := g.db.Transaction(func(tx *gorm.DB) error {
		dbSession := models.Session{
			UUID: uuid.FromStringOrNil(sessionId),
		}

		result := tx.Preload("Agent").Clauses(clause.Locking{Strength: "UPDATE"}).Where(&dbSession, "UUID").First(&dbSession)
		if result.Error != nil {
			return result.Error
		}

		if dbSession.Agent == nil {
			dbSession.State = models.SessionStateClosed
		} else {
			dbSession.State = models.SessionStateCanceling
		}

		tx.Updates(&dbSession)

		return nil
	})

	return mapError(err)
}

func (g *gormDriver) GetSessionById(id string) (restapi.Session, error) {
	dbSession := models.Session{
		UUID: uuid.FromStringOrNil(id),
	}

	result := g.db.Preload("Connections").Where(&dbSession, "UUID").First(&dbSession)
	if result.Error != nil {
		return restapi.Session{}, mapError(result.Error)
	}

	return restSessionFromSession(dbSession)
}

func (g *gormDriver) GetQueuedSessionById(id string) (storage.QueuedSession, error) {
	dbSession := models.Session{
		UUID:  uuid.FromStringOrNil(id),
		State: models.SessionStateQueued,
	}

	result := g.db.Model(&models.Session{}).Where(&dbSession, "UUID", "State").First(&dbSession)
	if result.Error != nil {
		return storage.QueuedSession{}, mapError(result.Error)
	}

	queuedSession := storage.QueuedSession{
		Id: dbSession.UUID.String(),
	}

	err := json.Unmarshal(dbSession.Requirements, &queuedSession.Requirements)
	if err != nil {
		return storage.QueuedSession{}, err
	}

	return queuedSession, nil
}

func (g *gormDriver) GetAgents() (storage.Iterator[restapi.Agent], error) {
	// TODO pagination should be passed in through storage interface
	var dbAgents []models.Agent
	result := g.db.Model(&models.Agent{}).
		Preload("Labels").Preload("Taints").
		Where("state = ?", models.AgentStateActive).
		Limit(20).
		Find(&dbAgents)

	if result.Error != nil {
		return nil, mapError(result.Error)
	}

	agents := []restapi.Agent{}
	for _, dbAgent := range dbAgents {
		agent, err := restAgentFromAgent(dbAgent)

		if err != nil {
			logger.Warning(err)
		} else {
			agents = append(agents, agent)
		}
	}

	return storage.NewDefaultIterator[restapi.Agent](agents), nil
}

func (g *gormDriver) GetAvailableAgentsMatching(totalAvailableVramAtLeast uint64) (storage.Iterator[restapi.Agent], error) {
	// TODO pagination should be passed in through storage interface
	var dbAgents []models.Agent
	result := g.db.Model(&models.Agent{}).
		Preload("Labels").Preload("Taints").
		Where("state = ?", models.AgentStateActive).
		Where("vram_available >= ?", totalAvailableVramAtLeast).
		Limit(20).
		Find(&dbAgents)

	if result.Error != nil {
		return nil, mapError(result.Error)
	}

	agents := []restapi.Agent{}
	for _, dbAgent := range dbAgents {
		agent, err := restAgentFromAgent(dbAgent)
		if err != nil {
			logger.Warning(err)
		} else {
			agents = append(agents, agent)
		}
	}

	return storage.NewDefaultIterator[restapi.Agent](agents), nil

}

func (g *gormDriver) GetQueuedSessionsIterator() (storage.Iterator[storage.QueuedSession], error) {
	// TODO pagination should be passed in through storage interface
	var dbSessions []models.Session
	result := g.db.Model(&models.Session{}).
		Where("state = ?", models.SessionStateQueued).
		Limit(20).
		Find(&dbSessions)
	if result.Error != nil {
		return nil, mapError(result.Error)
	}

	queuedSessions := []storage.QueuedSession{}
	for _, dbSession := range dbSessions {
		queuedSession := storage.QueuedSession{
			Id: dbSession.UUID.String(),
		}

		if err := json.Unmarshal(dbSession.Requirements, &queuedSession.Requirements); err != nil {
			logger.Error(err)
			continue
		}

		queuedSessions = append(queuedSessions, queuedSession)
	}

	return storage.NewDefaultIterator[storage.QueuedSession](queuedSessions), nil

}

func (g *gormDriver) SetAgentsMissingIfNotUpdatedFor(duration time.Duration) error {

	result := g.db.Model(&models.Agent{}).
		Where("state = ?", models.AgentStateActive).
		Where("updated_at <= ?", time.Now().Add(-duration)).
		Updates(models.Agent{State: models.AgentStateMissing})
	return mapError(result.Error)
}

func (g *gormDriver) RemoveMissingAgentsIfNotUpdatedFor(duration time.Duration) error {
	// TODO soft-delete maybe?
	result := g.db.Unscoped().
		Where("state = ?", models.AgentStateMissing).
		Where("updated_at <= ?", time.Now().Add(-duration)).
		Delete(&models.Agent{})
	return mapError(result.Error)
}
