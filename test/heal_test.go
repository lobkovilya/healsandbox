package test

import (
	"github.com/lobkovilya/healsandbox/sandbox"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"testing"
)

func TestHeal_DyingNSC(t *testing.T) {
	g := NewWithT(t)

	router := sandbox.NewRouter()
	actors := actorsChain(router,
		newNSC("nsc-1", "master"),
		newNSMgr("master"),
		newNSE("icmp-responder-1", "master"))

	join := forEach(actors).Run()
	defer func() {
		logrus.Info("======= CLEANUP =======")
		join()
	}()
	forEach(actors).WaitRegistered()

	_, err := actors[0].Request(sandbox.Request{
		ConnectionID: "conn-1",
		Route: []string{
			"nsc",
			"nsmgr",
			"nse",
		},
	})
	g.Expect(err).To(BeNil())

	nsc := forEach(actors).FindByID("nsc-1")
	g.Expect(nsc).ToNot(BeNil())

	nsc.Kill()
	logrus.Info("NSC killed")
	g.Expect(forEach(actors).CheckNetworkConnectivity()).To(BeFalse())

	nsmgr := forEach(actors).FindByID("nsmgr-master")
	forEach(single(nsmgr)).WaitConnectionState("conn-1", sandbox.WaitSrc)
	logrus.Info("nsmgr moved to healing")

	newNSC := sandbox.NewActor(newNSC("nsc-2", "master"), router)
	joinNSC := forEach(single(newNSC)).Run()
	defer joinNSC()

	forEach(single(newNSC)).WaitRegistered()

	_, err = newNSC.Request(sandbox.Request{
		ConnectionID: "conn-1",
		Route: []string{
			"nsc",
			"nsmgr",
			"nse",
		},
	})
	g.Expect(err).To(BeNil())

	resultChain := append(single(newNSC), actors[1:]...)
	forEach(resultChain).WaitConnectionState("conn-1", sandbox.Ready)
	g.Expect(forEach(resultChain).CheckNetworkConnectivity()).To(BeTrue())
	forEach(append(single(newNSC), actors[1:]...)).PrintState()
}

//func TestHeal_DyingNSMgr(t *testing.T) {
//	g := NewWithT(t)
//
//	router := sandbox.NewRouter()
//	actors := actorsChain(router,
//		newNSC("nsc-1", "master"),
//		newNSMgr("master"),
//		newNSE("icmp-responder-1", "master"))
//
//	join := forEach(actors).Run()
//	defer func() {
//		logrus.Info("======= CLEANUP =======")
//		join()
//	}()
//	forEach(actors).WaitRegistered()
//
//	_, err := actors[0].Request(sandbox.Request{
//		ConnectionID: "conn-1",
//		Route: []string{
//			"nsc",
//			"nsmgr",
//			"nse",
//		},
//	})
//	g.Expect(err).To(BeNil())
//
//	forEach(actors).WaitConnectionState("conn-1", sandbox.Ready)
//	forEach(actors).PrintState()
//
//	nsmgr := forEach(actors).FindByID("nsmgr-master")
//	nsmgr.Kill()
//
//	logrus.Info("NSMgr killed")
//	<-time.After(30 * time.Second)
//
//	forEach(actors).PrintState()
//
//}
