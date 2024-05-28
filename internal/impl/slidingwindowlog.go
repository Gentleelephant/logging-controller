package impl

import (
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/extension"
	"github.com/reugn/go-streams/flow"
	"time"
)

type SlidingWindowLog struct {
	Name            string
	WindowSize      int
	SlidingInterval int
	in              chan any
	out             chan any
	slidingWindow   *flow.SlidingWindow[any]
	operations      []streams.Flow
}

func NewSlidingWindowInstance(name string, windowSize int, slidingInterval int, operations []streams.Flow) *SlidingWindowLog {
	slidingWindowLog := SlidingWindowLog{
		Name:            name,
		WindowSize:      windowSize,
		SlidingInterval: slidingInterval,
		in:              make(chan any),
		out:             make(chan any),
		operations:      operations,
	}
	dwin := time.Duration(windowSize) * time.Millisecond
	dsi := time.Duration(slidingInterval) * time.Millisecond
	slidingWindowLog.slidingWindow = flow.NewSlidingWindow[any](dwin, dsi)
	return &slidingWindowLog
}

func (s *SlidingWindowLog) In() chan any {
	return s.in
}

func (s *SlidingWindowLog) Out() chan any {
	return s.out
}

func (s *SlidingWindowLog) Start() {
	source := extension.NewChanSource(s.in)
	sink := extension.NewChanSink(s.out)
	f := source.Via(s.slidingWindow)
	for _, operator := range s.operations {
		f = f.Via(operator)
	}
	go f.To(sink)
}

func (s *SlidingWindowLog) GetName() string {
	return s.Name
}

func (s *SlidingWindowLog) Close() {
	close(s.in)
}
