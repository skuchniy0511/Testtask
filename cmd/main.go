package main

import (
	"flag"
	"github.com/streadway/amqp"
	"testask/client"

	"testask/config"
	"testask/server"
)

func main() {
	configFilePathFlag := flag.String("config", "./config.yaml", "Path to config file")
	flag.Parse()

	if configFilePathFlag == nil {
		panic("Specify config file path with --config flag")
	}

	cfg, err := config.LoadConfig(*configFilePathFlag)
	if err != nil {
		panic(err)
	}
	ch, err := GetConnection(cfg)
	if err != nil {
		panic(err)
	}
	serverR, err := server.NewServer(cfg, ch)
	if err != nil {
		panic(err)
	}

	go serverR.StartServer()

	clientsManager, err := client.NewClientsManager(cfg)
	if err != nil {
		panic(err)
	}

	err = clientsManager.ListenClientActions()
	if err != nil {
		panic(err)
	}
}

func GetConnection(conf *config.Config) (*amqp.Channel, error) {
	conn, err := amqp.Dial(conf.AmqpUrl)
	if err != nil {
		return &amqp.Channel{}, err
	}
	ch, err := conn.Channel()
	if err != nil {
		return &amqp.Channel{}, err
	}
	return ch, nil
}
