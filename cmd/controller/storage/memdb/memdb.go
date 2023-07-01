/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package memdb

import (
	"context"
	"encoding/binary"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-memdb"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/utilities"
)

// TimeFieldIndex is used to extract a time field from an object using
// reflection and builds an index on that field.
type TimeFieldIndex struct {
	Field string
}

func (i *TimeFieldIndex) FromObject(obj interface{}) (bool, []byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Dereference the pointer if any

	fv := v.FieldByName(i.Field)
	if !fv.IsValid() {
		return false, nil,
			fmt.Errorf("field '%s' for %#v is invalid", i.Field, obj)
	}

	// Check the type
	if fv.Type() != reflect.TypeOf(time.Time{}) {
		return false, nil, fmt.Errorf("field %q is of type %v; want time.Time", i.Field, fv.Type())
	}

	// Get the value and encode it
	val := utilities.Require[time.Time](fv.Interface())
	buf := encodeInt(val.Unix(), 8)

	return true, buf, nil
}

func (i *TimeFieldIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}

	v := reflect.ValueOf(args[0])
	if !v.IsValid() {
		return nil, fmt.Errorf("%#v is invalid", args[0])
	}

	// Check the type
	if v.Type() != reflect.TypeOf(time.Time{}) {
		return nil, fmt.Errorf("field %q is of type %v; want time.Time", i.Field, v.Type())
	}

	// Get the value and encode it
	val := utilities.Require[time.Time](v.Interface())
	buf := encodeInt(val.Unix(), 8)

	return buf, nil
}

func encodeInt(val int64, size int) []byte {
	buf := make([]byte, size)

	// This bit flips the sign bit on any sized signed twos-complement integer,
	// which when truncated to a uint of the same size will bias the value such
	// that the maximum negative int becomes 0, and the maximum positive int
	// becomes the maximum positive uint.
	scaled := val ^ int64(-1<<(size*8-1))

	switch size {
	case 1:
		buf[0] = uint8(scaled)
	case 2:
		binary.BigEndian.PutUint16(buf, uint16(scaled))
	case 4:
		binary.BigEndian.PutUint32(buf, uint32(scaled))
	case 8:
		binary.BigEndian.PutUint64(buf, uint64(scaled))
	default:
		panic(fmt.Sprintf("unsupported int size parameter: %d", size))
	}

	return buf
}

type storageDriver struct {
	ctx context.Context
	db  *memdb.MemDB
}

func OpenStorage(ctx context.Context) (storage.Storage, error) {
	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"agents": &memdb.TableSchema{
				Name: "agents",
				Indexes: map[string]*memdb.IndexSchema{
					"id": &memdb.IndexSchema{
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.UUIDFieldIndex{Field: "Id"},
					},
					"state": &memdb.IndexSchema{
						Name:    "state",
						Unique:  false,
						Indexer: &memdb.IntFieldIndex{Field: "State"},
					},
					"last_updated": &memdb.IndexSchema{
						Name:    "last_updated",
						Unique:  false,
						Indexer: &TimeFieldIndex{Field: "LastUpdated"},
					},
				},
			},
			"sessions": &memdb.TableSchema{
				Name: "sessions",
				Indexes: map[string]*memdb.IndexSchema{
					"id": &memdb.IndexSchema{
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.UUIDFieldIndex{Field: "Id"},
					},
					"state": &memdb.IndexSchema{
						Name:    "state",
						Unique:  false,
						Indexer: &memdb.IntFieldIndex{Field: "State"},
					},
					"last_updated": &memdb.IndexSchema{
						Name:    "last_updated",
						Unique:  false,
						Indexer: &TimeFieldIndex{Field: "LastUpdated"},
					},
				},
			},
		},
	}

	db, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, err
	}

	return &storageDriver{
		ctx: ctx,
		db:  db,
	}, nil
}

func (driver *storageDriver) Close() error {
	return nil
}

func (driver *storageDriver) AddAgent(agent storage.Agent) (string, error) {
	agent.Id = uuid.NewString()

	txn := driver.db.Txn(true)
	err := txn.Insert("agents", agent)
	if err != nil {
		txn.Abort()
		return "", nil
	}

	txn.Commit()
	return agent.Id, nil
}

func (driver *storageDriver) AddSession(session storage.Session) (string, error) {
	session.Id = uuid.NewString()

	txn := driver.db.Txn(true)
	err := txn.Insert("sessions", session)
	if err != nil {
		txn.Abort()
		return "", nil
	}

	txn.Commit()
	return session.Id, nil
}

func (driver *storageDriver) GetActiveAgents() ([]storage.Agent, error) {
	txn := driver.db.Txn(false)
	defer txn.Abort()

	iterator, err := txn.Get("agents", "state", restapi.StateActive)
	if err != nil {
		return nil, err
	}

	agents := make([]storage.Agent, 0)
	for obj := iterator.Next(); obj != nil; obj = iterator.Next() {
		agents = append(agents, utilities.Require[storage.Agent](obj))
	}

	return agents, nil
}

func (driver *storageDriver) UpdateAgentsAndSessions(agents []storage.Agent, sessions []storage.Session) error {
	if len(agents) > 0 || len(sessions) > 0 {
		txn := driver.db.Txn(true)

		for _, agent := range agents {
			err := txn.Insert("agents", agent)
			if err != nil {
				txn.Abort()
				return nil
			}
		}

		for _, session := range sessions {
			err := txn.Insert("sessions", session)
			if err != nil {
				txn.Abort()
				return nil
			}
		}

		txn.Commit()
	}

	return nil
}

func (driver *storageDriver) GetSessionById(id string) (storage.Session, error) {
	txn := driver.db.Txn(false)
	defer txn.Abort()

	obj, err := txn.First("sessions", "id", id)
	if err != nil {
		return storage.Session{}, err
	}

	return utilities.Require[storage.Session](obj), nil
}

func (driver *storageDriver) GetAgentsAndSessionsUpdatedSince(time time.Time) ([]storage.Agent, []storage.Session, error) {
	txn := driver.db.Txn(false)
	defer txn.Abort()

	iterator, err := txn.Get("agents", "last_updated", time)
	if err != nil {
		return nil, nil, err
	}

	agents := make([]storage.Agent, 0)
	for obj := iterator.Next(); obj != nil; obj = iterator.Next() {
		agents = append(agents, utilities.Require[storage.Agent](obj))
	}

	iterator, err = txn.Get("sessions", "last_updated", time)
	if err != nil {
		return nil, nil, err
	}

	sessions := make([]storage.Session, 0)
	for obj := iterator.Next(); obj != nil; obj = iterator.Next() {
		sessions = append(sessions, utilities.Require[storage.Session](obj))
	}

	return agents, sessions, nil
}
