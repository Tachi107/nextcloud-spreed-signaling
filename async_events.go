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

type AsyncBackendRoomEventListener interface {
	ProcessBackendRoomRequest(request *BackendServerRoomRequest)
}

type AsyncRoomEventListener interface {
	ProcessAsyncRoomMessage(message *AsyncMessage)
}

type AsyncUserEventListener interface {
	ProcessAsyncUserMessage(message *AsyncMessage)
}

type AsyncSessionEventListener interface {
	ProcessAsyncSessionMessage(message *AsyncMessage)
}

type AsyncEvents interface {
	Close()

	RegisterBackendRoomListener(roomId string, backend *Backend, listener AsyncBackendRoomEventListener) error
	UnregisterBackendRoomListener(roomId string, backend *Backend, listener AsyncBackendRoomEventListener)

	RegisterRoomListener(roomId string, backend *Backend, listener AsyncRoomEventListener) error
	UnregisterRoomListener(roomId string, backend *Backend, listener AsyncRoomEventListener)

	RegisterUserListener(userId string, backend *Backend, listener AsyncUserEventListener) error
	UnregisterUserListener(userId string, backend *Backend, listener AsyncUserEventListener)

	RegisterSessionListener(sessionId string, backend *Backend, listener AsyncSessionEventListener) error
	UnregisterSessionListener(sessionId string, backend *Backend, listener AsyncSessionEventListener)

	PublishBackendRoomMessage(roomId string, backend *Backend, message *AsyncMessage) error
	PublishRoomMessage(roomId string, backend *Backend, message *AsyncMessage) error
	PublishUserMessage(userId string, backend *Backend, message *AsyncMessage) error
	PublishSessionMessage(sessionId string, backend *Backend, message *AsyncMessage) error
}
