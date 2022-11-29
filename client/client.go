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
	Conn     *amqp.Connection
	ConnChan *amqp.Channel
	Quene    *types.Item
}

func NewClient(conf *config.Config) (*Client, error) {
	conn, err := amqp.Dial(conf.AmqpUrl)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	channelConn, err := conn.Channel()
	if err != nil {
		panic(err)
	}

	return &Client{
		Conn:     conn,
		ConnChan: channelConn,
	}, nil
}

func (c *Client) SendMessage(item *types.Item) {

	req, err := json.Marshal(item)
	if err != nil {
		panic(err)
	}
	q, err := c.ConnChan.QueueDeclare(
		c.Quene.Action, // queue name
		true,           // durable
		false,          // auto delete
		false,          // exclusive
		false,          // no wait
		nil,            // arguments
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(q)

	// attempt to publish a message to the queue!
	err = c.ConnChan.Publish(
		"",
		c.Quene.Action,
		false,
		false,
		amqp.Publishing{

			ContentType: "text/plain",

			Body: req,
		},
	)
	if err != nil {
		panic(err)
	}
}

func (c *Client) AddItem(key, value string) {
	item := &types.Item{Action: "AddItem", Key: key, Value: value}
	go c.SendMessage(item)
}

func (c *Client) GetItem(key string) {
	item := &types.Item{Action: "GetItem", Key: key}
	go c.SendMessage(item)
}

func (c *Client) GetAllItems() {
	item := &types.Item{Action: "GetAllItems"}
	go c.SendMessage(item)
}

func (c *Client) RemoveItem(key string) {
	item := &types.Item{Action: "RemoveItem", Key: key}
	go c.SendMessage(item)
}

type ClientsManager struct {
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

func NewClientsManager(cfg *config.Config) (manager *ClientsManager, err error) {
	input := os.Stdin
	if len(cfg.ClientsInputPath) != 0 {
		input, err = os.Open(cfg.ClientsInputPath)
		if err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &ClientsManager{
		clients:   make(map[string]*ClientUsage),
		mux:       sync.Mutex{},
		input:     input,
		clientCfg: cfg,
		ctx:       ctx,
		Cancel:    cancel,
	}, nil
}

func (cm *ClientsManager) ListenClientActions() error {
	if cm.input == os.Stdin {
		fmt.Println("Write clients tasks here in format <clientId> <item>")
	}

	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			<-ticker.C
			cm.removeUnusedClients()
		}
	}()

	lines, errChan := SubscribeToFileInput(cm.input)

	for {
		select {
		case <-cm.ctx.Done():
			return nil
		case line := <-lines:
			if len(line) != 0 {
				go cm.processClientAction(line)
			}
		case err := <-errChan:
			if err != nil {
				return err
			}
		}
	}
}

func (cm *ClientsManager) removeUnusedClients() {
	cm.mux.Lock()
	defer cm.mux.Unlock()
	for clientId, clientUsage := range cm.clients {
		if time.Since(clientUsage.lastUsed) > time.Second*10 {
			delete(cm.clients, clientId)
		}
	}

}

func (cm *ClientsManager) processClientAction(inputStr string) error {
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
