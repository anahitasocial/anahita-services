package main

import (
	"anahita-libs/event"
	"hashtag/cmd/api/handlers"
	"hashtag/config"
	"log"
)

func startEventListerner(h *handlers.Handlers, config *config.Config) error {
	// create amqp connection
	amqpConn, err := event.ConnectToAMQP(config)
	if err != nil {
		return err
	}

	cosumer, err := event.NewConsumer(amqpConn)
	if err != nil {
		return err
	}

	topics := []string{
		"node.medium.*.*.*.v1",
		"node.actor.*.*.*.v1",
	}
	if err = cosumer.Listen(topics, h.Hashtag.HandleEvents); err != nil {
		return err
	}

	log.Println("Event listener started successfully.")

	return nil
}
