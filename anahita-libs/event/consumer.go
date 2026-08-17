package event

import (
	"encoding/json"
	"log"

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

	forever := make(chan bool)
	go func() {
		for d := range messages {
			payload := &EventPayload{}
			_ = json.Unmarshal(d.Body, payload)
			go handlePayload(payload)
		}
	}()

	log.Printf("[*] Waiting for message [Exchange, Queue][%s, %s].", getExchangeName(), q.Name)
	<-forever
	return nil
}
