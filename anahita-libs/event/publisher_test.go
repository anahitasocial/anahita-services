package event

import "testing"

type unreachable struct{}

func (unreachable) GetAmqpAddr() string { return "amqp://guest:guest@127.0.0.1:1/" }

// A lazy publisher is made whether the bus is there or not, and only says
// so when it has something to send, each time, without stopping.
func TestALazyPublisherConnectsOnlyWhenItHasSomethingToSay(t *testing.T) {
	p := NewLazyPublisher(unreachable{})
	if p == nil {
		t.Fatal("no publisher")
	}
	for i := 0; i < 2; i++ {
		if err := p.Publish(&EventPayload{}, "node.medium.text-service.note.added.v1"); err == nil {
			t.Error("sent to a bus that is not there")
		}
	}
}
