package impl

import (
	"github.com/Gentleelephant/logging-controller/internal/types"
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/extension"
	"github.com/reugn/go-streams/flow"
	"log"
	"regexp"
)

type Keylog struct {
	Name            string
	RegexExpression string
	in              chan any
	out             chan any
	filter          *flow.Filter[any]
	parallelism     int
	operations      []streams.Flow
}

func NewKeylogInstance(name, expression string, parallelism int, operations []streams.Flow) *Keylog {
	keylog := Keylog{
		Name:            name,
		RegexExpression: expression,
		in:              make(chan any),
		out:             make(chan any),
		parallelism:     parallelism,
		operations:      operations,
	}
	keylog.filter = flow.NewFilter(keylog.predict, keylog.parallelism)

	return &keylog
}

func (k *Keylog) In() chan any {
	return k.in
}

func (k *Keylog) Out() chan any {
	return k.out
}

func (k *Keylog) Start() {
	//
	log.Println("=====>启动filter")
	source := extension.NewChanSource(k.in)
	sink := extension.NewChanSink(k.out)

	f := source.Via(k.filter)
	for _, operator := range k.operations {
		f = f.Via(operator)
	}
	go f.To(sink)
}

func (k *Keylog) Close() {
	log.Println("=====>关闭in channel")
	// 需要数据写入端不往当前通道写入数据
	// 通道关闭，需要关闭filter的
	close(k.in)
}

func (k *Keylog) GetName() string {
	return k.Name
}

func (k *Keylog) predict(l any) bool {
	if l == nil {
		return false
	}
	msg, ok := l.(types.Result)
	if !ok {
		log.Println("Invalid log type:", l)
		return false
	}
	regex, err := regexp.Compile(k.RegexExpression)
	if err != nil {
		log.Println("Invalid regex pattern:", err)
		return false
	}
	match := regex.Match([]byte(msg.Log))
	if !match {
		return false
	}
	return true
}
