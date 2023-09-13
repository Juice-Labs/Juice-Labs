package gorm

import (
	"context"
	"database/sql"
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
		PoolId:   dbAgent.PoolID.String(),
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
			Id:      dbSession.UUID.String(),
			State:   dbSession.State.String(),
			Address: dbSession.Address,
			Version: dbSession.Version,
		}

		if err := json.Unmarshal(dbSession.GPUs, &session.Gpus); err != nil {
			continue
		}

		for _, dbConnection := range dbSession.Connections {
			session.Connections = append(session.Connections, restapi.Connection{
				ConnectionData: restapi.ConnectionData{
					Id:          dbConnection.UUID.String(),
					Pid:         dbConnection.Pid,
					ProcessName: dbConnection.ProcessName,
				},
				ExitCode: dbConnection.ExitCode,
			})
		}

		agent.Sessions = append(agent.Sessions, session)
	}

	return agent, nil
}

func restSessionFromSession(dbSession models.Session) (restapi.Session, error) {
	session := restapi.Session{
		Id:      dbSession.UUID.String(),
		State:   dbSession.State.String(),
		Address: dbSession.Address,
		Version: dbSession.Version,
	}
	if dbSession.PoolID.Valid {
		session.PoolId = dbSession.PoolID.UUID.String()
	}

	if err := json.Unmarshal(dbSession.GPUs, &session.Gpus); err != nil {
		return restapi.Session{}, nil
	}

	for _, dbConnection := range dbSession.Connections {
		session.Connections = append(session.Connections, restapi.Connection{
			ConnectionData: restapi.ConnectionData{
				Id:          dbConnection.UUID.String(),
				Pid:         dbConnection.Pid,
				ProcessName: dbConnection.ProcessName,
			},
			ExitCode: dbConnection.ExitCode,
		})
	}
	return session, nil
}

func restPoolFromPool(dbPool models.Pool) restapi.Pool {
	pool := restapi.Pool{
		Id:   dbPool.ID.String(),
		Name: dbPool.PoolName,
	}

	return pool
}

func dbPermissionTypeToRestPermissionType(dbPermissionType models.PermissionType) (restapi.Permission, error) {
	switch dbPermissionType {
	case models.CreateSession:
		return restapi.PermissionCreateSession, nil
	case models.RegisterAgent:
		return restapi.PermissionRegisterAgent, nil
	case models.Admin:
		return restapi.PermissionAdmin, nil
	default:
		return "", fmt.Errorf("unknown permission type")
	}
}

