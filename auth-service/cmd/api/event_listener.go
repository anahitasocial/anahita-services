package main

import (
	"log"

	"anahita-libs/event"
	"auth/cmd/api/handlers"
	"auth/config"
)

// startEventListener subscribes auth-service to actor lifecycle events.
//
// It has published events for a long time and never consumed any. The one that
// matters is actor deletion: person-service marks the profile deleted, and this
// service holds everything the person could still sign in with — refresh
// tokens, the IdP session cookie, passkeys. Without a consumer, all three
// outlived the account.
//
// Subscribed to the whole actor topic rather than to a deleted-only routing
// key, matching how person-service and hashtag-service subscribe; the handler
// filters. Group deletions arrive here too and resolve to no credentials, which
// the handler treats as a no-op rather than an error.
func startEventListener(h *handlers.Handlers, cfg *config.Config) error {
	amqpConn, err := event.ConnectToAMQP(cfg)
	if err != nil {
		return err
	}

	consumer, err := event.NewConsumer(amqpConn)
	if err != nil {
		return err
	}

	topics := []string{
		"node.actor.*.*.*.v1",
	}

	if err = consumer.Listen(topics, h.AccountDeleted.HandleEvents); err != nil {
		return err
	}

	log.Println("Event listener started successfully.")

	return nil
}
