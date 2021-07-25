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

func (l *Listener) RegisterOnce(key string, cb ListenerCallback) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	m, ok := l.handlers[key]
	if !ok {
		m = map[string]handler{}
		l.handlers[key] = m
	}
	m[cb.ID()] = handler{cb: cb, once: true}
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
	remove := []string{}
	for _, v := range arr {
		if v.once {
			remove = append(remove, v.cb.ID())
		}
		v.cb.Cb(key, val)
	}
	for i := len(remove) - 1; i >= 0; i-- {
		delete(arr, remove[i])
	}
}
