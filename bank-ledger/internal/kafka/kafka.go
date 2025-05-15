package kafka

import (
	"bank-ledger/internal/conf"
	"time"

	"github.com/IBM/sarama"
	"github.com/go-kratos/kratos/v2/log"
)

// Producer is a kafka producer interface
type Producer interface {
	SendMessage(topic string, key, value []byte) error
	Close() error
}

type kafkaProducer struct {
	producer sarama.SyncProducer
	log      *log.Helper
}

func NewProducer(c *conf.Data, logger log.Logger) (Producer, error) {
	l := log.NewHelper(log.With(logger, "module", "kafka/producer"))

	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.RequiredAcks(c.Kafka.RequiredAcks)
	config.Producer.Retry.Max = int(c.Kafka.Retries)
	config.Producer.Return.Successes = true

	if c.Kafka.Timeout != nil {
		timeout := c.Kafka.Timeout.AsDuration()
		config.Producer.Timeout = timeout
	} else {
		config.Producer.Timeout = 3 * time.Second
	}

	producer, err := sarama.NewSyncProducer(c.Kafka.Brokers, config)
	if err != nil {
		l.Errorf("failed to create kafka producer: %v", err)
		return nil, err
	}

	l.Infof("kafka producer created successfully, brokers: %v", c.Kafka.Brokers)

	return &kafkaProducer{
		producer: producer,
		log:      l,
	}, nil
}

func (p *kafkaProducer) SendMessage(topic string, key, value []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(value),
	}

	if key != nil {
		msg.Key = sarama.ByteEncoder(key)
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		p.log.Errorf("failed to send message: %v", err)
		return err
	}

	p.log.Infof("message sent to partition %d at offset %d", partition, offset)
	return nil
}

func (p *kafkaProducer) Close() error {
	if err := p.producer.Close(); err != nil {
		p.log.Errorf("failed to close producer: %v", err)
		return err
	}

	p.log.Info("producer closed successfully")
	return nil
}
