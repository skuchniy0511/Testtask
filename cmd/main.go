package main

import (
	"flag"
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

	serverR, err := server.NewServer(cfg)
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

func StartServerAsync(conf *config.Config) chan error {
	errChan := make(chan error)
	server, err := server.NewServer(conf)
	if err != nil {
		errChan <- err
	}
	go func() {
		err = server.StartServer()
		if err != nil {
			errChan <- err
		}
	}()
	return errChan
}
