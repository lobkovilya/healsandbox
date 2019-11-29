package sandbox

import (
	"fmt"
	"sync"
)

type Connection struct {
	ID        string
	LastActor string
}

type ConnectionWrapper struct {
	Connection
	State   HealState
	request Request

	next           Actor
	stopCh         chan struct{}
	resetWaitSrcCh chan struct{}
	resetWaitDstCh chan struct{}
	logFunc        func(connID, str string)
	wg             sync.WaitGroup
}

func NewConnectionWrapper(conn Connection, request Request, next Actor, logFunc func(connID, str string)) *ConnectionWrapper {
	return &ConnectionWrapper{
		Connection: conn,
		next:       next,
		request:    request,
		stopCh:     make(chan struct{}),
		logFunc:    logFunc,
		State:      Ready,
	}
}

func (c *ConnectionWrapper) Destroy() {
	close(c.stopCh)
	c.wg.Wait()
}

func (c *ConnectionWrapper) Monitor(healer Healer) {
	if c.next != nil {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()

			err := waitDeath(c.next, c.stopCh)
			if err == nil {
				return
			}
			c.logFunc(c.ID, "down")
			healer.Emit(DstDown, c.ID)
		}()
	}
	if c.request.From != nil {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()

			err := waitDeath(c.request.From, c.stopCh)
			if err == nil {
				return
			}
			c.logFunc(c.ID, "down")
			healer.Emit(SrcDown, c.ID)
		}()
	}
}

func waitDeath(peer Actor, stopCh <-chan struct{}) error {
	select {
	case <-peer.Liveness():
		return fmt.Errorf("peer %v is dead", peer.GetMeta().ID)
	case <-stopCh:
		return nil
	}
}
