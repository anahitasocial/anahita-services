package event

import (
	"context"
	"encoding/json"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	conn *amqp.Connection
}

func (p *Publisher) setup() error {
	channel, err := p.conn.Channel()
	if err != nil {
		panic(err)
	}

	defer channel.Close()
	return declareExchange(channel)
}

// Publish a specified message to the AMQP exchange
func (p *Publisher) Publish(payload *EventPayload, routing_key string) error {
	channel, err := p.conn.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()

	log.Println("Publishing to", getExchangeName())

	if payload.Name == "" {
		payload.Name = routing_key
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("Failed to marshal event: %s", err)
	}

	err = channel.PublishWithContext(
		context.Background(),
		getExchangeName(),
		routing_key,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		},
	)

	if err != nil {
		log.Println(err)
		return err
	}

	log.Printf("Sending message: %s -> %s", payload.Name, getExchangeName())

	return nil
}

// NewEventPublisher returns a new event.Publisher object
// ensuring that the object is initialised, without error
func NewEventPublisher(conn *amqp.Connection) (*Publisher, error) {
	publisher := &Publisher{
		conn: conn,
	}

	err := publisher.setup()
	if err != nil {
		return &Publisher{}, err
	}

	return publisher, nil
}
