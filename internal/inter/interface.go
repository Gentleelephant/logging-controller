package inter

type LogOperator interface {
	In() chan any
	Out() chan any
	Start()
	GetName() string
	Close()
}
