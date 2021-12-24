Simple chat module.

session["id"] and session["username"] used to identificate User.

# Routes:

1. `/chat` index page of chat (rendering chat template)

2. `/chat/join` init point for register js client in chat (return chat sessionID)

3. `/chat/udate?session=` provide all chat events for this session

4. `/chat/send?session=` pass client messages (mesage itself or operation info) to chat message processor

5. `/chat/close?session=` cancel client chat session

 
# Shared part of chat app:

 ## shared/chat.go

 Initial setup of params and message processor

 ## shared/message.go

 Message type and templates for basic messages

 ## shared/room.go

 Room type and logic of room creation, joining and leave

 ## shared/user.go

 User type and logic of user handling (create, get), message throttling.
 
 User created as 1 per gowebapp user (session["id"]) and can contain many real sessions.

 Throttling watched per user to avoid separate session message burst.

 ## shared/session.go

 Session type and logic

 Session represent User separate connection (for different devices or browser tabs for example)

 ## shared/utils.go

 Helper to generate radom strings

# Events on client side

## ondisconnected

Raised when fetch can't complete request (network error or huge timeout). Client trying to recoonect fixed times and after raise error;

## onmessages

Raised on every typed message from server (more for debugging porpose)

## onRooms

Raised on `rooms` message type. Body contain public room list from server (this list is predifined in shared/chat.go)

## onRoomJoin

Raised on `room.join` message type. From part contain info of joined user, To part contain room info.


## onRoomLeave

Same as previous but for `room.leave` message type


## onRoomUsers

Raised on `room.users` message type. Body return all users in room from From part

## onRoomMessage

Raised on `room.message` message type. Basic text message in room from user

## onPrivateCreated

Raised on `private.created` message type as response to init private room creation from user

## onPrivateInvite

Raised on `private.invite` message type when user invited to new private room

