package sandbox

import (
	"fmt"
	"time"
)

type HealState int

const (
	Unknown HealState = iota
	Requesting
	Ready
	WaitSrc
	WaitDst
	Healing
	Closing
)

const (
	WaitDstTimeout = 5 * time.Second
	WaitSrcTimeout = 2 * WaitDstTimeout
)

func (h HealState) String() string {
	switch h {
	case Unknown:
		return "Unknown"
	case Requesting:
		return "Requesting"
	case Ready:
		return "Ready"
	case WaitSrc:
		return "WaitSrc"
	case WaitDst:
		return "WaitDst"
	case Healing:
		return "Healing"
	case Closing:
		return "Closing"
	default:
		panic("unknown heal State")
	}
}

type HealEvent int

const (
	SrcDown HealEvent = iota
	SrcUp
	DstDown
	DstUp
	Timeout
)

func (h HealEvent) String() string {
	switch h {
	case SrcDown:
		return "SrcDown"
	case SrcUp:
		return "SrcUp"
	case DstDown:
		return "DstDown"
	case DstUp:
		return "DstUp"
	case Timeout:
		return "Timeout"
	default:
		panic("unknown event")
	}
}

type Healer interface {
	Emit(event HealEvent, connId string) func()
	Serve(stopCh <-chan struct{})
}

type CloseHealer struct {
	router      Router
	connections ConnectionDomain
	transitions map[HealState]map[HealEvent]HealState
	handlers    map[HealState]func(cd *ConnectionWrapper)
	logFunc     func(string, string)
	eventCh     chan struct {
		event  HealEvent
		connID string
		joinCh chan struct{}
	}
}

func NewCloseHealer(router Router, connections ConnectionDomain, logFunc func(string, string)) Healer {
	rv := &CloseHealer{
		router:      router,
		connections: connections,
		logFunc:     logFunc,
		transitions: map[HealState]map[HealEvent]HealState{
			Ready: {
				SrcDown: WaitSrc,
				DstDown: WaitDst,
				SrcUp:   Ready,
			},
			WaitSrc: {
				Timeout: Closing,
				SrcUp:   Healing,
			},
			WaitDst: {
				Timeout: Closing,
				DstUp:   Healing,
			},
			Healing: {
				DstUp: Ready,
			},
		},
		eventCh: make(chan struct {
			event  HealEvent
			connID string
			joinCh chan struct{}
		}, 1),
	}

	rv.handlers = map[HealState]func(cd *ConnectionWrapper){
		WaitSrc: rv.WaitSrc,
		WaitDst: rv.WaitDst,
		Closing: rv.Closing,
		Healing: rv.Healing,
	}

	return rv
}

func (c *CloseHealer) Emit(event HealEvent, connID string) func() {
	c.logFunc(connID, fmt.Sprintf("emit event: %v", event))
	joinCh := make(chan struct{})

	c.eventCh <- struct {
		event  HealEvent
		connID string
		joinCh chan struct{}
	}{event: event, connID: connID, joinCh: joinCh}

	return func() { <-joinCh }
}

func (c *CloseHealer) WaitSrc(cw *ConnectionWrapper) {
	c.logFunc(cw.ID, "handler for 'WaitSrc' State")
	cw.resetWaitSrcCh = make(chan struct{})
	go func() {
		select {
		case <-cw.resetWaitSrcCh:
			return
		case <-time.After(WaitSrcTimeout):
			c.Emit(Timeout, cw.ID)
		}
	}()
}

func (c *CloseHealer) WaitDst(cw *ConnectionWrapper) {
	c.logFunc(cw.ID, "handler for 'WaitDst' State")
	cw.resetWaitDstCh = make(chan struct{})
	go func() {
		select {
		case <-cw.resetWaitDstCh:
			return
		case <-time.After(WaitDstTimeout):
			c.Emit(Timeout, cw.ID)
		}
	}()
}

func (c *CloseHealer) Closing(cw *ConnectionWrapper) {
	c.logFunc(cw.ID, "handler for 'Closing' State")
	if cw.next != nil {
		cw.next.Close(cw.ID)
	}
	c.connections.Delete(cw.ID, false)
}

func (c *CloseHealer) Healing(cw *ConnectionWrapper) {
	c.logFunc(cw.ID, "handler for 'Healing' State")
	if cw.resetWaitSrcCh != nil {
		close(cw.resetWaitSrcCh)
	}

	// TODO: some logic where we should decide do we need to 'next' sandbox

	if cw.next == nil {
		return
	}

	conn, err := cw.next.Request(cw.request)
	if err != nil {
		c.logFunc(cw.ID, fmt.Sprintf("error during 'Healing' State: %v", err))
	}

	cw.Connection = conn
	c.connections.Update(cw)

	c.Emit(DstUp, cw.ID)
}

func (c *CloseHealer) Serve(stopCh <-chan struct{}) {
	for {
		select {
		case <-stopCh:
			return
		case event := <-c.eventCh:
			c.logFunc(event.connID, fmt.Sprintf("new event %v received", event.event))
			cd, err := c.connections.Get(event.connID)
			if err != nil {
				c.logFunc(event.connID, err.Error())
				continue
			}

			c.transit(cd, event.event)
			close(event.joinCh)
		}
	}
}

func (c *CloseHealer) transit(cw *ConnectionWrapper, event HealEvent) {
	newState := c.nextState(cw.State, event)
	c.logFunc(cw.ID, fmt.Sprintf("change State from %v to %v, event - %v", cw.State, newState, event))
	cw.State = newState
	c.connections.Update(cw)
	h, ok := c.handlers[newState]
	if ok {
		h(cw)
	}
}

func (c *CloseHealer) nextState(current HealState, event HealEvent) HealState {
	events, ok := c.transitions[current]
	if !ok {
		panic(fmt.Sprintf("unknown transition for State %v with event %v", current, event))
	}

	newState, ok := events[event]
	if !ok {
		panic(fmt.Sprintf("unknown transition for State %v with event %v", current, event))
	}

	return newState
}
