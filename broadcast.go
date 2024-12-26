package main

import "sync"

type BroadcastServer[T any] struct {
	chans []chan T
	sync.RWMutex
}

func (b *BroadcastServer[T]) Subscribe() chan T {
	result := make(chan T)
	b.Lock()
	b.chans = append(b.chans, result)
	b.Unlock()
	return result
}

func (b *BroadcastServer[T]) Broadcast(message T) int {
	b.RLock()
	defer b.RUnlock()
	for _, c := range b.chans {
		c <- message
	}

	return len(b.chans)
}

func (b *BroadcastServer[T]) Close() int {
	b.RLock()
	n := len(b.chans)

	for _, c := range b.chans {
		close(c)
	}
	b.RUnlock()

	b.Lock()
	b.chans = nil
	b.Unlock()

	return n
}

type StopBroadcast = BroadcastServer[struct{}]
