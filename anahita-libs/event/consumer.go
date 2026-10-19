package event

import (
	"encoding/json"
	"errors"
	"log"
	"runtime/debug"

	amqp "github.com/rabbitmq/amqp091-go"
)

type HandlePayload func(payload *EventPayload) error

type Consumer struct {
	conn *amqp.Connection
}

func NewConsumer(conn *amqp.Connection) (Consumer, error) {
	consumer := Consumer{
		conn: conn,
	}
	err := consumer.setup()
	if err != nil {
		return Consumer{}, err
	}

	return consumer, nil
}

// setup opens a channel and declares the exchange
func (consumer *Consumer) setup() error {
	channel, err := consumer.conn.Channel()
	if err != nil {
		return err
	}

	return declareExchange(channel)
}

// Listen will listen for all new queue publications
func (consumer *Consumer) Listen(topics []string, handlePayload HandlePayload) error {
	ch, err := consumer.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	q, err := declareRandomQueue(ch)
	if err != nil {
		return err
	}

	for _, s := range topics {
		err = ch.QueueBind(
			q.Name,
			s,
			getExchangeName(),
			false,
			nil,
		)
		if err != nil {
			log.Println(err)
			return err
		}
	}

	messages, err := ch.Consume(
		q.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	log.Printf("[*] Waiting for message [Exchange, Queue][%s, %s].", getExchangeName(), q.Name)

	// Each message is acknowledged as it arrives and never redelivered: a
	// handler that fails part way is not run again, so nothing it wrote or
	// sent is written or sent twice (anahita.io sent the same email again
	// and again on 2026-10-15 that way). Its error is logged, and a panic in
	// it is caught, so one bad event takes nothing else down.
	for d := range messages {
		payload := &EventPayload{}
		if err := json.Unmarshal(d.Body, payload); err != nil {
			log.Printf("event on %s not read: %v", d.RoutingKey, err)
			continue
		}
		go handle(handlePayload, payload)
	}

	// The bus closed the channel: said, and returned, so that the service
	// stops and is started again, or listens again, instead of hearing
	// nothing from here on.
	return errors.New("event: the bus closed the channel; listening stopped")
}

// handle runs one handler, and says what went wrong in it.
func handle(handlePayload HandlePayload, payload *EventPayload) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("event %s: the handler panicked: %v\n%s", payload.Name, r, debug.Stack())
		}
	}()
	if err := handlePayload(payload); err != nil {
		log.Printf("event %s: %v", payload.Name, err)
	}
}
