package actor

import (
	"fmt"
	"sync"
)

type Router interface {
	FindActors(class string) []Actor
	Register(actor Actor)
	StateToString() string
}

type router struct {
	mtx    sync.RWMutex
	actors []Actor
}

func NewRouter() Router {
	return &router{}
}

func (r *router) Register(actor Actor) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	r.actors = append(r.actors, actor)
}

func (r *router) FindActors(class string) (actors []Actor) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	for i := 0; i < len(r.actors); i++ {
		if r.actors[i].GetMeta().Class == class {
			actors = append(actors, r.actors[i])
		}
	}
	return
}

func (r *router) StateToString() string {
	ids := []string{}
	for _, a := range r.actors {
		ids = append(ids, a.GetMeta().ID)
	}
	return fmt.Sprint(ids)
}
