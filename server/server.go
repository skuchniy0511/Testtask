package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/streadway/amqp"
	"os"
	"testask/config"
	"testask/types"

	"github.com/inconshreveable/log15"
)

type Server struct {
	data     *types.OrderedMap
	Consumer string
	channel  *amqp.Channel
	queueUrl string
	waitTime int64
	logFile  log15.Logger
	ctx      context.Context
	Cancel   context.CancelFunc
	logger   log15.Logger
}

const consumer = "Consumer"
const QueneName = "NameQuene"

func NewServer(conf *config.Config, ch *amqp.Channel) (*Server, error) {

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
		data: types.NewOrderedMap(),

		channel:  ch,
		ctx:      ctx,
		Cancel:   cancel,
		Consumer: consumer,
		logger:   logger,

		waitTime: conf.ServerWaitTimeSeconds,
		logFile:  logFile,
	}, nil
}

func (s *Server) StartServer() error {
	s.logger.Debug("Listening queue!")
	messagesChan := make(chan *amqp.Delivery, 50)
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

	_, err := s.channel.QueueDeclare(QueneName, false, false, false, false, nil)
	if err != nil {
		return nil
	}
	//fmt.Println(declare)
	msgs, err := s.channel.Consume(
		QueneName,
		"",
		true,
		false,
		false,
		false,
		nil)
	if err != nil {
		panic(err)
	}

	// create a goroutine for the number of concurrent threads requested
	go func() {
		var Msg amqp.Delivery
		for msg := range msgs {

			fmt.Printf("Try to recieve a message %s\n", msg.Body)
			Msg = msg
		}

		fmt.Println("Rabbit consumer closed - critical Error")

		messages <- &Msg
		os.Exit(1)
	}()

	return errChan
}

func handler(d *amqp.Delivery) bool {
	if d.Body == nil {
		fmt.Println("Error, no message body!")
		return false
	}
	fmt.Println(string(d.Body))
	return true
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
					s.data.Lock()
					defer s.data.Unlock()
					s.logFile.Info(log)
				}
				if handler(message) {
					message.Ack(false)
				} else {
					message.Nack(false, true)
				}

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
		s.data.Lock()
		defer s.data.Unlock()
		ok := s.data.Set(item.Key, item.Value)
		return fmt.Sprintf("SetItem() done. Item(key: %s, value: %s) created: %t", item.Key, item.Value, ok)
	case types.GetItem:
		s.data.Lock()
		defer s.data.Unlock()
		v, _ := s.data.Get(item.Key)
		return fmt.Sprintf("GetItem() done. Item(key: %s, value: %s)", item.Key, v)
	case types.GetAllItems:
		s.data.Lock()
		defer s.data.Unlock()
		resp := ""
		for _, key := range s.data.Keys() {
			v, _ := s.data.Get(key)
			resp += fmt.Sprintf(" Item(key: %s, value: %s)", key, v)
		}
		return fmt.Sprintf("GetAllTimes() done.%s", resp)
	case types.RemoveItem:
		s.data.Lock()
		defer s.data.Unlock()
		ok := s.data.Delete(item.Key)
		return fmt.Sprintf("DeleteItem() done. Item(key: %s) deleted: %t", item.Key, ok)
	default:
		return "Unknown action!"
	}
}
