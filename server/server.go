package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/streadway/amqp"
	"sync"
	"testask/config"
	"testask/types"

	"github.com/inconshreveable/log15"
)

type Server struct {
	data     *types.OrderedMap
	Queue    *types.Item
	Conn     *amqp.Connection
	channel  *amqp.Channel
	queueUrl string
	waitTime int64
	logFile  log15.Logger
	ctx      context.Context
	Cancel   context.CancelFunc
	logger   log15.Logger
	dataMux  sync.RWMutex
	logsMux  sync.Mutex
}

func NewServer(conf *config.Config) (*Server, error) {
	conn, err := amqp.Dial(conf.AmqpUrl)
	if err != nil {
		panic(err)
	}
	ch, err := conn.Channel()
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	logger := log15.New("service", "server")
	logger.SetHandler(log15.LvlFilterHandler(log15.LvlDebug, log15.StdoutHandler))

	logFile := log15.New()
	logfileHandler, err := log15.FileHandler(conf.LogFilePath, log15.LogfmtFormat())
	if err != nil {
		return nil, err
	}

	logFile.SetHandler(logfileHandler)

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		data:    types.NewOrderedMap(),
		channel: ch,
		Conn:    conn,
		ctx:     ctx,
		Cancel:  cancel,

		logger:   logger,
		dataMux:  sync.RWMutex{},
		logsMux:  sync.Mutex{},
		waitTime: conf.ServerWaitTimeSeconds,
		logFile:  logFile,
	}, nil
}

func (s *Server) StartServer() error {
	s.logger.Debug("Listening queue!")
	messagesChan := make(chan *amqp.Delivery)
	listenMsg := make(chan byte)
	go s.listenMessages(listenMsg)
	err := s.processMessages(messagesChan)
	return err
}

func (s *Server) listenMessages(messagesChan chan byte) {

	receivedMessages := make(chan *amqp.Delivery)
	errChan := s.receiveMessages(receivedMessages)
	for {
		select {
		case <-s.ctx.Done():
			return
		case msgResult := <-receivedMessages:
			if msgResult != nil {
				for _, message := range msgResult.Body {
					messagesChan <- message
				}
			}
		case err := <-errChan:
			s.logger.Error("Error while receiving messages", "error", err.Error())
		}
	}
}

func (s *Server) receiveMessages(messages chan *amqp.Delivery) chan error {
	errChan := make(chan error)

	ch, err := s.Conn.Channel()
	if err != nil {
		panic(err)
	}
	defer ch.Close()

	msg, err := ch.Consume(

		s.Queue.Action,
		"",
		true,
		false,
		false,
		false,
		nil)
	forever := make(chan bool)

	go func() {
		for d := range msg {
			fmt.Printf("Try to recieve a message: %s\n\n", d.Body)
		}
	}()

	<-forever
	return errChan
}

func (s *Server) processMessages(messagesChan chan *amqp.Delivery) error {
	for {
		select {
		case <-s.ctx.Done():
			return nil
		case message := <-messagesChan:
			if message == nil {
				continue
			}
			go func(messageData string) {
				var item *types.Item
				err := json.Unmarshal([]byte(messageData), &item)
				if err != nil {
					s.logger.Error("Cannot unmarhsal message", "error", err.Error())
				}
				if item != nil {
					log := s.processItem(item)
					s.logsMux.Lock()
					defer s.logsMux.Unlock()
					s.logFile.Info(log)
				}
				removed, err := s.channel.QueuePurge(message.MessageId, false)
				fmt.Sprintf("quene sucssesfully removed: %s\n", removed)

				if err != nil {
					s.logger.Error("Error while deleting message", "error", err)
				}
			}(string(message.Body))
		}
	}
}

func (s *Server) processItem(item *types.Item) (logMessage string) {
	switch item.Action {
	case types.AddItem:
		s.dataMux.Lock()
		defer s.dataMux.Unlock()
		ok := s.data.Set(item.Key, item.Value)
		return fmt.Sprintf("SetItem() done. Item(key: %s, value: %s) created: %t", item.Key, item.Value, ok)
	case types.GetItem:
		s.dataMux.RLock()
		defer s.dataMux.RUnlock()
		v, _ := s.data.Get(item.Key)
		return fmt.Sprintf("GetItem() done. Item(key: %s, value: %s)", item.Key, v)
	case types.GetAllItems:
		s.dataMux.RLock()
		defer s.dataMux.RUnlock()
		resp := ""
		for _, key := range s.data.Keys() {
			v, _ := s.data.Get(key)
			resp += fmt.Sprintf(" Item(key: %s, value: %s)", key, v)
		}
		return fmt.Sprintf("GetAllTimes() done.%s", resp)
	case types.RemoveItem:
		s.dataMux.Lock()
		defer s.dataMux.Unlock()
		ok := s.data.Delete(item.Key)
		return fmt.Sprintf("DeleteItem() done. Item(key: %s) deleted: %t", item.Key, ok)
	default:
		return "Unknown action!"
	}
}
