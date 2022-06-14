/**
 * Standalone signaling server for the Nextcloud Spreed app.
 * Copyright (C) 2022 struktur AG
 *
 * @author Joachim Bauch <bauch@struktur.de>
 *
 * @license GNU AGPL version 3 or any later version
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */
package signaling

import (
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

func GetSubjectForBackendRoomId(roomId string, backend *Backend) string {
	if backend == nil || backend.IsCompat() {
		return GetEncodedSubject("backend.room", roomId)
	}

	return GetEncodedSubject("backend.room", roomId+"|"+backend.Id())
}

func GetSubjectForRoomId(roomId string, backend *Backend) string {
	if backend == nil || backend.IsCompat() {
		return GetEncodedSubject("room", roomId)
	}

	return GetEncodedSubject("room", roomId+"|"+backend.Id())
}

func GetSubjectForUserId(userId string, backend *Backend) string {
	if backend == nil || backend.IsCompat() {
		return GetEncodedSubject("user", userId)
	}

	return GetEncodedSubject("user", userId+"|"+backend.Id())
}

func GetSubjectForSessionId(sessionId string, backend *Backend) string {
	return "session." + sessionId
}

type asyncSubscriberCloser interface {
	close()
}

type asyncSubscriber struct {
	mu     sync.Mutex
	key    string
	client NatsClient

	receiver     chan *nats.Msg
	closeChan    chan bool
	subscription NatsSubscription

	processMessage func(*nats.Msg)
}

func newAsyncSubscriber(key string, client NatsClient) (*asyncSubscriber, error) {
	receiver := make(chan *nats.Msg, 64)
	sub, err := client.Subscribe(key, receiver)
	if err != nil {
		return nil, err
	}

	result := &asyncSubscriber{
		key:    key,
		client: client,

		receiver:     receiver,
		closeChan:    make(chan bool),
		subscription: sub,
	}
	return result, nil
}

func (s *asyncSubscriber) run() {
	defer func() {
		if err := s.subscription.Unsubscribe(); err != nil {
			log.Printf("Error unsubscribing %s: %s", s.key, err)
		}
	}()

	for {
		select {
		case msg := <-s.receiver:
			s.processMessage(msg)
			for count := len(s.receiver); count > 0; count-- {
				s.processMessage(<-s.receiver)
			}
		case <-s.closeChan:
			return
		}
	}
}

func (s *asyncSubscriber) close() {
	close(s.closeChan)
}

type asyncBackendRoomSubscriber struct {
	*asyncSubscriber

	listeners map[AsyncBackendRoomEventListener]bool
}

func newAsyncBackendRoomSubscriber(key string, client NatsClient) (*asyncBackendRoomSubscriber, error) {
	sub, err := newAsyncSubscriber(key, client)
	if err != nil {
		return nil, err
	}

	result := &asyncBackendRoomSubscriber{
		asyncSubscriber: sub,
		listeners:       make(map[AsyncBackendRoomEventListener]bool),
	}
	result.processMessage = result.doProcessMessage
	go result.run()
	return result, nil
}

func (s *asyncBackendRoomSubscriber) doProcessMessage(msg *nats.Msg) {
	var message AsyncMessage
	if err := s.client.Decode(msg, &message); err != nil {
		log.Printf("Could not decode NATS message %+v, %s", msg, err)
		return
	}

	switch message.Type {
	case "room":
		s.processBackendRoomRequest(message.Room)
	default:
		log.Printf("Unsupported NATS room request with type %s: %+v", message.Type, message)
	}
}

func (s *asyncBackendRoomSubscriber) processBackendRoomRequest(message *BackendServerRoomRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for listener := range s.listeners {
		s.mu.Unlock()
		listener.ProcessBackendRoomRequest(message)
		s.mu.Lock()
	}
}

func (s *asyncBackendRoomSubscriber) addListener(listener AsyncBackendRoomEventListener) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.listeners[listener] = true
}

func (s *asyncBackendRoomSubscriber) removeListener(listener AsyncBackendRoomEventListener) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.listeners, listener)
	return len(s.listeners) > 0
}

type asyncRoomSubscriber struct {
	*asyncSubscriber

	listeners map[AsyncRoomEventListener]bool
}

func newAsyncRoomSubscriber(key string, client NatsClient) (*asyncRoomSubscriber, error) {
	sub, err := newAsyncSubscriber(key, client)
	if err != nil {
		return nil, err
	}

	result := &asyncRoomSubscriber{
		asyncSubscriber: sub,
		listeners:       make(map[AsyncRoomEventListener]bool),
	}
	result.processMessage = result.doProcessMessage
	go result.run()
	return result, nil
}

