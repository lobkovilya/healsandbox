package test

import (
	"context"
	"fmt"
	"github.com/lobkovilya/healsendbox/actor"
	"github.com/sirupsen/logrus"
	"sync"
)

type forEach []actor.Actor

func single(a actor.Actor) []actor.Actor {
	return []actor.Actor{a}
}

func list(actors ...actor.Actor) []actor.Actor {
	return actors
}

func (f forEach) Run() func() {
	var wg sync.WaitGroup
	for i := 0; i < len(f); i++ {
		wg.Add(1)
		a := f[i]
		go func() {
			defer func() {
				wg.Done()
			}()
			a.Run()
		}()
	}
	return func() {
		f.Kill()
		wg.Wait()
	}
}

func (f forEach) WaitRegistered() {
	readyCh := make(chan struct{}, len(f))

	for i := 0; i < len(f); i++ {
		a := f[i]
		go func() {
			<-a.IsRegistered()
			readyCh <- struct{}{}
		}()
	}

	for i := 0; i < len(f); i++ {
		<-readyCh
	}
	logrus.Info("all actors successfully registered")
}

func (f forEach) CheckNetworkConnectivity() bool {
	for i := 0; i < len(f); i++ {
		if !f[i].IsAlive() && f[i].GetMeta().NetworkHolder {
			return false
		}
	}
	return true
}

func (f forEach) Kill() {
	for i := 0; i < len(f); i++ {
		if f[i].IsAlive() {
			f[i].Kill()
		}
	}
}

func (f forEach) WaitClosed(ctx context.Context, connId string) error {
	readyCh := make(chan struct{}, len(f))

	for i := 0; i < len(f); i++ {
		a := f[i]
		go func() {
			monitor := a.Monitor()
			for event := range monitor {
				if event.EventType != actor.Delete {
					continue
				}

				for _, c := range event.Connections {
					if c.ID == connId {
						readyCh <- struct{}{}
						return
					}
				}
			}
		}()
	}

	for i := 0; i < len(f); i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-readyCh:
			continue
		}
	}

	return nil
}

func (f forEach) WaitConnectionState(connID string, state actor.HealState) {
	var wg sync.WaitGroup

	for i := 0; i < len(f); i++ {
		a := f[i]
		wg.Add(1)
		go func() {
			defer wg.Done()

			monitor := a.Monitor()
			for event := range monitor {
				if event.EventType == actor.Delete {
					continue
				}

				for _, c := range event.Connections {
					if c.ID == connID && c.State == state {
						return
					}
				}
			}
		}()
	}
}

func (f forEach) FindByID(id string) actor.Actor {
	for i := 0; i < len(f); i++ {
		if f[i].GetMeta().ID == id {
			return f[i]
		}
	}
	return nil
}

func (f forEach) PrintState() {
	fmt.Println()
	for i := 0; i < len(f); i++ {
		f[i].PrintState()
		fmt.Println()
	}
}

func actorsChain(router actor.Router, meta ...actor.Meta) []actor.Actor {
	actors := make([]actor.Actor, 0, len(meta))

	for _, m := range meta {
		actors = append(actors, actor.New(m, router))
	}

	return actors
}

func newNSC(id, node string) actor.Meta {
	return actor.Meta{
		ID:            id,
		Node:          node,
		Class:         "nsc",
		NetworkHolder: true,
	}
}

func newNSE(id, node string) actor.Meta {
	return actor.Meta{
		ID:            id,
		Node:          node,
		Class:         "nse",
		NetworkHolder: true,
	}
}

func newNSMgr(node string) actor.Meta {
	return actor.Meta{
		ID:            fmt.Sprintf("nsmgr-%s", node),
		Node:          node,
		Class:         "nsmgr",
		NetworkHolder: false,
	}
}

func newForwarder(id, node string) actor.Meta {
	return actor.Meta{
		ID:            id,
		Node:          node,
		Class:         "forwarder",
		NetworkHolder: true,
	}
}
