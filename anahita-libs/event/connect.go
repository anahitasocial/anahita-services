package event

import (
	"errors"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type ConfigInterface interface {
	GetAmqpAddr() string
}

// connectToRabbit tries to connect to RabbitMQ, for up to 30 seconds
func ConnectToAMQP(config ConfigInterface) (*amqp.Connection, error) {
	var (
		counts int64
		conn   *amqp.Connection
	)

	for {
		connection, err := amqp.Dial(config.GetAmqpAddr())
		if err != nil {
			fmt.Println("RabbitMQ not ready...")
			counts++
		} else {
			fmt.Println()
			conn = connection
			break
		}

		if counts > AmqpConnCounts {
			fmt.Println(err)
			return nil, errors.New("cannot connect to rabbit")
		}
		fmt.Println("Backing off for 2 seconds...")
		time.Sleep(2 * time.Second)
		continue
	}

	fmt.Println("Connected to RabbitMQ!")

	return conn, nil
}
