package event

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher sends events to the bus.
//
// One made by NewPublisher knows where the bus is, and connects again when
// the bus has closed its connection: RabbitMQ restarted, say, after the
// service had connected to it. Without that, every event after the restart
// was lost without a word (banu.io, 2026-11-22: a channel's post never
// reached Bluesky). One made by NewEventPublisher, from a connection made
// elsewhere, cannot connect again; whoever made it does that.
type Publisher struct {
	mu     sync.Mutex
	conn   *amqp.Connection
	config ConfigInterface
}

// NewPublisher connects to the bus and declares the exchange, and connects
// again whenever the bus closes the connection.
func NewPublisher(config ConfigInterface) (*Publisher, error) {
	conn, err := ConnectToAMQP(config)
	if err != nil {
		return nil, err
	}
	publisher := &Publisher{conn: conn, config: config}
	if err := publisher.setup(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return publisher, nil
}

// NewEventPublisher sends events over a connection made elsewhere, which it
// does not open again when it is closed.
func NewEventPublisher(conn *amqp.Connection) (*Publisher, error) {
	publisher := &Publisher{conn: conn}
	if err := publisher.setup(); err != nil {
		return nil, err
	}
	return publisher, nil
}

func (p *Publisher) setup() error {
	channel, err := p.conn.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()
	return declareExchange(channel)
}

// channel opens a channel for one event, on a connection made again first
// when the bus closed the last one.
func (p *Publisher) channel() (*amqp.Channel, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn == nil {
		return nil, errors.New("not connected to the bus")
	}
	if !p.conn.IsClosed() {
		channel, err := p.conn.Channel()
		if err == nil || p.config == nil {
			return channel, err
		}
	}
	if p.config == nil {
		return nil, amqp.ErrClosed
	}

	// Once, without waiting: an event is sent while somebody waits for an
	// answer, and the next event tries again.
	conn, err := amqp.Dial(p.config.GetAmqpAddr())
	if err != nil {
		return nil, err
	}
	_ = p.conn.Close()
	p.conn = conn
	log.Println("event: connected to the bus again")
	if err := p.setup(); err != nil {
		return nil, err
	}
	return p.conn.Channel()
}

// Publish sends an event to the exchange. What goes wrong is logged here,
// since most callers send and go on.
func (p *Publisher) Publish(payload *EventPayload, routing_key string) error {
	if payload.Name == "" {
		payload.Name = routing_key
	}
	// None at all: a test, or a tool that sends nothing. Said, not a panic.
	if p == nil {
		log.Printf("event %s not sent: no publisher", payload.Name)
		return errors.New("event: no publisher")
	}

	channel, err := p.channel()
	if err != nil {
		log.Printf("event %s not sent: %v", payload.Name, err)
		return err
	}
	defer channel.Close()

	log.Println("Publishing to", getExchangeName())

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("event %s not sent: %v", payload.Name, err)
		return err
	}

	err = channel.PublishWithContext(
		context.Background(),
		getExchangeName(),
		routing_key,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        body,
		},
	)
	if err != nil {
		log.Printf("event %s not sent: %v", payload.Name, err)
		return err
	}

	log.Printf("Sending message: %s -> %s", payload.Name, getExchangeName())

	return nil
}
