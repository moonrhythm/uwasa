package uwasa

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/acoshift/arpc/v2"
	"github.com/moonrhythm/randid"
)

var (
	ErrForbidden    = arpc.NewError("uwasa: forbidden")
	ErrServerClosed = arpc.NewError("uwasa: server closed")
)

type Event struct {
	Topic string `json:"topic"`
	Data  string `json:"data"`
}

type Server struct {
	ctx         context.Context
	eventBuffer chan *Event
	mu          sync.RWMutex
	bufferSize  int
	subscribeCh map[randid.ID]chan<- []byte
}

func NewServer(ctx context.Context, bufferSize int) *Server {
	s := &Server{
		ctx:         ctx,
		eventBuffer: make(chan *Event, bufferSize),
		bufferSize:  bufferSize,
		subscribeCh: make(map[randid.ID]chan<- []byte),
	}
	go s.startEventLoop()
	return s
}

func (s *Server) startEventLoop() {
	for {
		select {
		case e := <-s.eventBuffer:
			s.pushEventToClient(e)
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Server) pushEventToClient(e *Event) {
	data, err := json.Marshal(e)
	if err != nil {
		log.Printf("uwasa: marshal event error; %v", err)
		return
	}

	for _, f := range s.subscribeCh {
		select {
		case f <- data:
		default:
		}
	}
}

func (s *Server) Publish(ctx context.Context, e *Event) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.ctx.Done():
		return ErrServerClosed
	case s.eventBuffer <- e:
	}

	return nil
}

func (s *Server) registerSubscribeCh(ch chan<- []byte) randid.ID {
	s.mu.Lock()
	defer s.mu.Unlock()

	for {
		id := randid.MustGenerate()
		if s.subscribeCh[id] == nil {
			s.subscribeCh[id] = ch
			return id
		}
	}
}

func (s *Server) Subscribe(ctx context.Context, sse arpc.SSEResponseWriter) error {
	sse.WriteHeader(http.StatusOK)

	ch := make(chan []byte, s.bufferSize)
	id := s.registerSubscribeCh(ch)

	defer func() {
		s.mu.Lock()
		delete(s.subscribeCh, id)
		s.mu.Unlock()
		close(ch)
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.ctx.Done():
			return ErrServerClosed
		case data := <-ch:
			err := sse.WriteData(string(data))
			if err != nil {
				return err
			}
		}
	}
}
