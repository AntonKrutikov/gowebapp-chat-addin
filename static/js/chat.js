const UPDATE_POLLING_TIMEOUT = 100 // 0.1 sec (not bad to make smaller)
const HEARTBEAT_TIMEOUT = 30 * 1000 // must be lower then server timeout
const RECONNECT_ATTEMPT_TIMEOUT = 5 * 1000
const MAX_RECONNECT_ATTEMPTS = 3

class Chat {
    user; // user info
    rooms = []; // public rooms (default)
    constructor() {
        this.api = new ChatApi()
        this.gui = new ChatGUI()

        this.gui.tab.USE_COLORS = true
    }
    async init() {
        this.user = await this.api.join()
        this.api.update(this.user.session)

        this.gui.init()

        this.api.ondisconnected = (status) => {
            // If no events from server side and server close network connection (not session)
            // Try to reconnect with same session
            clearTimeout(this.api.heartbeat_timeout)
            this.api.reconnect_attempts++
            if (this.api.reconnect_attempts <= MAX_RECONNECT_ATTEMPTS) {
                if (status === undefined) {
                    setTimeout(() => this.api.update(this.user.session), RECONNECT_ATTEMPT_TIMEOUT)
                }
            }
        }

        this.api.onRooms = (m) => {
            let rooms = m.body
            rooms.forEach(room => this.gui.rooms.add(room))
        }

        this.api.onRoomMessage = (m) => {
            let room = m.to
            this.gui.tab.chat.add_message(room, m)
        }

        this.gui.onRoomListRowClick = (room) => {
            this.api.joinRoom(room) //TODO: not send if already joined on client
            this.gui.tab.add(room)
            this.api.roomUsers(room)

        }

        this.api.onRoomJoin = (m) => {
            let room = m.body
            if (m.from.id == this.user.id) {
                // user self join resonse
                let already = this.user.rooms.find(r => r.name == room.name)
                if (already === undefined) {
                    this.user.rooms.push(room)
                }
            }
            this.gui.tab.chat.add_user(room, m.from)
        }

        this.api.onRoomLeave = (m) => {
            let room = m.body
            this.gui.tab.chat.remove_user(room, m.from)
            if (room.type == 'private') {
                this.gui.tab.chat.add_system_message(room, `${m.from.name} leave`)
            }
        }

        this.api.onRoomUsers = (m) => {
            let users = m.body
            this.gui.tab.chat.refresh_users(m.from, users)
        }

        this.gui.onRoomTabClosed = (room) => {
            this.api.leaveRoom(room)
        }

        this.gui.onSendText = (room, text) => {
            this.api.sendText(room, text)
        }

        this.gui.onRequestPrivate = (user) => {
            this.gui.tab.add(user)
            this.api.requestPrivate(user)
        }

        this.api.onPrivateInvite = (m) => {
            this.gui.tab.add(m.from, false)
            this.gui.tab.header.update_room_name(m.from, m.body)
            this.gui.tab.chat.update_room_name(m.from, m.body)
        }

        this.api.onPrivateCreated = (m) => {
            this.gui.tab.header.update_room_name(m.from, m.body)
            this.gui.tab.chat.update_room_name(m.from, m.body)
        }

        this.api.getRooms()
    }
}

class ChatApi {
    endpoint = {
        join: '/chat/join',
        update: '/chat/update',
        send: '/chat/send',
        close: '/chat/close'
    };
    session;
    status = {
        disconnected: 599 // 599 http code when server kill session
    }
    timeout; // store setTimeout result
    abort; // store AbortController
    reconnect_attempts = 0;
    heartbeat_timeout;

    ondisconnected; // callback
    onmessages; // callback, message array as param

    onRooms; // public room list
    onRoomJoin; // new user join active room
    onRoomLeave; // user left room
    onRoomUsers; // all users in room
    onRoomMessage; //
    onPrivateInvite;
    onPrivateCreated;
    onPrivateMessage;


    async join() {
        try {
            let response = await fetch(this.endpoint.join)
            let user_response = await response.json()
            let user = new User(
                user_response.id,
                user_response.name,
                user_response.session
            )

            this.session = user_response.session

            // register unload call
            // send request to /close and no wait response
            window.addEventListener('beforeunload', () => {
                if (this.session) {
                    fetch(`${this.endpoint.close}?session=${this.session}`)
                }
            })

            return user
        } catch (err) {
            console.log(err)
        }
    }

