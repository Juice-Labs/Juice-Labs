/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package backend

import (
	"context"
	"errors"
	"time"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage"
	"github.com/Juice-Labs/Juice-Labs/pkg/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

type Backend struct {
	storage storage.Storage
}

func NewBackend(storage storage.Storage) *Backend {
	return &Backend{
		storage: storage,
	}
}

func (backend *Backend) Run(group task.Group) error {
	err := backend.update(group.Ctx())
	if err == nil {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for err == nil {
			select {
			case <-group.Ctx().Done():
				return err

			case <-ticker.C:
				err = backend.update(group.Ctx())
			}
		}
	}

	return err
}

func isSubset(set, subset map[string]string) bool {
	for key, value := range subset {
		checkValue, present := set[key]
		if !present || value != checkValue {
			return false
		}
	}

	return true
}

func matchesLabels(set, subset map[string]string) bool {
	return isSubset(set, subset)
}

func canTolerate(taints, tolerates map[string]string) bool {
	// tolerates must be a superset of taints to be acceptable
	return isSubset(tolerates, taints)
}

func agentMatches(agent restapi.Agent, requirements restapi.SessionRequirements) (*gpu.SelectedGpuSet, error) {
	if matchesLabels(agent.Labels, requirements.MatchLabels) && canTolerate(agent.Taints, requirements.Tolerates) {
		var err error

		// Need to ensure the agent has the GPU capacity to support this session
		gpuSet := gpu.NewGpuSet(agent.Gpus)

		// Add the currently assigned sessions to the gpuSet
		for _, session := range agent.Sessions {
			_, err_ := gpuSet.Select(session.Gpus)
			err = errors.Join(err, err_)
		}

		selectedGpus, err_ := gpuSet.Find(requirements.Gpus)
		err = errors.Join(err, err_)
		return selectedGpus, err
	}

	return nil, nil
}

func validateSession(session storage.QueuedSession) error {
	if len(session.Requirements.Gpus) == 0 {
		return errors.New("session must request at least one GPU")
	}

	return nil
}

func (backend *Backend) update(ctx context.Context) error {
	err := backend.storage.SetAgentsMissingIfNotUpdatedFor(30 * time.Second)
	if err != nil {
		return err
	}

	err = backend.storage.RemoveMissingAgentsIfNotUpdatedFor(5 * time.Minute)
	if err != nil {
		return err
	}

	sessionIterator, err := backend.storage.GetQueuedSessionsIterator()
	if err != nil {
		return err
	}

	for sessionIterator.Next() {
		select {
		case <-ctx.Done():
			return nil

		default:
			session := sessionIterator.Value()
			err_ := validateSession(session)
			if err_ != nil {
				err = errors.Join(err_, backend.storage.CancelSession(session.Id))
				logger.Debugf("invalid session, %s", err.Error())
				continue
			}

			// Get an iterator of the agents matching a subset of the requirements
			agentIterator, err_ := backend.storage.GetAvailableAgentsMatching(storage.TotalVramRequired(session.Requirements))
			err = errors.Join(err, err_)
			if err_ == nil {
				for agentIterator.Next() {
					agent := agentIterator.Value()

					selectedGpus, err_ := agentMatches(agent, session.Requirements)
					err = err_
					if err != nil {
						logger.Debugf("unable to match agent, %s", err.Error())
						continue
					}

					if selectedGpus != nil {
						logger.Debugf("assigning %s to %s", session.Id, agent.Id)
						err = errors.Join(err, backend.storage.AssignSession(session.Id, agent.Id, selectedGpus.GetGpus()))
						break
					}
				}
			}
		}
	}

	return err
}
