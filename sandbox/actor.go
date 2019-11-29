package sandbox

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"sync"
)

type Request struct {
	// List of class that Request has to pass
	Route        []string
	Current      int
	ConnectionID string

	From Actor
}

type Actor interface {
	Request(request Request) (Connection, error)
	Close(connID string)
	Monitor() <-chan ConnectionEvent
	Run()

	Liveness() <-chan struct{}
	IsRegistered() <-chan struct{}

	GetMeta() Meta
	PrintState()

	Kill()
	IsAlive() bool
}

type Meta struct {
	ID    string
	Class string
	Node  string

	// Does Actor produces some 'network' side-effects
	// like kernel-interface, routing tables and etc...
	NetworkHolder bool
}

func (m Meta) Clone() Meta {
	return Meta{
		ID:            m.ID,
		Class:         m.Class,
		Node:          m.Node,
		NetworkHolder: m.NetworkHolder,
	}
}

type actor struct {
	Meta
	*connectionMonitor

	mtx sync.RWMutex

	router Router
	healer Healer

	regCh  chan struct{}
	killCh chan struct{}
	killed bool
}

func NewActor(meta Meta, router Router) Actor {
	rv := &actor{
		Meta:   meta,
		router: router,
		regCh:  make(chan struct{}),
		killCh: make(chan struct{}),
	}

	rv.connectionMonitor = newConnectionMonitor(rv.logWithConn)
	rv.healer = NewCloseHealer(rv.router, rv.connectionMonitor, rv.logWithConn)

	return rv
}

func (a *actor) GetMeta() Meta {
	a.mtx.RLock()
	defer a.mtx.RUnlock()

	return a.Meta.Clone()
}

func (a *actor) Request(request Request) (Connection, error) {
	a.mtx.RLock()
	defer a.mtx.RUnlock()

	a.log(fmt.Sprintf("request accepted: %v", request))
	if a.killed {
		return Connection{}, fmt.Errorf("sandbox '%s' is dead", a.ID)
	}

	if cw, err := a.Get(request.ConnectionID); err == nil {
		joinFunc := a.healer.Emit(SrcUp, request.ConnectionID)
		joinFunc()
		return cw.Connection, nil
	}

	if request.Current == len(request.Route)-1 {
		conn := Connection{
			ID:        request.ConnectionID,
			LastActor: a.ID,
		}
		a.storeConn(NewConnectionWrapper(conn, request, nil, a.logWithConn))
		return conn, nil
	}

	available := a.router.FindActors(request.Route[request.Current+1])
	if len(available) == 0 {
		return Connection{}, fmt.Errorf("no actors with class '%s' are available", request.Route[request.Current+1])
	}

	conn, err := available[0].Request(Request{
		Route:        request.Route,
		Current:      request.Current + 1,
		ConnectionID: request.ConnectionID,
		From:         a,
	})

	if err != nil {
		return Connection{}, err
	}

	a.storeConn(NewConnectionWrapper(conn, request, available[0], a.logWithConn))

	return conn, nil
}

func (a *actor) Close(connID string) {
	a.healer.Emit(SrcDown, connID)
}

func (a *actor) Run() {
	a.log("Started!")

	a.router.Register(a)
	close(a.regCh)
	a.log("Registered!")

	stopCh := make(chan struct{})
	joinCh := make(chan struct{})
	go func() {
		a.healer.Serve(stopCh)
		close(joinCh)
	}()

	<-a.killCh
	close(stopCh)
	<-joinCh
	a.log("Killed!")
}

func (a *actor) storeConn(cw *ConnectionWrapper) {
	a.Update(cw)
	go cw.Monitor(a.healer)
}

func (a *actor) log(s string) {
	logrus.Infof("%v: %s", a.ID, s)
}

func (a *actor) logWithConn(connID, s string) {
	a.log(fmt.Sprintf("connID = %s: %s", connID, s))
}

func (a *actor) Liveness() <-chan struct{} {
	return a.killCh
}

func (a *actor) IsRegistered() <-chan struct{} {
	return a.regCh
}

func (a *actor) Kill() {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	conns := a.connectionMonitor.List()
	for _, c := range conns {
		a.Delete(c.ID, true)
	}

	a.killed = true
	close(a.killCh)
}

func (a *actor) IsAlive() bool {
	a.mtx.RLock()
	defer a.mtx.RUnlock()

	return !a.killed
}

func (a *actor) PrintState() {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	fmt.Println("========================================")
	fmt.Printf("%s:\n", a.ID)
	defer fmt.Println("========================================")

	conns := a.connectionMonitor.List()
	for _, c := range conns {
		fmt.Printf("\tconnID = %s, State = %v\n", c.ID, c.State.String())
	}
}
