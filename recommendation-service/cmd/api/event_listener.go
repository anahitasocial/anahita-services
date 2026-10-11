package main

import (
	"log"

	"recommendation/cmd/api/handlers"
	"recommendation/config"

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
	// An actor removed for good.
	topics := []string{
		"node.actor.*.*.purged.v1",
	}

	if err = consumer.Listen(topics, h.Events.Handle); err != nil {
		return err
	}

	log.Println("Event listener started successfully.")

	return nil
}
