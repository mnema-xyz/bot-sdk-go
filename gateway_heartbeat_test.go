package mnema

import (
	"math"
	"testing"
	"time"
)

func TestTheHELLOHeartbeatIntervalIsClamped(t *testing.T) {
	c := &Client{minHeartbeat: minHeartbeatInterval}
	cases := []struct {
		ms   int
		want time.Duration
	}{
		{1, time.Second},
		{999, time.Second},
		{1000, time.Second},
		{41250, 41250 * time.Millisecond},
		{300000, 5 * time.Minute},
		{86400000, 5 * time.Minute},
		{math.MaxInt, 5 * time.Minute},
	}
	for _, tc := range cases {
		if got := c.clampHeartbeat(tc.ms); got != tc.want {
			t.Errorf("clampHeartbeat(%d) = %v, want %v", tc.ms, got, tc.want)
		}
	}
}

func TestATinyHELLOIntervalDoesNotMakeTheClientBeatInATightLoop(t *testing.T) {
	g := newFakeGateway(t, 1)
	c := g.client(t, Options{Intents: IntentGuilds})
	c.minHeartbeat = minHeartbeatInterval
	runClient(t, c)
	fc := g.next(t)
	fc.expect(t, opIdentify)
	fc.ready(t, "s-1")
	beats := 0
	timeout := time.After(700 * time.Millisecond)
	for {
		select {
		case f := <-fc.frames:
			if f.Op == opHeartbeat {
				beats++
				fc.write(t, opHeartbeatACK, nil)
			}
		case <-timeout:
			if beats > 1 {
				t.Fatalf("%d heartbeats in 700ms for a 1ms HELLO interval, want at most 1", beats)
			}
			return
		}
	}
}
