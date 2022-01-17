// support emoji with external lib
import { EmojiButton } from '/static/js/emoji-button.min.js'

const UPDATE_POLLING_TIMEOUT = 500 // 0.1 sec (not bad to make smaller)
const HEARTBEAT_TIMEOUT = 15 * 1000 // must be lower then server timeout (server_value/2 - is ok)
const RECONNECT_ATTEMPT_TIMEOUT = 5 * 1000
const MAX_RECONNECT_ATTEMPTS = 3

const DEFAULT_ROOM_NAME = 'default'

const DEBUG = true //show all messages in console

//const MAX_CHAT_HISTORY_LENGTH = 1000 //Limit length of tab history (not requrired)

const TEXTAREA_PLACEHOLDER = 'پیام شما'
const ADD_ROOM_PLACEHOLDER = 'room name'

// This colors used one by one fro room colors and users colors if options enabled: this.gui.tab.USE_COLORS and this.gui.rooms.USE_COLORS
const COLORS = ['#E57373', '#81D4FA', '#81C784', '#F06292', '#A1887F', '#90A4AE', '#FF8F00', '#43A047']

class Chat {
    user; // user info
    rooms = []; // public rooms (default)
    constructor() {
        this.api = new ChatApi()
        this.gui = new ChatGUI()

        this.gui.tab.USE_COLORS = true
        this.gui.rooms.USE_COLORS = true
    }
    async init() {
        this.user = await this.api.join()        
        this.gui.user = this.user
        this.api.update(this.user.session)

        this.gui.init()

        this.api.ondisconnected = (status) => {
            // Try to reconnect some times
            this.gui.popup.show('Connection lost. Reconnecting.', false)
            clearTimeout(this.api.heartbeat_timeout)
            this.api.reconnect_attempts++
            if (this.api.reconnect_attempts <= MAX_RECONNECT_ATTEMPTS) {
                setTimeout(async () => {
                    try {
                        let user = await this.api.join()
                        this.user.session = user.session
                        this.api.update(this.user.session)
                        this.api.getRooms()
                        this.gui.popup.hide()
                    } catch (err) {
                        this.api.ondisconnected()
                    }
                }, RECONNECT_ATTEMPT_TIMEOUT)
            }
        }

        this.api.onRoomList = (m) => {
            let rooms = m.body
            this.rooms = rooms
            rooms.forEach(room => {
                this.gui.rooms.add(room)
                this.gui.tab.modify_id_by_name(room) //used then recoonect
                if (room.name == DEFAULT_ROOM_NAME) {
                    this.api.joinRoom(room)
                }
            })

        }

        this.api.onRoomMessage = (m) => {
            let room = m.to
            if (m.from.id == this.user.id) {
                this.gui.tab.chat.add_message(room, m, "end")
                this.gui.tab.chat.scroll_to_bottom(room, true) //always if user send self message
            } else {
                this.gui.tab.chat.add_message(room, m)
                this.gui.tab.chat.scroll_to_bottom(room) // if user near 100 from bottom
            }
        }

        this.gui.onRoomListRowClick = (room) => {
            if (room.type == 'public') {
                this.api.joinRoom(room)
            } else if (room.type == 'private') {
                this.gui.onRequestPrivate(room)
            }
            this.gui.tab.make_active(room)
            this.gui.tab.container.classList.remove('chat-hide')
            this.gui.rooms.list.classList.add('chat-hide')
        }

        this.api.onRoomJoin = (m) => {
            // Self join response (with history). Body contain room and history
            if (m.to.id == this.user.id) {
                let room = m.body.room
                let history = m.body.history
                let already = this.user.rooms.find(r => r.name == room.name)
                if (already === undefined) {
                    this.user.rooms.push(room)
                }
                this.gui.tab.add(room)
                this.gui.rooms.enter(room)
                this.api.roomUsers(room)
                this.gui.tab.chat.add_user(room, m.from)
                this.gui.rooms.list.classList.add('chat-hide')
                this.gui.tab.container.classList.remove('chat-hide')
                history.forEach(m => {
                    if (m.from.id == this.user.id) {
                        this.gui.tab.chat.add_message(room, m, 'flex-end')
                    } else {
                        this.gui.tab.chat.add_message(room, m)
                    }
                })
            } else {
                let room = m.body
                this.gui.tab.chat.add_user(room, m.from)
                this.gui.tab.chat.add_system_message(m.to, `${m.from.name} joined`)
            }

        }

        this.api.onRoomLeave = (m) => {
            let room = m.body
            this.gui.tab.chat.remove_user(room, m.from)
            this.gui.tab.chat.add_system_message(room, `${m.from.name} leave`)
        }

        this.api.onRoomUsers = (m) => {
            let users = m.body
            this.gui.tab.chat.refresh_users(m.from, users)
        }

        this.gui.onRoomTabClosed = (room) => {
            this.api.leaveRoom(room)
            let index = this.user.rooms.findIndex(r => r.name == room.name)
            if (index !== -1) {
                this.user.rooms.splice(index, 1)
            }
            this.gui.rooms.leave(room)
            this.gui.rooms.list.classList.remove('chat-hide')
        }

        this.gui.onSendText = (room, text) => {
            if (room.type == 'public') {
                this.api.sendText(room, text)
            } else if (room.type == 'private') {
                this.api.sendPrivateMessage(room, text)
            }
        }

        this.api.onPrivateMessage = (m) => {
            let room = m.from
            room.type = 'private'
            this.gui.rooms.add(room, false)
            this.gui.tab.add(room, false)
            this.gui.tab.chat.add_message(room, m)
        }

        this.api.onPrivateDelivered = (m) => {
            let room = m.from
            m.from = m.to
            room.type = 'private'
            this.gui.tab.chat.add_message(room, m, 'flex-end')
        }

        this.gui.onRequestPrivate = (user) => {
            user.type = 'private'
            this.gui.rooms.add(user, false)
            this.gui.tab.add(user, true)
        }

        // Mute/unmute user
        this.gui.onMute = (user) => {
            this.api.muteUser(user)
        }

        this.api.onMuted = (m) => {
            let user = m.from
            user.muted = true
            this.gui.tab.chat.update_mute_state(user)
        }

        this.gui.onUnmute = (user) => {
            this.api.unmuteUser(user)
        }

        this.api.onUnmuted = (m) => {
            let user = m.from
            user.muted = false
            this.gui.tab.chat.update_mute_state(user)
        }

        this.gui.onCreateRoom = (room) => {
            this.api.createRoom(room)
            this.user.wait_room_created.push(room)
        }

        this.api.onNewRoom = (m) => {
            m.from.type = 'public'
            let index = this.user.wait_room_created.findIndex(r => r.name == m.from.name)
            if (index !== -1) {
                this.gui.rooms.add(m.from, true)
                this.user.wait_room_created.splice(index, 1)
                this.api.joinRoom(m.from)
            } else {
                this.gui.rooms.add(m.from)
            }
        }

        this.api.onRoomDelete = (m) => {
            this.gui.rooms.remove(m.from)
        }

        this.api.onThrottling = (m) => {
            this.gui.tab.chat.add_system_message(m.from, m.body)
        }

        this.api.onRoomFull = (m) => {
            this.gui.popup.show(m.body)
        }

        this.api.onRoomMaxCount = (m) => {
            this.gui.popup.show(m.body)
        }

        this.api.onRoomBadName = (m) => {
            this.gui.popup.show(m.body)
        }

        this.api.onRoomAlreadyExists = (m) => {
            this.gui.popup.show(m.body)
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

    onRoomList; // public room list
    onNewRoom; // new room was created by someone
    onRoomDelete; // all users leave non permanent room and it was removed
    onRoomJoin; // new user join active room
    onRoomLeave; // user left room
    onRoomUsers; // all users in room
    onRoomMessage; // new message in room
    onPrivateMessage; //new private message
    onThrottling; // to fast sending - bad
    onRoomFull; // no free space - show error, can't join
    onRoomMaxCount; // can't create more rooms
    onRoomBadName; // room name depricated
    onRoomAlreadyExists; // this room already exists
    onMuted; //user muted
    onUnmuted; //user unmuted


    async join() {
        try {
            let response = await fetch(this.endpoint.join)

            if (response.status != 200 && this.ondisconnected) {
                this.ondisconnected(response.status)
                return
            }

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

        this.heartbeat_timeout = setTimeout(() => {
            this.send({
                type: 'heartbeat'
            })
            this.heartbeat()
        }, HEARTBEAT_TIMEOUT)
    }

    process(message) {
        if (DEBUG === true) console.log(message)
        if (message.body) {
            try {
                message.body = JSON.parse(message.body)
            } catch { }
        }

        switch (message.type) {
            case 'room.list':
                if (this.onRoomList) this.onRoomList(message)
                break
            case 'room.created':
                if (this.onNewRoom) this.onNewRoom(message)
                break
            case 'room.deleted':
                if (this.onRoomDelete) this.onRoomDelete(message)
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
            case 'private.message':
                if (this.onPrivateMessage) this.onPrivateMessage(message)
                break
            case 'private.delivered':
                if (this.onPrivateDelivered) this.onPrivateDelivered(message)
                break
            case 'to_many_requests':
                if (this.onThrottling) this.onThrottling(message)
                break
            case 'room.full':
                if (this.onRoomFull) this.onRoomFull(message)
                break
            case 'room.max_count':
                if (this.onRoomMaxCount) this.onRoomMaxCount(message)
                break
            case 'room.bad_name':
                if (this.onRoomBadName) this.onRoomBadName(message)
                break
            case 'room.already_exists':
                if (this.onRoomAlreadyExists) this.onRoomAlreadyExists(message)
                break
            case 'muted':
                if (this.onMuted) this.onMuted(message)
                break
            case 'unmuted':
                if (this.onUnmuted) this.onUnmuted(message)
                break
        }
    }

    getRooms() {
        this.send({
            type: 'rooms'
        })
    }

    createRoom(room) {
        this.send({
            type: 'room.create',
            body: room.name,
            to: {
                id: room.id,
                name: room.name
            }
        })
    }

    joinRoom(room) {
        this.send({
            type: 'room.join',
            body: room.name, // This from first version (server join use 'to' data)
            to: {
                id: room.id,
                name: room.name
            }
        })
    }

    leaveRoom(room) {
        this.send({
            type: 'room.leave',
            body: room.name,
            to: {
                id: room.id,
                name: room.name
            }
        })
    }

    roomUsers(room) {
        this.send({
            type: 'room.users',
            body: room.name,
            to: {
                id: room.id,
                name: room.name
            }
        })
    }

    sendText(room, text) {
        this.send({
            to: room,
            type: 'room.message',
            body: text
        })
    }

    sendPrivateMessage(room, text) {
        this.send({
            to: room,
            type: 'private.message',
            body: text
        })
    }

    requestPrivate(user) {
        this.send({
            type: 'private',
            to: user
        })
    }

    muteUser(user) {
        this.send({
            type: 'mute',
            to: user
        })
    }

    unmuteUser(user) {
        this.send({
            type: 'unmute',
            to: user
        })
    }
}

class User {
    id;
    name;
    session;
    rooms = [];
    wait_room_created = [];

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
    onCreateRoom;
    onMute;
    onUnmute;

    container = document.createElement('div')
    rooms = {
        USE_COLORS: true,
        last_color_index: 0,
        list: document.createElement('div'),
        init(root) {
            this.root = root
            this.list.classList.add('chat-room-list')
            this.list.appendChild(this.create_area())
            return this.list
        },
        row(room, alias) {
            let row = document.createElement('div')
            let icon = document.createElement('div')
            let name = document.createElement('span')
            let last_message = document.createElement('div')
            let last_message_from = document.createElement('b')
            let last_message_text = document.createElement('span')
            let indicator = document.createElement('div')

            row.classList.add('chat-room-list-row')
            icon.classList.add('chat-room-icon')
            last_message.classList.add('chat-room-last-message')
            last_message_from.classList.add('chat-room-last-message-from')
            last_message_text.classList.add('chat-room-last-message-text')
            indicator.classList.add('chat-room-join-indicator')

            if (room.permanent == true) {
                row.classList.add('chat-room-list-row-permanent')
            }

            if (this.USE_COLORS) {
                icon.style.background = COLORS[this.last_color_index]
                this.last_color_index = this.last_color_index == COLORS.length - 1 ? 0 : this.last_color_index + 1
            }
            icon.innerText = alias ? alias.slice(1, 2).toUpperCase() : room.name.slice(0, 1).toUpperCase()
            name.innerText = alias ?? room.name
            name.title = alias ?? room.name

            if (room.type == 'public') {
                last_message.appendChild(last_message_from)
            }
            last_message.appendChild(last_message_text)

            row.appendChild(icon)
            row.appendChild(name)
            row.appendChild(last_message)
            row.appendChild(indicator)
            row.dataset.name = room.name
            row.dataset.id = room.id
            row.dataset.type = room.type
            row.dataset.alias = alias
            row.dataset.permanent = room.permanent
            row.addEventListener('click', () => {
                let room = {
                    id: row.dataset.id,
                    name: row.dataset.name,
                    type: row.dataset.type,
                    alias: row.dataset.alias
                }
                if (this.root.onRoomListRowClick) this.root.onRoomListRowClick(room)
            })
            return row
        },
        add(room, first = false, alias) {
            let exists = false
            this.list.querySelectorAll('.chat-room-list-row').forEach(n => {
                if (n.dataset.name == room.name) {
                    n.dataset.id = room.id //used in reconnect
                    exists = true
                }
            })
            if (exists) return

            let new_room_area = this.list.querySelector('.chat-room-add-container')
            let permanent = this.list.querySelectorAll('.chat-room-list-row[data-permanent="true"]')
            let last_permanent = permanent.length > 0 ? permanent[permanent.length - 1] : null

            if (last_permanent && first == true) {
                let row = this.row(room, alias)
                row.classList.add('animation_appear')
                last_permanent.after(row)
            } else if (new_room_area && first == true) {
                let row = this.row(room, alias)
                new_room_area.after(row)
            } else {
                this.list.appendChild(this.row(room, alias))
            }
        },
        add_last_message(room, from, text) {
            let target = this.list.querySelector(`.chat-room-list-row[data-id='${room.id}']`)
            if (target) {
                let f = target.querySelector('.chat-room-last-message-from')
                if (f) f.innerText = from
                let t = target.querySelector('.chat-room-last-message-text')
                if (t) t.innerText = text
            }
        },
        remove(room) {
            let target = this.list.querySelector(`.chat-room-list-row[data-id='${room.id}']`)
            if (target) {
                this.list.removeChild(target)
            }
        },
        enter(room) {
            let target = this.list.querySelector(`.chat-room-list-row[data-id='${room.id}'] .chat-room-join-indicator`)
            if (target) {
                target.style.display = 'block'
            }
        },
        leave(room) {
            let target = this.list.querySelector(`.chat-room-list-row[data-id='${room.id}']`)
            if (target) {
                let indicator = target.querySelector('.chat-room-join-indicator')
                if (indicator) indicator.style.display = 'none'
                let last_from = target.querySelector('.chat-room-last-message-from')
                if (last_from) last_from.innerText = ''
                let last_message = target.querySelector('.chat-room-last-message-text')
                if (last_message) last_message.innerText = ''
            }
        },
        create_area() {
            let container = document.createElement('div')
            let inner = document.createElement('div')
            let add_button = document.createElement('div')
            let input = document.createElement('input')
            let button = document.createElement('div')

            container.classList.add('chat-room-add-container')
            inner.classList.add('chat-room-add-container-inner')
            add_button.classList.add('chat-room-add-container-add-button')
            input.classList.add('chat-room-add-input')
            button.classList.add('chat-room-add-button')

            add_button.innerText = "+"
            button.innerText = "→"
            input.placeholder = ADD_ROOM_PLACEHOLDER
            input.maxLength = 128

            inner.appendChild(input)
            inner.appendChild(button)
            container.appendChild(add_button)
            container.appendChild(inner)

            add_button.addEventListener('click', () => {
                let d = window.getComputedStyle(inner).display
                if (d == 'none') {
                    inner.style.display = 'flex'
                    add_button.innerText = '-'
                } else {
                    inner.style.display = 'none'
                    add_button.innerText = '+'
                }
            })

            button.addEventListener('click', () => {
                let room = input.value
                if (this.root.onCreateRoom) this.root.onCreateRoom({ name: room, id: room })
                input.value = ''
                inner.style.display = 'none'
                add_button.innerText = '+'
            })

            input.addEventListener('keypress', (e) => {
                if (e.key == "Enter") {
                    e.preventDefault()
                    let room = input.value
                    if (this.root.onCreateRoom) this.root.onCreateRoom({ name: room, id: room })
                    input.value = ''
                    input.blur()
                    inner.style.display = 'none'
                    add_button.innerText = '+'
                }
            })

            return container
        }
    }

    // tab represent each chat instance per room
    tab = {
        USE_COLORS: false,
        last_color_index: 0,
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
        add(room, activate = true, alias) {
            this.header.add(room, alias)
            this.chat.add(room)
            if (activate === true) this.make_active(room)
        },
        close(room) {
            if (this.root.onRoomTabClosed) this.root.onRoomTabClosed(room)
            this.header.remove(room)
            this.chat.remove(room)
        },
        modify_id_by_name(room) {
            this.header.modify_id_by_name(room)
            this.chat.modify_id_by_name(room)
        },
        make_active(room) {
            this.header.make_active(room)
            this.chat.make_active(room)
            this.root.rooms.list.classList.add('chat-hide') //mobile fix

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
            add(room, alias) {
                if (this.is_opened(room)) return

                let menu_icon = document.createElement('div')
                let item = document.createElement('div')
                let text = document.createElement('span')
                let close = document.createElement('span')

                item.classList.add('chat-tab-header')
                text.classList.add('chat-tab-header-title')
                close.classList.add('chat-tab-header-close')
                menu_icon.classList.add('chat-tab-header-menu-icon')
                text.innerText = alias ?? room.name
                text.title = alias ?? room.name
                close.innerText = 'x'
                menu_icon.appendChild(document.createElement('div'))
                menu_icon.appendChild(document.createElement('div'))
                menu_icon.appendChild(document.createElement('div'))

                item.dataset.name = room.name
                item.dataset.id = room.id
                item.dataset.type = room.type

                item.appendChild(menu_icon)
                item.appendChild(text)
                item.appendChild(close)

                this.container.appendChild(item)

                menu_icon.addEventListener('click', (e) => {
                    e.stopPropagation()
                    this.tab.root.rooms.list.classList.remove('chat-hide')
                    this.tab.container.classList.add('chat-hide')
                })

                item.addEventListener('click', (e) => {
                    e.stopPropagation()
                    this.tab.make_active({ id: item.dataset.id, name: item.dataset.name })
                    //mobile user list toggle
                    let mql = window.matchMedia('(hover: none) and (orientation: portrait)');
                    if (mql.matches == true) {
                        this.tab.chat.toggle_user_list(room)
                    }
                })

                close.addEventListener('click', (e) => {
                    e.stopPropagation()
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
            modify_id_by_name(room) {
                [...this.container.children].forEach(i => {
                    if (i.dataset.name == room.name) {
                        i.dataset.id = room.id
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
                        this.blink_stop(room)
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
                    if (i.dataset.name == room.name && i.dataset.active == 'true') {
                        active = true
                    }
                })
                return active
            },
            blink_start(room) {
                let target = this.container.querySelector(`.chat-tab-header[data-id='${room.id}'] .chat-tab-header-title`)
                if (target) target.classList.add('blink')
            },
            blink_stop(room) {
                let target = this.container.querySelector(`.chat-tab-header[data-id='${room.id}'] .chat-tab-header-title`)
                if (target) target.classList.remove('blink')
            }
        },
        chat: {
            container: document.createElement('div'),
            input: document.createElement('div'),
            textarea: document.createElement('textarea'),
            emoji: document.createElement('img'),
            send_button: document.createElement('div'),
            inner: document.createElement('div'),
            users: document.createElement('div'),
            users_total: document.createElement('div'),
            init(tab) {
                this.tab = tab

                this.container_class = 'chat-tab-inner-container'
                this.inner_class = 'chat-chat-inner'
                this.users_class = 'chat-user-list'

                this.container.classList.add(this.container_class)
                this.input.classList.add('chat-input')
                this.textarea.classList.add('chat-input-textarea')
                this.emoji.classList.add('chat-input-emoji')
                this.emoji.src = '/static/assets/smile.png'
                this.inner.classList.add(this.inner_class)
                this.users.classList.add(this.users_class)
                this.users.classList.add('chat-hide')
                this.users_total.classList.add('chat-user-list-total')
                this.send_button.classList.add('chat-input-send-button')

                this.textarea.placeholder = TEXTAREA_PLACEHOLDER
            },
            add(room) {
                if (this.is_opened(room)) return

                let container = this.container.cloneNode()
                let inner = this.inner.cloneNode()
                let input = this.input.cloneNode()
                let textarea = this.textarea.cloneNode()
                let emoji = this.emoji.cloneNode()
                let users = this.users.cloneNode()
                let users_total = this.users_total.cloneNode()
                let send_button = this.send_button.cloneNode()

                container.dataset.id = room.id
                container.dataset.name = room.name
                container.dataset.type = room.type

                inner.dataset.id = room.id
                inner.dataset.name = room.name

                input.dataset.id = room.id
                input.dataset.name = room.name

                users.dataset.id = room.id
                users.dataset.name = room.name

                send_button.innerText = '→'
                textarea.rows = 3

                const emojiPicker = new EmojiButton({
                    theme: 'dark', 
                    // position: {
                    //     bottom: 0
                    // },
                    showSearch: false,
                    showPreview: false,
                    rows: 4,
                    autoHide: false
                })

                emojiPicker.on('emoji', selection => {
                    textarea.value += selection.emoji
                  });

                emoji.addEventListener('click', (e) => {
                    emojiPicker.togglePicker(textarea)
                })

                input.appendChild(emoji)
                input.appendChild(textarea)
                input.appendChild(send_button)
                container.appendChild(inner)
                container.appendChild(input)
                users.appendChild(users_total)
                container.appendChild(users)

                textarea.addEventListener('keypress', (e) => {
                    if (e.key == "Enter") {
                        e.preventDefault()
                        let msg = e.target.value
                        this.tab.send({ id: container.dataset.id, name: container.dataset.name, type: container.dataset.type }, msg)
                        e.target.value = ''
                    }
                })

                send_button.addEventListener('click', (e) => {
                    let msg = textarea.value
                    this.tab.send({ id: container.dataset.id, name: container.dataset.name, type: container.dataset.type }, msg)
                    textarea.value = ''
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
            modify_id_by_name(room) {
                this.tab.container.querySelectorAll('.' + this.container_class).forEach(i => {
                    if (i.dataset.name == room.name) {
                        i.dataset.id = room.id
                    }
                })
            },
            add_message(room, message, align_self = 'start') {
                let row = document.createElement('div')
                let from = document.createElement('b')
                let text = document.createElement('span')

                row.classList.add('chat-message')
                row.style.alignSelf = align_self

                from.innerText = message?.from?.name
                text.innerText = message?.body

                if (this.tab.USE_COLORS == true) {
                    let name = message?.from?.name
                    if (this.tab.user_color_map[name] === undefined) {
                        this.tab.user_color_map[name] = COLORS[this.tab.last_color_index]
                    }
                    from.style.color = this.tab.user_color_map[name]
                    this.tab.last_color_index = this.tab.last_color_index == COLORS.length - 1 ? 0 : this.tab.last_color_index + 1
                }

                from.addEventListener('click', () => {
                    this.tab.request_private(message?.from)
                })

                // blink if not active tab
                if (!this.is_active(room)) {
                    this.tab.header.blink_start(room)
                }

                row.appendChild(from)
                row.appendChild(text)

                try {
                    let d = new Date(message?.timestamp)
                    row.title = d.toLocaleTimeString()
                } catch {

                }

                let target = this.tab.container.querySelector(`.${this.container_class}[data-id='${room.id}']`)
                let inner = target?.querySelector('.' + this.inner_class)
                inner?.prepend(row)

                if (typeof MAX_CHAT_HISTORY_LENGTH !== 'undefined' && Number.isInteger(MAX_CHAT_HISTORY_LENGTH)) {
                    this.delete_older_then(room, MAX_CHAT_HISTORY_LENGTH)
                }

                // add last message to room list
                if (target) {
                    let tab_room = {
                        id: target.dataset.id,
                        name: target.dataset.name
                    }
                    this.tab.root.rooms.add_last_message(tab_room, message?.from?.name, message?.body)
                }
            },
            // Sometimes if user scrolled to bottom - browser treat this as non 0 value, we adding bottom windows 0-100 to accurate scrolling on new messages
            scroll_to_bottom(room, always = false) {
                let target = this.tab.container.querySelector(`.${this.inner_class}[data-id='${room.id}']`)
                if (target) {
                    if (always == true || Math.abs(target.scrollTop) < 100) {
                        target.scrollTo({ top: 0, behavior: 'smooth' })
                    }
                }
            },
            delete_older_then(room) {
                let target = this.tab.container.querySelector(`.${this.container_class}[data-id='${room.id}']`)
                let inner = target?.querySelector('.' + this.inner_class)
                if (inner) {
                    let i = 0
                    inner.querySelectorAll('.chat-message, .chat-system-message').forEach(m => {
                        i++
                        if (i > MAX_CHAT_HISTORY_LENGTH) {
                            inner.removeChild(m)
                        }
                    })
                }
            },
            add_system_message(room, text) {
                let row = document.createElement('div')
                row.classList.add('chat-system-message')

                row.innerText = text

                let target = this.tab.container.querySelector(`.${this.container_class}[data-id='${room.id}']`)
                let inner = target?.querySelector('.' + this.inner_class)
                inner?.prepend(row)
                if (typeof MAX_CHAT_HISTORY_LENGTH !== 'undefined' && Number.isInteger(MAX_CHAT_HISTORY_LENGTH)) {
                    this.delete_older_then(room, MAX_CHAT_HISTORY_LENGTH)
                }
            },
            add_user(room, user) {
                let exists = this.tab.container.querySelector(`.${this.users_class}[data-id='${room.id}'] .chat-user-list-row[data-id='${user.id}']`)
                if (exists) {
                    return
                }

                let row = document.createElement('div')
                let text = document.createElement('span')

                row.classList.add('chat-user-list-row')
                text.innerText = user.name

                row.dataset.id = user.id
                row.dataset.name = user.name
                row.title = user.name
                row.appendChild(text)

                // Add ability to start private chat with others and mute others
                if (user.id !== this.tab.root.user.id) {

                    row.addEventListener('click', () => {
                        this.tab.request_private(user)
                    })

                    let mute = document.createElement('img')
                    mute.src = '/static/assets/block.png'
                    mute.classList.add('chat-user-list-row-mute')
                    row.dataset.muted = user.muted ?? false

                    mute.addEventListener('click', (e) => {
                        e.preventDefault()
                        e.stopPropagation()
                        console.log(this.tab.root.user)
                        if (row.dataset.muted == 'false') {
                            this.tab.root.onMute(user)
                        } else {
                            this.tab.root.onUnmute(user)
                        }
                    })
                    row.appendChild(mute)
                }


                let target = this.tab.container.querySelector(`.${this.users_class}[data-id='${room.id}']`)
                if (target) {
                    let total = target.querySelector('.chat-user-list-total')
                    if (total != null) {
                        total.innerText = total.innerText == '' ? 1 : parseInt(total.innerText) + 1
                    }
                    target.appendChild(row)
                }
            },
            remove_user(room, user) {
                let exists = this.tab.container.querySelector(`.${this.users_class}[data-id='${room.id}'] .chat-user-list-row[data-id='${user.id}']`)
                if (exists) {
                    let users = this.tab.container.querySelector(`.${this.users_class}[data-id='${room.id}']`)
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
                    if (i.dataset.name == room.name && i.dataset.active == 'true') {
                        active = true
                    }
                })
                return active
            },
            show_user_list(room) {
                let target = this.tab.container.querySelector(`.${this.users_class}[data-id='${room.id}']`)
                if (target) {
                    target.classList.remove('chat-hide')
                    document.addEventListener('click', (e) => {
                        if (e.target != target) {
                            this.hide_user_list(room)
                        }
                    }, { once: true })
                }
            },
            hide_user_list(room) {
                let target = this.tab.container.querySelector(`.${this.users_class}[data-id='${room.id}']`)
                if (target) target.classList.add('chat-hide')
            },
            toggle_user_list(room) {
                let target = this.tab.container.querySelector(`.${this.users_class}[data-id='${room.id}']`)
                if (target) {
                    if (target.classList.contains('chat-hide')) {
                        this.show_user_list(room)
                    } else {
                        this.hide_user_list(room)
                    }
                }
            },
            update_room_name(room, updated) {
                let target = this.tab.container.querySelector(`.${this.container_class}[data-id='${room.id}']`)
                if (target) target.dataset.name = updated.name
            },
            update_mute_state(user) {
                //Update mute state for all chat tabs
                let user_list = this.tab.container.querySelectorAll(`.chat-user-list-row[data-id='${user.id}']`)
                user_list.forEach(u => {
                    u.dataset.muted = user.muted
                })
            }
        }
    }

    popup = {
        container: document.createElement('div'),
        inner: document.createElement('inner'),
        message: document.createElement('div'),
        close_button: document.createElement('button'),
        init(root) {
            this.root = root
            this.container.classList.add('chat-popup')
            this.inner.classList.add('chat-popup-inner')
            this.message.classList.add('chat-popup-message')
            this.close_button.classList.add('chat-popup-close')

            this.close_button.innerText = "Close"
            this.message.innerText = "Max room count exceed"

            this.inner.appendChild(this.message)
            this.inner.appendChild(this.close_button)

            this.close_button.addEventListener('click', () => {
                this.hide()
            })

            this.hide()

            this.container.appendChild(this.inner)
            root.appendChild(this.container)
        },
        show(msg, close = true) {
            this.close_button.style.display = null
            if (close == false) {
                this.close_button.style.display = 'none'
            }
            if (msg) {
                this.message.innerText = msg
            }
            setTimeout(() => this.close_button.focus(), 100)
            this.container.style.display = null
        },
        hide() {
            this.container.style.display = 'none'
        },
        close() {
            this.message.innerText = ''
            this.hide()
        }
    }


    init() {
        this.container.classList.add('chat-container')

        this.container.appendChild(this.rooms.init(this))
        this.container.appendChild(this.tab.init(this))

        document.body.appendChild(this.container)

        this.popup.init(this.container)
    }
}

let chat = new Chat()
chat.init()