func (s *asyncRoomSubscriber) doProcessMessage(msg *nats.Msg) {
	var message AsyncMessage
	if err := s.client.Decode(msg, &message); err != nil {
		log.Printf("Could not decode nats message %+v, %s", msg, err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for listener := range s.listeners {
		s.mu.Unlock()
		listener.ProcessAsyncRoomMessage(&message)
		s.mu.Lock()
	}
}

func (s *asyncRoomSubscriber) addListener(listener AsyncRoomEventListener) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.listeners[listener] = true
}

func (s *asyncRoomSubscriber) removeListener(listener AsyncRoomEventListener) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.listeners, listener)
	return len(s.listeners) > 0
}

type asyncUserSubscriber struct {
	*asyncSubscriber

	listeners map[AsyncUserEventListener]bool
}

func newAsyncUserSubscriber(key string, client NatsClient) (*asyncUserSubscriber, error) {
	sub, err := newAsyncSubscriber(key, client)
	if err != nil {
		return nil, err
	}

	result := &asyncUserSubscriber{
		asyncSubscriber: sub,
		listeners:       make(map[AsyncUserEventListener]bool),
	}
	result.processMessage = result.doProcessMessage
	go result.run()
	return result, nil
}

func (s *asyncUserSubscriber) doProcessMessage(msg *nats.Msg) {
	var message AsyncMessage
	if err := s.client.Decode(msg, &message); err != nil {
		log.Printf("Could not decode nats message %+v, %s", msg, err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for listener := range s.listeners {
		s.mu.Unlock()
		listener.ProcessAsyncUserMessage(&message)
		s.mu.Lock()
	}
}

func (s *asyncUserSubscriber) addListener(listener AsyncUserEventListener) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.listeners[listener] = true
}

func (s *asyncUserSubscriber) removeListener(listener AsyncUserEventListener) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.listeners, listener)
	return len(s.listeners) > 0
}

type asyncSessionSubscriber struct {
	*asyncSubscriber

	listeners map[AsyncSessionEventListener]bool
}

func newAsyncSessionSubscriber(key string, client NatsClient) (*asyncSessionSubscriber, error) {
	sub, err := newAsyncSubscriber(key, client)
	if err != nil {
		return nil, err
	}

	result := &asyncSessionSubscriber{
		asyncSubscriber: sub,
		listeners:       make(map[AsyncSessionEventListener]bool),
	}
	result.processMessage = result.doProcessMessage
	go result.run()
	return result, nil
}

func (s *asyncSessionSubscriber) doProcessMessage(msg *nats.Msg) {
	var message AsyncMessage
	if err := s.client.Decode(msg, &message); err != nil {
		log.Printf("Could not decode nats message %+v, %s", msg, err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for listener := range s.listeners {
		s.mu.Unlock()
		listener.ProcessAsyncSessionMessage(&message)
		s.mu.Lock()
	}
}

func (s *asyncSessionSubscriber) addListener(listener AsyncSessionEventListener) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.listeners[listener] = true
}

func (s *asyncSessionSubscriber) removeListener(listener AsyncSessionEventListener) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.listeners, listener)
	return len(s.listeners) > 0
}

type asyncEventsNats struct {
	mu     sync.RWMutex
	client NatsClient

	backendRoomSubscriptions map[string]*asyncBackendRoomSubscriber
	roomSubscriptions        map[string]*asyncRoomSubscriber
	userSubscriptions        map[string]*asyncUserSubscriber
	sessionSubscriptions     map[string]*asyncSessionSubscriber
}

func NewAsyncEventsNats(client NatsClient) (AsyncEvents, error) {
	events := &asyncEventsNats{
		client: client,

		backendRoomSubscriptions: make(map[string]*asyncBackendRoomSubscriber),
		roomSubscriptions:        make(map[string]*asyncRoomSubscriber),
		userSubscriptions:        make(map[string]*asyncUserSubscriber),
		sessionSubscriptions:     make(map[string]*asyncSessionSubscriber),
	}
	return events, nil
}

func (e *asyncEventsNats) Close() {
	e.client.Close()

	e.mu.Lock()
	defer e.mu.Unlock()
	go func(subscriptions map[string]*asyncBackendRoomSubscriber) {
		for _, sub := range subscriptions {
			sub.close()
		}
	}(e.backendRoomSubscriptions)
	go func(subscriptions map[string]*asyncRoomSubscriber) {
		for _, sub := range subscriptions {
			sub.close()
		}
	}(e.roomSubscriptions)
	e.backendRoomSubscriptions = make(map[string]*asyncBackendRoomSubscriber)
	e.roomSubscriptions = make(map[string]*asyncRoomSubscriber)
}

