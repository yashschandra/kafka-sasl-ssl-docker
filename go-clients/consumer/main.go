package main

import (
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/xdg/scram"
	"io/ioutil"
	"os"
	"os/signal"
	"sync"
)

const (
	caFilePath = "/path/to/certs/ca.pem"

	keyFilePath = "/path/to/certs/client.key"

	certFilePath = "/path/to/certs/client.pem"
)

func panicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	brokers := []string{"kafka.confluent.local:9093"}
	conf := sarama.NewConfig()
	conf.Producer.RequiredAcks = sarama.WaitForAll
	conf.Net.SASL.Enable = true
	conf.Net.SASL.User = "consumer"
	conf.Net.SASL.Password = "consumer-pass"
	conf.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
	conf.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
	conf.Net.TLS.Enable = true
	conf.Net.TLS.Config = createTLSConfig(certFilePath, keyFilePath, caFilePath)
	conf.Producer.Return.Successes = true
	client, err := sarama.NewClient(brokers, conf)
	panicOnError(err)
	consumer, err := sarama.NewConsumerFromClient(client)
	panicOnError(err)
	consumerLoop(consumer, "test")
}

func createTLSConfig(certFile, keyFile, caFile string) *tls.Config {
	if certFile == "" && keyFile == "" && caFile == "" {
		return &tls.Config{}
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil
	}

	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	t := &tls.Config {
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		InsecureSkipVerify: false,
	}

	return t
}

func consumerLoop(consumer sarama.Consumer, topic string) {
	partitions, err := consumer.Partitions(topic)
	panicOnError(err)

	// Trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	var wg sync.WaitGroup
	for partition := range partitions {
		wg.Add(1)
		go func() {
			consumePartition(consumer, int32(partition), signals)
			wg.Done()
		}()
	}
	wg.Wait()
}

func consumePartition(consumer sarama.Consumer, partition int32, signals chan os.Signal) {
	fmt.Println("Receving on partition", partition)
	partitionConsumer, err := consumer.ConsumePartition("test", partition, sarama.OffsetNewest)
	panicOnError(err)
	defer func() {
		if err := partitionConsumer.Close(); err != nil {

		}
	}()

	consumed := 0
ConsumerLoop:
	for {
		select {
		case msg := <-partitionConsumer.Messages():
			fmt.Printf("Consumed message offset %d\nData: %s\n", msg.Offset, msg.Value)
			consumed++
		case <-signals:
			break ConsumerLoop
		}
	}
	fmt.Printf("Consumed: %d\n", consumed)
}

var SHA256 scram.HashGeneratorFcn = sha256.New
var SHA512 scram.HashGeneratorFcn = sha512.New

type XDGSCRAMClient struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

func (x *XDGSCRAMClient) Begin(userName, password, authzID string) (err error) {
	x.Client, err = x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	x.ClientConversation = x.Client.NewConversation()
	return nil
}

func (x *XDGSCRAMClient) Step(challenge string) (response string, err error) {
	response, err = x.ClientConversation.Step(challenge)
	return
}

func (x *XDGSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}


