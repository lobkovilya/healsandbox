package sandbox

import (
	"fmt"
	"sync"
)

type ConnectionEventType int

func (et ConnectionEventType) String() string {
	switch et {
	case 0:
		return "InitialTransfer"
	case 1:
		return "Update"
	case 2:
		return "Delete"
	default:
		panic("unknown event type")
	}
}

const (
	InitialTransfer ConnectionEventType = iota
	Update
	Delete
)

const capacity = 10

type ConnectionEvent struct {
	EventType   ConnectionEventType
	Connections map[string]*ConnectionWrapper
}

type Monitor interface {
	Monitor() <-chan ConnectionEvent
}

type ConnectionDomain interface {
	Update(cw *ConnectionWrapper)
	Delete(connID string, silent bool)
	Get(connID string) (*ConnectionWrapper, error)
}

type connectionMonitor struct {
	sync.Mutex
	recipients  []chan<- ConnectionEvent
	logFunc     func(connID, str string)
	connections sync.Map
}

func newConnectionMonitor(logFunc func(connID, str string)) *connectionMonitor {
	return &connectionMonitor{
		recipients:  []chan<- ConnectionEvent{},
		logFunc:     logFunc,
		connections: sync.Map{},
	}
}

func (cm *connectionMonitor) Monitor() <-chan ConnectionEvent {
	cm.Lock()
	defer cm.Unlock()
	ch := make(chan ConnectionEvent, capacity)
	cm.recipients = append(cm.recipients, ch)

	go func() {
		conns := map[string]*ConnectionWrapper{}
		cm.connections.Range(func(key, value interface{}) bool {
			conns[key.(string)] = value.(*ConnectionWrapper)
			return true
		})
		ch <- ConnectionEvent{
			EventType:   InitialTransfer,
			Connections: conns,
		}
	}()
	return ch
}

func (cm *connectionMonitor) List() (conns []*ConnectionWrapper) {
	cm.connections.Range(func(key, value interface{}) bool {
		conns = append(conns, value.(*ConnectionWrapper))
		return true
	})
	return
}

func (cm *connectionMonitor) Update(cw *ConnectionWrapper) {
	cm.logFunc(cw.ID, fmt.Sprintf("update: %v", cw))
	cm.connections.Store(cw.ID, cw)
	cm.send(ConnectionEvent{
		EventType: Update,
		Connections: map[string]*ConnectionWrapper{
			cw.ID: cw,
		},
	})
}

func (cm *connectionMonitor) Delete(connID string, silent bool) {
	cm.logFunc(connID, "delete")
	uncast, _ := cm.connections.Load(connID)
	cw := uncast.(*ConnectionWrapper)
	cw.Destroy()

	cm.connections.Delete(connID)
	if !silent {
		cm.send(ConnectionEvent{
			EventType: Delete,
			Connections: map[string]*ConnectionWrapper{
				connID: cw,
			},
		})
	}
}

func (cm *connectionMonitor) Get(connID string) (*ConnectionWrapper, error) {
	conn, ok := cm.connections.Load(connID)
	if !ok {
		return nil, fmt.Errorf("no connection with id %v", connID)
	}

	return conn.(*ConnectionWrapper), nil
}

func (cm *connectionMonitor) send(event ConnectionEvent) {
	cm.Lock()
	defer cm.Unlock()

	for _, ch := range cm.recipients {
		ch <- event
	}
}