func (e *asyncEventsNats) RegisterBackendRoomListener(roomId string, backend *Backend, listener AsyncBackendRoomEventListener) error {
	key := GetSubjectForBackendRoomId(roomId, backend)

	e.mu.Lock()
	defer e.mu.Unlock()
	sub, found := e.backendRoomSubscriptions[key]
	if !found {
		var err error
		sub, err = newAsyncBackendRoomSubscriber(key, e.client)
		if err != nil {
			return err
		}

		e.backendRoomSubscriptions[key] = sub
	}
	sub.addListener(listener)
	return nil
}

func (e *asyncEventsNats) UnregisterBackendRoomListener(roomId string, backend *Backend, listener AsyncBackendRoomEventListener) {
	key := GetSubjectForBackendRoomId(roomId, backend)

	e.mu.Lock()
	defer e.mu.Unlock()
	sub, found := e.backendRoomSubscriptions[key]
	if !found {
		return
	}

	if !sub.removeListener(listener) {
		delete(e.backendRoomSubscriptions, key)
		sub.close()
	}
}

func (e *asyncEventsNats) RegisterRoomListener(roomId string, backend *Backend, listener AsyncRoomEventListener) error {
	key := GetSubjectForRoomId(roomId, backend)

	e.mu.Lock()
	defer e.mu.Unlock()
	sub, found := e.roomSubscriptions[key]
	if !found {
		var err error
		sub, err = newAsyncRoomSubscriber(key, e.client)
		if err != nil {
			return err
		}

		e.roomSubscriptions[key] = sub
	}
	sub.addListener(listener)
	return nil
}

func (e *asyncEventsNats) UnregisterRoomListener(roomId string, backend *Backend, listener AsyncRoomEventListener) {
	key := GetSubjectForRoomId(roomId, backend)

	e.mu.Lock()
	defer e.mu.Unlock()
	sub, found := e.roomSubscriptions[key]
	if !found {
		return
	}

	if !sub.removeListener(listener) {
		delete(e.roomSubscriptions, key)
		sub.close()
	}
}

func (e *asyncEventsNats) RegisterUserListener(roomId string, backend *Backend, listener AsyncUserEventListener) error {
	key := GetSubjectForUserId(roomId, backend)

	e.mu.Lock()
	defer e.mu.Unlock()
	sub, found := e.userSubscriptions[key]
	if !found {
		var err error
		sub, err = newAsyncUserSubscriber(key, e.client)
		if err != nil {
			return err
		}

		e.userSubscriptions[key] = sub
	}
	sub.addListener(listener)
	return nil
}

func (e *asyncEventsNats) UnregisterUserListener(roomId string, backend *Backend, listener AsyncUserEventListener) {
	key := GetSubjectForUserId(roomId, backend)

	e.mu.Lock()
	defer e.mu.Unlock()
	sub, found := e.userSubscriptions[key]
	if !found {
		return
	}

	if !sub.removeListener(listener) {
		delete(e.userSubscriptions, key)
		sub.close()
	}
}

func (e *asyncEventsNats) RegisterSessionListener(sessionId string, backend *Backend, listener AsyncSessionEventListener) error {
	key := GetSubjectForSessionId(sessionId, backend)

	e.mu.Lock()
	defer e.mu.Unlock()
	sub, found := e.sessionSubscriptions[key]
	if !found {
		var err error
		sub, err = newAsyncSessionSubscriber(key, e.client)
		if err != nil {
			return err
		}

		e.sessionSubscriptions[key] = sub
	}
	sub.addListener(listener)
	return nil
}

func (e *asyncEventsNats) UnregisterSessionListener(sessionId string, backend *Backend, listener AsyncSessionEventListener) {
	key := GetSubjectForSessionId(sessionId, backend)

	e.mu.Lock()
	defer e.mu.Unlock()
	sub, found := e.sessionSubscriptions[key]
	if !found {
		return
	}

	if !sub.removeListener(listener) {
		delete(e.sessionSubscriptions, key)
		sub.close()
	}
}

func (e *asyncEventsNats) publish(subject string, message *AsyncMessage) error {
	message.SendTime = time.Now()
	return e.client.Publish(subject, message)
}

func (e *asyncEventsNats) PublishBackendRoomMessage(roomId string, backend *Backend, message *AsyncMessage) error {
	subject := GetSubjectForBackendRoomId(roomId, backend)
	return e.publish(subject, message)
}

func (e *asyncEventsNats) PublishRoomMessage(roomId string, backend *Backend, message *AsyncMessage) error {
	subject := GetSubjectForRoomId(roomId, backend)
	return e.publish(subject, message)
}

func (e *asyncEventsNats) PublishUserMessage(userId string, backend *Backend, message *AsyncMessage) error {
	subject := GetSubjectForUserId(userId, backend)
	return e.publish(subject, message)
}

func (e *asyncEventsNats) PublishSessionMessage(sessionId string, backend *Backend, message *AsyncMessage) error {
	subject := GetSubjectForSessionId(sessionId, backend)
	return e.publish(subject, message)
}
