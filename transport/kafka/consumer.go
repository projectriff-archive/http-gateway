package kafka

import (
	"github.com/projectriff/function-sidecar/pkg/dispatcher"
	"github.com/projectriff/function-sidecar/pkg/wireformat"
	"github.com/bsm/sarama-cluster"
	"log"
)

func NewKafkaConsumer(addrs []string, groupID string, topics []string) (*consumer, error) {
	consumerConfig := makeConsumerConfig()
	clusterConsumer, err := cluster.NewConsumer(addrs, groupID, topics, consumerConfig)
	if err != nil {
		return &consumer{}, err
	}

	if consumerConfig.Consumer.Return.Errors {
		go consumeErrors(clusterConsumer)
	}
	if consumerConfig.Group.Return.Notifications {
		go consumeNotifications(clusterConsumer)
	}

	messages := make(chan dispatcher.Message)

	go func(clusterConsumer *cluster.Consumer) {
		for {
			kafkaMsg := <-clusterConsumer.Messages()
			messageWithHeaders, err := wireformat.FromKafka(kafkaMsg)
			if err != nil {
				log.Println("Failed to extract message ", err)
				continue
			}
			messages <- messageWithHeaders
			clusterConsumer.MarkOffset(kafkaMsg, "") // mark message as processed
		}
	}(clusterConsumer)

	return &consumer{
		clusterConsumer: clusterConsumer,
		messages: messages,
	}, nil
}

type consumer struct {
	clusterConsumer *cluster.Consumer
	messages <-chan dispatcher.Message
}

func (c *consumer) Messages() <-chan dispatcher.Message {
	return c.messages
}

func (c *consumer) Close() error {
	return c.clusterConsumer.Close()
}

func makeConsumerConfig() *cluster.Config {
	consumerConfig := cluster.NewConfig()
	consumerConfig.Consumer.Return.Errors = true
	consumerConfig.Group.Return.Notifications = true
	return consumerConfig
}

func consumeErrors(consumer *cluster.Consumer) {
	for err := range consumer.Errors() {
		log.Printf("Error: %s\n", err.Error())
	}
}

func consumeNotifications(consumer *cluster.Consumer) {
	for ntf := range consumer.Notifications() {
		log.Printf("Rebalanced: %+v\n", ntf)
	}
}