    async update() {
        if (this.timeout) clearTimeout(this.timeout)
        this.abort = new AbortController()

        try {
            let response = await fetch(`${this.endpoint.update}?session=${this.session}`)

            if (response.status == 200) {
                this.reconnect_attempts = 0
                this.heartbeat()
            }

            if (response.status != 200 && this.ondisconnected) {
                this.ondisconnected(response.status)
                return
            }

            let messages = await response.json()
            if (this.onmessages) this.onmessages(messages)

            messages.forEach(m => this.process(m))

            this.timeout = setTimeout(() => { this.update() }, UPDATE_POLLING_TIMEOUT)
        } catch (err) {
            console.log(err)
            if (err.name == 'TypeError' && this.ondisconnected) this.ondisconnected()
        }
    }

    async send(message) {
        try {
            let response = await fetch(
                `${this.endpoint.send}?session=${this.session}`,
                {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify(message)
                })
            let result = await response.json()
            if (response.status != 200) {
                console.log('POST error:', result)
            }
        } catch (err) {
            console.log(err)
        }
    }

    heartbeat() {
        clearTimeout(this.heartbeat_timeout)
        this.send({
            type: 'heartbeat'
        })
        this.heartbeat_timeout = setTimeout(() => { this.heartbeat() }, HEARTBEAT_TIMEOUT)
    }

    process(message) {
        console.log(message)
        if (message.body) {
            try {
                message.body = JSON.parse(message.body)
            } catch { }
        }

        switch (message.type) {
            case 'rooms':
                if (this.onRooms) this.onRooms(message)
                break
            case 'room.join':
                if (this.onRoomJoin) this.onRoomJoin(message)
                break
            case 'room.leave':
                if (this.onRoomLeave) this.onRoomLeave(message)
                break
            case 'room.users':
                if (this.onRoomUsers) this.onRoomUsers(message)
                break
            case 'room.message':
                if (this.onRoomMessage) this.onRoomMessage(message)
                break
            case 'private.created':
                if (this.onPrivateCreated) this.onPrivateCreated(message)
                break
            case 'private.invite':
                if (this.onPrivateInvite) this.onPrivateInvite(message)
                break
        }
    }

    getRooms() {
        this.send({
            type: 'rooms'
        })
    }

    joinRoom(room) {
        this.send({
            type: 'room.join',
            body: room.name // TODO: pass object with id and name
        })
    }

    leaveRoom(room) {
        this.send({
            type: 'room.leave',
            body: room.name
        })
    }

    roomUsers(room) {
        this.send({
            type: 'room.users',
            body: room.name
        })
    }

    sendText(room, text) {
        let mtype = 'room.message'
        this.send({
            to: room,
            type: mtype,
            body: text
        })
    }

    requestPrivate(user) {
        this.send({
            type: 'private',
            to: user
        })
    }
}

class User {
    id;
    name;
    session;
    rooms = [];

    constructor(id, name, session) {
        this.id = id
        this.name = name
        this.session = session
    }
}

class ChatGUI {
    onRoomListRowClick;
    onRoomTabClosed;
    onSendText;
    onRequestPrivate;

    container = document.createElement('div')
    rooms = {
        ROOM_COLORS: ['#81D4FA', '#81C784', '#F06292', '#A1887F', '#90A4AE', '#E57373'],
        list: document.createElement('div'),
        init(root) {
            this.root = root
            this.list.classList.add('chat-room-list')
            return this.list
        },
        row(room) {
            let row = document.createElement('div')
            let icon = document.createElement('div')
            let text = document.createElement('span')

            row.classList.add('chat-room-list-row')
            icon.classList.add('chat-room-icon')
            icon.style.background = this.ROOM_COLORS.pop() //TODO: temp
            text.innerText = room.name

            row.appendChild(icon)
            row.appendChild(text)
            row.dataset.name = room.name
            row.dataset.id = room.id
            row.addEventListener('click', () => {
                if (this.root.onRoomListRowClick) this.root.onRoomListRowClick(room)
            })
            return row
        },
        add(room) {
            this.list.querySelectorAll('.chat-room-list-row').forEach(n => {
                if (n.dataset.name == room.name)
                    return
            })
            this.list.appendChild(this.row(room))
        }
    }

