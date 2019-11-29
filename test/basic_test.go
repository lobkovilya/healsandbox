package test

import (
	"github.com/lobkovilya/healsendbox/actor"
	. "github.com/onsi/gomega"
	"testing"
)

// basic tests have to work even if healing is not implemented
// they check some fundamental contracts

func TestBasic_ConnectionEstablished(t *testing.T) {
	g := NewWithT(t)

	router := actor.NewRouter()
	actors := actorsChain(router,
		newNSC("nsc-1", "master"),
		newNSMgr("master"),
		newNSE("icmp-responder-1", "master"))

	join := forEach(actors).Run()
	defer join()

	forEach(actors).WaitRegistered()

	resp, err := actors[0].Request(actor.Request{
		ConnectionID: "conn-id",
		Route: []string{
			"nsc",
			"nsmgr",
			"nse",
		},
	})

	forEach(actors).PrintState()
	g.Expect(err).To(BeNil())
	g.Expect(resp.LastActor).To(Equal("icmp-responder-1"))
	g.Expect(forEach(actors).CheckNetworkConnectivity()).To(BeTrue())
}

func TestBasic_HealFailed(t *testing.T) {
	g := NewWithT(t)

	router := actor.NewRouter()
	actors := actorsChain(router,
		newNSC("nsc-1", "master"),
		newNSMgr("master"),
		newForwarder("fw1", "master"),
		newNSE("icmp-responder-1", "master"))
	join := forEach(actors).Run()
	defer join()
	forEach(actors).WaitRegistered()

	resp, err := actors[0].Request(actor.Request{
		ConnectionID: "conn-id",
		Route: []string{
			"nsc",
			"nsmgr",
			"forwarder",
			"nse",
		},
	})
	g.Expect(err).To(BeNil())
	g.Expect(resp.LastActor).To(Equal("icmp-responder-1"))
	g.Expect(forEach(actors).CheckNetworkConnectivity()).To(BeTrue())

	actors[1].Kill()
	g.Expect(forEach(actors).CheckNetworkConnectivity()).To(BeTrue())

	actors[2].Kill()
	g.Expect(forEach(actors).CheckNetworkConnectivity()).To(BeFalse())
}
