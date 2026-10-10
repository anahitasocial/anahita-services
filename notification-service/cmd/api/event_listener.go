package main

import (
	"anahita-libs/event"
	"log"
	"notification/cmd/api/handlers"
	"notification/config"
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
		"node.repost.*.*.*.v1",
		"edge.voteup.*.*.v1",
		"edge.follow.*.*.v1",
		"edge.follow-request.*.*.v1",
		"edge.invite.*.*.v1",
		"edge.admin.*.*.v1",
	}
	if err = cosumer.Listen(topics, h.Event.HandleEvents); err != nil {
		return err
	}

	log.Println("Event listener started successfully.")

	return nil
}