    // tab represent each chat instance per room
    tab = {
        USE_COLORS: false,
        USER_COLORS: ['#81D4FA', '#81C784', '#F06292', '#A1887F', '#90A4AE', '#E57373'],
        user_color_map: {},

        container: document.createElement('div'),
        init(root) {
            this.root = root

            this.container.classList.add('chat-tabs-container')
            this.container.appendChild(this.header.container)
            this.header.init(this)
            this.chat.init(this)
            return this.container
        },
        add(room, activate = true) {
            this.header.add(room)
            this.chat.add(room)
            if (activate === true) this.make_active(room)
        },
        close(room) {
            if (this.root.onRoomTabClosed) this.root.onRoomTabClosed(room)
            this.header.remove(room)
            this.chat.remove(room)
        },
        make_active(room) {
            this.header.make_active(room)
            this.chat.make_active(room)
        },
        send(room, text) {
            if (this.root.onSendText) this.root.onSendText(room, text)
        },
        request_private(user) {
            if (this.root.onRequestPrivate) this.root.onRequestPrivate(user)
        },
        header: {
            container: document.createElement('div'),
            init(tab) {
                this.tab = tab

                this.container_class = 'chat-tabs-header-container'

                this.container.classList.add(this.container_class)
                return this.container
            },
            add(room) {
                if (this.is_opened(room)) return

                let item = document.createElement('div')
                let text = document.createElement('span')
                let close = document.createElement('span')

                item.classList.add('chat-tab-header')
                text.innerText = room.name
                close.classList.add('chat-tab-header-close')
                close.innerText = 'x'

                item.dataset.name = room.name
                item.dataset.id = room.id

                item.appendChild(text)
                item.appendChild(close)

                this.container.appendChild(item)

                item.addEventListener('click', () => {
                    this.tab.make_active({ id: item.dataset.id, name: item.dataset.name })
                })

                close.addEventListener('click', () => {
                    this.tab.close({ id: item.dataset.id, name: item.dataset.name })
                })
            },
            remove(room) {
                [...this.container.children].forEach(i => {
                    if (i.dataset.name == room.name) {
                        this.container.removeChild(i)
                    }
                })
            },
            update_room_name(room, updated) {
                let target = this.container.querySelector(`.chat-tab-header[data-name='${room.name}']`)
                if (target) target.dataset.name = updated.name
            },
            make_active(room) {
                [...this.container.children].forEach(i => {
                    i.dataset.active = false
                    i.classList.remove('chat-tab-header-active')
                    if (i.dataset.id == room.id) {
                        i.dataset.active = true
                        i.classList.add('chat-tab-header-active')
                    }
                })
            },
            is_opened(room) {
                let opened = false;
                [...this.container.children].forEach(i => {
                    if (i.dataset.id == room.id || i.dataset.name == room.name) {
                        opened = true
                    }
                })

                return opened
            },
            is_active(room) {
                let active = false;
                [...this.container.children].forEach(i => {
                    if (i.dataset.name == room.name && i.dataset.active == true) {
                        active = true
                    }
                })
                return active
            }
        },
        chat: {
            container: document.createElement('div'),
            input: document.createElement('textarea'),
            inner: document.createElement('div'),
            users: document.createElement('div'),
            init(tab) {
                this.tab = tab

                this.container_class = 'chat-tab-inner-container'
                this.inner_class = 'chat-chat-inner'
                this.users_class = 'chat-user-list'

                this.container.classList.add(this.container_class)
                this.input.classList.add('chat-input')
                this.inner.classList.add(this.inner_class)
                this.users.classList.add(this.users_class)
            },
            add(room) {
                if (this.is_opened(room)) return

                let container = this.container.cloneNode()
                let inner = this.inner.cloneNode()
                let input = this.input.cloneNode()
                let users = this.users.cloneNode()

                container.dataset.id = room.id
                container.dataset.name = room.name

                inner.dataset.id = room.id
                inner.dataset.name = room.name

                input.dataset.id = room.id
                input.dataset.name = room.name

                users.dataset.id = room.id
                users.dataset.name = room.name

                container.appendChild(inner)
                container.appendChild(input)
                container.appendChild(users)

                input.addEventListener('keypress', (e) => {
                    if (e.key == "Enter") {
                        e.preventDefault()
                        let msg = e.target.value
                        this.tab.send({ id: container.dataset.id, name: container.dataset.name }, msg)
                        e.target.value = ''
                    }
                })

                container.dataset.active = false
                container.style.display = 'none'
                this.tab.container.appendChild(container) //TODO: not good to modify parent
            },
            remove(room) {
                this.tab.container.querySelectorAll('.' + this.container_class).forEach(i => {
                    if (i.dataset.name == room.name) {
                        this.tab.container.removeChild(i)
                    }
                })
            },
            add_message(room, message) {
                let row = document.createElement('div')
                let from = document.createElement('b')
                let text = document.createElement('span')

                row.classList.add('chat-message')

                from.innerText = message?.from?.name
                text.innerText = message?.body

                if (this.tab.USE_COLORS == true) {
                    let name = message?.from?.name
                    if (this.tab.user_color_map[name] === undefined) {
                        this.tab.user_color_map[name] = this.tab.USER_COLORS.pop()
                    }
                    from.style.color = this.tab.user_color_map[name]
                }

                row.appendChild(from)
                row.appendChild(text)

                let target = this.tab.container.querySelector(`.${this.container_class}[data-name='${room.name}']`)
                let inner = target?.querySelector('.' + this.inner_class)
                inner?.prepend(row)
            },
            add_system_message(room, text) {
                let row = document.createElement('div')
                row.classList.add('chat-system-message')

                row.innerText = text

                let target = this.tab.container.querySelector(`.${this.container_class}[data-name='${room.name}']`)
                let inner = target?.querySelector('.' + this.inner_class)
                inner?.prepend(row)
            },
            add_user(room, user) {
                let exists = this.tab.container.querySelector(`.${this.users_class}[data-name='${room.name}'] .chat-user-list-row[data-id='${user.id}']`)
                if (exists) {
                    return
                }

                let row = document.createElement('div')
                let text = document.createElement('span')

                row.classList.add('chat-user-list-row')

                text.innerText = user.name

                row.dataset.id = user.id
                row.dataset.name = user.name

                row.addEventListener('click', () => {
                    this.tab.request_private(user)
                })

                row.appendChild(text)

                let target = this.tab.container.querySelector(`.${this.users_class}[data-name='${room.name}']`)
                target.appendChild(row)
            },
            remove_user(room, user) {
                let exists = this.tab.container.querySelector(`.${this.users_class}[data-name='${room.name}'] .chat-user-list-row[data-id='${user.id}']`)
                if (exists) {
                    let users = this.tab.container.querySelector(`.${this.users_class}[data-name='${room.name}']`)
                    users.removeChild(exists)
                }
            },
            refresh_users(room, users) {
                users.forEach(u => {
                    this.add_user(room, u)
                })
            },
            make_active(room) {
                this.tab.container.querySelectorAll('.' + this.container_class).forEach(i => {
                    i.dataset.active = false
                    i.style.display = 'none'
                    if (i.dataset.id == room.id) {
                        i.dataset.active = true
                        i.style.display = null
                    }
                })
            },
            is_opened(room) {
                let opened = false;
                this.tab.container.querySelectorAll('.' + this.container_class).forEach(i => {
                    if (i.dataset.id == room.id || i.dataset.name == room.name) {
                        opened = true
                    }
                })
                return opened
            },
            is_active(room) {
                let active = false;
                this.tab.container.querySelectorAll('.' + this.container_class).forEach(i => {
                    if (i.dataset.name == room.name && i.dataset.active == true) {
                        active = true
                    }
                })
                return active
            },
            update_room_name(room, updated) {
                let target = this.tab.container.querySelector(`.${this.container_class}[data-name='${room.name}']`)
                if (target) target.dataset.name = updated.name
            }
        }
    }


    init() {
        this.container.classList.add('chat-container')

        this.container.appendChild(this.rooms.init(this))
        this.container.appendChild(this.tab.init(this))

        document.body.appendChild(this.container)
    }
}

let chat = new Chat()
chat.init()