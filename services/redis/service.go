package redis

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/go-redis/redis"
	"github.com/influxdata/kapacitor/alert"
	"github.com/influxdata/kapacitor/keyvalue"
)

type Diagnostic interface {
	WithContext(ctx ...keyvalue.T) Diagnostic
	Error(msg string, err error)
}

type Service struct {
	mu     sync.RWMutex
	logger *log.Logger
	config Config
	cli    *redis.Client
	diag   Diagnostic
}

type handler struct {
	s      *Service
	logger *log.Logger
	diag   Diagnostic
	queue  string
}

type HandlerConfig struct {
	Queue string `mapstructure:"queue"`
}

func NewService(c Config, d Diagnostic) *Service {
	return &Service{
		diag:   d,
		config: c,
		cli: redis.NewClient(&redis.Options{
			Addr:     c.Addr,
			DB:       c.DB,
			Password: c.Password,
		}),
	}
}

func (s *Service) Open() error {
	return nil
}

func (s *Service) Close() error {
	s.cli.Close()
	return nil
}

func (s *Service) Update(newConfig []interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := len(newConfig); n != 1 {
		return fmt.Errorf("expected only one new config object, got %d", n)
	}
	c, ok := newConfig[0].(Config)
	if !ok {
		return fmt.Errorf("expected config object to be of type %T, got %T", c, newConfig[0])
	}
	if err := c.Validate(); err != nil {
		return err
	}
	s.cli.Close()
	if c.Enabled {
		s.cli = redis.NewClient(&redis.Options{
			Addr:     c.Addr,
			DB:       c.DB,
			Password: c.Password,
		})
	}
	s.config = c
	return nil
}

func (s *Service) Alert(queue string, event *alert.Event) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msg, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("can't marshal event to json: %v", err)
	}
	return s.SendMessageToQueue(queue, msg)
}

func (s *Service) SendMessageToQueue(queue string, msg []byte) error {
	if _, err := s.cli.LPush(queue, msg).Result(); err != nil {
		return fmt.Errorf("can't send to redis queue: %v", err)
	}
	return nil
}

func (s *Service) Handler(c HandlerConfig, ctx ...keyvalue.T) (alert.Handler, error) {
	return &handler{
		s:      s,
		queue:  c.Queue,
		logger: s.logger,
		diag:   s.diag.WithContext(ctx...),
	}, nil
}

type testOptions struct {
	Queue   string `json:"queue"`
	Message string `json:"msg"`
}

func (s *Service) TestOptions() interface{} {
	return &testOptions{
		Queue:   "_test_queue_",
		Message: "test redis message",
	}
}

func (s *Service) Test(options interface{}) error {
	o, ok := options.(*testOptions)
	if !ok {
		return fmt.Errorf("unexpected options type %t", options)
	}
	return s.SendMessageToQueue(o.Queue, []byte(o.Message))
}

func (h *handler) Handle(event alert.Event) {
	if err := h.s.Alert(h.queue, &event); err != nil {
		h.diag.Error("E! failed to handle event", err)
	}
}
