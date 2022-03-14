package gosse

type Stream interface {
	Value() interface{}
	Changes() chan struct{}
	Next() interface{}
	HasNext() bool
}
