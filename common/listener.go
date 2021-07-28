package common

import "sync"

type ListenerCallback interface {
	ID() string
	Cb(key string, val interface{})
}

type handler struct {
	cb   ListenerCallback
	once bool
}

type Listener struct {
	handlers map[string]map[string]handler
	mutex    sync.Mutex
}

func NewListener() *Listener {
	return &Listener{
		handlers: make(map[string]map[string]handler),
	}
}

func (l *Listener) Register(key string, cb ListenerCallback) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	m, ok := l.handlers[key]
	if !ok {
		m = map[string]handler{}
		l.handlers[key] = m
	}
	m[cb.ID()] = handler{cb: cb, once: false}
}

func (l *Listener) Unregister(key string, cb ListenerCallback) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	m, ok := l.handlers[key]
	if !ok {
		return
	}
	delete(m, cb.ID())
}

func (l *Listener) Broadcast(key string, val interface{}) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	arr := l.handlers[key]
	for _, v := range arr {
		v.cb.Cb(key, val)
	}
}
