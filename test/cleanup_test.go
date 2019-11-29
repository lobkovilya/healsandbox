package test

import (
	"context"
	"github.com/lobkovilya/healsendbox/sandbox"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"testing"
	"time"
)

func TestCleanup_DyingNSC(t *testing.T) {
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
	ctx, _ := context.WithTimeout(context.Background(), 25*time.Second)
	err = forEach(actors[1:]).WaitClosed(ctx, "conn-1")
	g.Expect(err).To(BeNil())
}

func TestCleanup_DyingNSMgr(t *testing.T) {
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

	forEach(actors).WaitConnectionState("conn-1", sandbox.Ready)
	forEach(actors).PrintState()

	nsmgr := forEach(actors).FindByID("nsmgr-master")
	nsmgr.Kill()

	logrus.Info("NSMgr killed")
	ctx, _ := context.WithTimeout(context.Background(), 25*time.Second)
	err = forEach(single(forEach(actors).FindByID("icmp-responder-1"))).WaitClosed(ctx, "conn-1")
	g.Expect(err).To(BeNil())

	forEach(actors).PrintState()

}
