package client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/streadway/amqp"
	"os"
	"strings"
	"sync"
	"testask/config"
	"testask/types"
	"time"
)

type Client struct {
	ConnChan *amqp.Channel
	Quene    string
}

const QueneName = "NameQuene"

func NewClient(conf *config.Config) (*Client, error) {
	conn, err := amqp.Dial(conf.AmqpUrl)
	if err != nil {
		panic(err)
	}

	channelConn, err := conn.Channel()
	if err != nil {
		panic(err)
	}

	q, err := channelConn.QueueDeclare(
		QueneName, // queue name
		false,     // durable
		false,     // auto delete
		false,     // exclusive
		false,     // no wait
		nil,       // arguments
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(q)

	return &Client{
		ConnChan: channelConn,
		Quene:    QueneName,
	}, nil
}

func (c *Client) SendMessage(item *types.Item) {

	req, err := json.Marshal(item)
	if err != nil {
		panic(err)
	}

	// attempt to publish a message to the queue!
	err = c.ConnChan.Publish(
		"",
		c.Quene,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         req,
			DeliveryMode: amqp.Persistent,
		},
	)
	if err != nil {
		panic(err)
	}
}

func (c *Client) AddI(key, value string) {
	item := &types.Item{Method: "AddItem", Key: key, Value: value}
	go c.SendMessage(item)
}

func (c *Client) GetI(key string) {
	item := &types.Item{Method: "GetItem", Key: key}
	go c.SendMessage(item)
}

func (c *Client) GetAllI() {
	item := &types.Item{Method: "GetAllItems"}
	go c.SendMessage(item)
}

func (c *Client) DeleteI(key string) {
	item := &types.Item{Method: "DeleteItem", Key: key}
	go c.SendMessage(item)
}

type ClientManager struct {
	clients   map[string]*ClientUsage
	input     *os.File
	clientCfg *config.Config
	mux       sync.Mutex
	ctx       context.Context
	Cancel    context.CancelFunc
}

type ClientUsage struct {
	client   *Client
	lastUsed time.Time
}

func ClientsManager(cfg *config.Config) (manager *ClientManager, err error) {
	input := os.Stdin
	if len(cfg.ClientsInputPath) != 0 {
		input, err = os.Open(cfg.ClientsInputPath)
		if err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &ClientManager{
		clients:   make(map[string]*ClientUsage),
		mux:       sync.Mutex{},
		input:     input,
		clientCfg: cfg,
		ctx:       ctx,
		Cancel:    cancel,
	}, nil
}

func (cm *ClientManager) ListenClientActions() error {
	if cm.input == os.Stdin {
		fmt.Println("Write clients tasks here in format <clientId> <item>")
	}

	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			<-ticker.C
			cm.deleteUnusedClients()
		}
	}()

	lines, errChan := SubscribeToFileInput(cm.input)

	for {
		select {
		case <-cm.ctx.Done():
			return nil
		case line := <-lines:
			if len(line) != 0 {
				err := cm.ClientAction(line)
				if err != nil {
					return err
				}
			}
		case err := <-errChan:
			if err != nil {
				return err
			}
		}
	}
}

func (cm *ClientManager) deleteUnusedClients() {
	cm.mux.Lock()
	defer cm.mux.Unlock()
	for clientId, clientUsage := range cm.clients {
		if time.Since(clientUsage.lastUsed) > time.Second*10 {
			delete(cm.clients, clientId)
		}
	}

}

func (cm *ClientManager) ClientAction(inputStr string) error {
	cm.mux.Lock()
	defer cm.mux.Unlock()
	if len(inputStr) <= 1 {
		return fmt.Errorf("Wrong input string. Should be in format <clientId> <item>")
	}

	splittedInput := strings.Split(inputStr, " ")

	clientId := splittedInput[0]
	itemStr := strings.Join(splittedInput[1:], " ")

	var item *types.Item
	err := json.Unmarshal([]byte(itemStr), &item)
	if err != nil {
		return err
	}
	if client, ok := cm.clients[clientId]; ok {
		go client.client.SendMessage(item)
		client.lastUsed = time.Now()
		return nil
	}
	client, err := NewClient(cm.clientCfg)
	if err != nil {
		return err
	}
	cm.clients[clientId] = &ClientUsage{client, time.Now()}
	go client.SendMessage(item)
	return nil
}