func restPermissionTypeToDbPermissionType(permission restapi.Permission) (models.PermissionType, error) {
	switch permission {
	case restapi.PermissionCreateSession:
		return models.CreateSession, nil
	case restapi.PermissionRegisterAgent:
		return models.RegisterAgent, nil
	case restapi.PermissionAdmin:
		return models.Admin, nil
	default:
		return -1, fmt.Errorf("unknown permission type")
	}
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
		&models.Permission{},
		&models.Pool{},
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
		PoolID:        uuid.FromStringOrNil(agent.PoolId),

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

	result := g.db.Preload("Labels").Preload("Taints").Preload("Sessions", "state NOT IN (?)", models.SessionStateClosed).Where(&dbAgent, "UUID").First(&dbAgent)

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

			state := models.SessionStateFromString(sessionUpdate.State)
			if state != dbSession.State {
				if dbSession.State == models.SessionStateClosed {
					dbAgent.VramAvailable += dbSession.VramRequired
				}
				dbSession.State = state
			}

			for _, connectionUpdate := range sessionUpdate.Connections {
				var dbConnection models.Connection
				tx.Where(models.Connection{UUID: uuid.FromStringOrNil(connectionUpdate.Id)}).
					Assign(models.Connection{
						SessionID:   dbSession.ID,
						Pid:         connectionUpdate.Pid,
						ProcessName: connectionUpdate.ProcessName,
						ExitCode:    connectionUpdate.ExitCode,
					}).
					FirstOrCreate(&dbConnection)

				tx.Updates(dbConnection)
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

		if sessionRequirements.PoolId != "" {
			dbSession.PoolID = uuid.NullUUID{
				UUID:  uuid.FromStringOrNil(sessionRequirements.PoolId),
				Valid: true,
			}
		} else {
			dbSession.PoolID = uuid.NullUUID{
				UUID:  uuid.Nil,
				Valid: false,
			}
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

func (g *gormDriver) GetAgentForSession(id string) (restapi.Agent, error) {

	dbSession := models.Session{
		UUID: uuid.FromStringOrNil(id),
	}

	dbAgent := models.Agent{}

	result := g.db.Preload("Labels").Preload("Taints").Joins("Agents", g.db.Where(&dbSession, "UUID")).Take(&dbAgent)

	//result := g.db.Joins("Sessions").Joins("Agents").Where(&dbSession, "UUID").First(&dbAgent)

	if result.Error != nil {
		return restapi.Agent{}, mapError(result.Error)
	}

	return restAgentFromAgent(dbAgent)
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

func (g *gormDriver) GetAgents(poolId string) (storage.Iterator[restapi.Agent], error) {
	// TODO pagination should be passed in through storage interface
	var dbAgents []models.Agent
	query := g.db.Model(&models.Agent{}).
		Preload("Labels").Preload("Taints").
		Where("state = ?", models.AgentStateActive).
		Limit(20)

	if poolId != "" {
		query = query.Where("pool_id = ?", poolId)
	}

	result := query.Find(&dbAgents)

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
	// Soft-deletes agent
	result := g.db.
		Where("state = ?", models.AgentStateMissing).
		Where("updated_at <= ?", time.Now().Add(-duration)).
		Delete(&models.Agent{})
	return mapError(result.Error)
}

func (g *gormDriver) DeletePool(id string) error {
	result := g.db.Where("id = ?", id).Delete(&models.Pool{})
	return mapError(result.Error)
}

func (g *gormDriver) GetPool(id string) (restapi.Pool, error) {
	dbPool := models.Pool{}

	result := g.db.Where(&dbPool, "ID").First(&dbPool)
	if result.Error != nil {
		return restapi.Pool{}, mapError(result.Error)
	}

	return restPoolFromPool(dbPool), nil
}

func (g *gormDriver) CreatePool(name string) (restapi.Pool, error) {
	dbPool := models.Pool{
		ID:       uuid.NewV4(),
		PoolName: name,
	}

	result := g.db.Create(&dbPool)
	if result.Error != nil {
		return restapi.Pool{}, mapError(result.Error)
	}

	return restPoolFromPool(dbPool), nil
}

func (g *gormDriver) AddPermission(poolId string, userId string, permission restapi.Permission) error {
	dbPermission := models.Permission{
		ID:         uuid.NewV4(),
		UserID:     userId,
		PoolID:     uuid.FromStringOrNil(poolId),
		Permission: models.PermissionTypeFromString(string(permission)),
	}

	result := g.db.Create(&dbPermission)

	return mapError(result.Error)
}

func (g *gormDriver) RemovePermission(poolId string, userId string, permission restapi.Permission) error {
	result := g.db.Where("pool_id = ?", poolId).Where("user_id = ?", userId).Delete(&models.Permission{})

	return mapError(result.Error)

}

type UserPermissionRow struct {
	PoolId       string
	Permission   models.PermissionType
	PoolName     string
	SessionCount int
	AgentCount   int
	UserCount    int
}

func (g *gormDriver) GetPermissions(userId string) (restapi.UserPermissions, error) {
	var result restapi.UserPermissions

	// Raw SQL because GORM doesn't support multiple counts and complex subqueries
	rows, err := g.db.Raw(`
		SELECT permissions.pool_id, permissions.permission, pools.pool_name, COUNT(DISTINCT sessions.id) AS session_count, COUNT(DISTINCT agents.id) AS agent_count, 
			(SELECT COUNT(DISTINCT p.user_id) FROM permissions p WHERE p.pool_id = permissions.pool_id AND deleted_at IS NULL) as user_count
	
		FROM permissions 
			JOIN pools ON pools.id = permissions.pool_id
			LEFT JOIN agents ON agents.pool_id = pools.id AND agents.state = @agentState
			LEFT JOIN sessions ON sessions.agent_id = agents.id AND sessions.state = @sessionState
		WHERE user_id = @userId AND permissions.deleted_at IS NULL
		GROUP BY permissions.pool_id, permissions.permission, pools.pool_name`,
		sql.Named("userId", userId), sql.Named("sessionState", models.SessionStateActive), sql.Named("agentState", models.AgentStateActive)).Rows()

	if err != nil {
		return restapi.UserPermissions{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var row UserPermissionRow
		err := rows.Scan(&row.PoolId, &row.Permission, &row.PoolName, &row.SessionCount, &row.AgentCount, &row.UserCount)
		if err != nil {
			return restapi.UserPermissions{}, err
		}
		permissionType, err := dbPermissionTypeToRestPermissionType(row.Permission)
		if err != nil {
			return restapi.UserPermissions{}, err
		}
		if result.Permissions == nil {
			result.Permissions = make(map[restapi.Permission][]restapi.Pool)
		}
		if result.Permissions[permissionType] == nil {
			result.Permissions[permissionType] = []restapi.Pool{}
		}
		result.Permissions[permissionType] = append(result.Permissions[permissionType], restapi.Pool{
			Id:           row.PoolId,
			Name:         row.PoolName,
			SessionCount: row.SessionCount,
			AgentCount:   row.AgentCount,
			UserCount:    row.UserCount,
		})
	}

	return result, nil
}

func (g *gormDriver) GetPoolPermissions(id string) (restapi.PoolPermissions, error) {
	var dbPermissions []models.Permission
	result := g.db.Where("pool_id = ?", id).Find(&dbPermissions)

	if result.Error != nil {
		return restapi.PoolPermissions{}, mapError(result.Error)
	}

	var permissions restapi.PoolPermissions
	for _, dbPermission := range dbPermissions {
		permission, err := dbPermissionTypeToRestPermissionType(dbPermission.Permission)
		if err != nil {
			return restapi.PoolPermissions{}, err
		}
		if permissions.UserIds == nil {
			permissions.UserIds = make(map[string][]restapi.Permission)
		}
		if permissions.UserIds[dbPermission.UserID] == nil {
			permissions.UserIds[dbPermission.UserID] = []restapi.Permission{}
		}
		permissions.UserIds[dbPermission.UserID] = append(permissions.UserIds[dbPermission.UserID], permission)
	}
	return permissions, nil

}
