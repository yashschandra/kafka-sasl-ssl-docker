package main

import (
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"github.com/Shopify/sarama"
	"github.com/xdg/scram"
	"io/ioutil"
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
	conf.Net.SASL.User = "producer"
	conf.Net.SASL.Password = "producer-pass"
	conf.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
	conf.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
	conf.Net.TLS.Enable = true
	conf.Net.TLS.Config = createTLSConfig(certFilePath, keyFilePath, caFilePath)
	conf.Producer.Return.Successes = true
	producer, err := sarama.NewSyncProducer(brokers, conf)
	panicOnError(err)
	msg := &sarama.ProducerMessage{
		Topic: "test",
		Value: sarama.StringEncoder("hello hello"),
	}
	_, _, err = producer.SendMessage(msg)
	panicOnError(err)
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


