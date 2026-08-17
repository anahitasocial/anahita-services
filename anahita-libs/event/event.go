package event

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

func getExchangeName() string {
	return EventExchangeName
}

func declareExchange(ch *amqp.Channel) error {
	return ch.ExchangeDeclare(
		getExchangeName(), // name
		"topic",           // type
		true,              // durable
		false,             // auto-deleted
		false,             // internal
		false,             // no-wait
		nil,               // arguments
	)
}

// declareRandomQueue just creates a random queue
func declareRandomQueue(ch *amqp.Channel) (amqp.Queue, error) {
	args := make(amqp.Table)
	args["x-message-ttl"] = int32(QueueMessageTTL)
	return ch.QueueDeclare(
		"",    // name
		false, // durable
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		args,  // arguments
	)
}
