/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package frontend

import (
	"context"
	"encoding/json"

	"github.com/Juice-Labs/Juice-Labs/pkg/utilities"
)

type Message struct {
	topic string
	msg   json.RawMessage
}

type AgentHandler struct {
	subscriberChannels *utilities.ConcurrentMap[string, chan []byte]
}

func NewAgentHandler(id string) *AgentHandler {
	return &AgentHandler{
		subscriberChannels: utilities.NewConcurrentMap[string, chan []byte](),
	}
}

func (handler *AgentHandler) Subscribe(ctx context.Context, topic string) (<-chan []byte, error) {
	msgCh := make(chan []byte, 1)

	// WARNING: Only expecting one channel per topic

	handler.subscriberChannels.Set(topic, msgCh)

	go func() {
		<-ctx.Done()

		handler.subscriberChannels.Delete(topic)

		close(msgCh)
	}()

	return msgCh, nil
}

func (handler *AgentHandler) Publish(topic string, msg []byte) error {
	msgCh, found := handler.subscriberChannels.Get(topic)
	if found {
		msgCh <- msg
	}

	return nil
}
