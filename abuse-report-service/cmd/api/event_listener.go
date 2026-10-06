package main

import (
	"log"

	"abusereport/cmd/api/handlers"
	"abusereport/config"

	"anahita-libs/event"
)

func startEventListener(h *handlers.Handlers, config *config.Config) error {
	amqpConn, err := event.ConnectToAMQP(config)
	if err != nil {
		return err
	}

	consumer, err := event.NewConsumer(amqpConn)
	if err != nil {
		return err
	}

	// Everything that says a node is gone. See handlers.eventsHandler.
	topics := []string{
		"node.medium.*.*.deleted.v1",
		"node.actor.*.*.purged.v1",
	}

	if err = consumer.Listen(topics, h.Events.Handle); err != nil {
		return err
	}

	log.Println("Event listener started successfully.")

	return nil
}
