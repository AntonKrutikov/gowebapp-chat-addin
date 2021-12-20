(async () => {

    const HEARTBEAT_TIMEOUT = 30 * 1000

    const MAX_MESSAGES_IN_TAB = 50

    const ROOM_COLORS = ['#81D4FA', '#81C784', '#F06292', '#A1887F', '#90A4AE', '#E57373']

    const USER_COLORS = ['#81D4FA', '#81C784', '#F06292', '#A1887F', '#90A4AE', '#E57373']

    let user_colors_bind = {}

    let User = {
        id: null,
        name: null,
        rooms: [],
        currentRoom: null,
        session: null
    }

    let Rooms = []

    // Obtain id and name of client from sever session before start
    async function ChatRegister() {
        let response = await fetch('/chat/join')
        let client = await response.json()
        if (client) {
            User.id = client.id
            User.name = client.name
            // Reuse channel id obtained on first time page access
            if (sessionStorage.getItem('session') === null) {
                User.session = client.session
                sessionStorage.setItem('session', client.session)
            } else {
                User.session = sessionStorage.getItem('session')
            }
            console.log(User)
        }
    }
    try {
        await ChatRegister()
    } catch (err) {
        console.log(`Can't register in chat app`)
        throw err
    }

    // Handle messages by type
    let MessageHandlers = {
        rooms(message) {
            if (message && message.body) {
                JSON.parse(message.body).forEach(r => {
                    Chat.addRoom(r)
                    if (!Rooms[r]) Rooms[r] = []
                })
            }
        },
        roomUsers(message) {
            users = JSON.parse(message.body)
            room = message.from

            if (!Array.isArray(users)) {
                console.log('Bad users response format')
                return
            }

            if (Rooms[room]) {
                Rooms[room] = []
                users.forEach(u => {
                    if (!Rooms[room].find(user => user.id == u.id)) {
                        Rooms[room].push(u)
                    }
                })
                Chat.userListRefresh(room, users)
            }
        },
        roomJoin(message) {
            user = JSON.parse(message.body)
            from = message.from
            room = message.to
            //upate self room if changed
            if (from == User.session) {
                User.rooms.push(room)
            } else {
                Rooms[room].push(user)
                Chat.userListAdd(room, user)
            }
        },
        roomLeave(message) {
            user = JSON.parse(message.body)
            from = message.from
            room = message.to
            if (Rooms[room]) {
                Rooms[room] = Rooms[room].filter(u => u.id != user.id)
                if (room.startsWith('room.')) {
                    Chat.userListRemove(room, user)
                }
                // if (room.startsWith('private.')) {
                //     Chat.addMessage(room, "user leave")
                // }
            }
        },
        addMessage(message) {
            m = message.body
            from = message.from
            if (!user_colors_bind[from]) user_colors_bind[from] = USER_COLORS.pop()
            room = message.to
            Chat.addMessage(room, m, from, user_colors_bind[from])
        },
        privateRequest(message) {
            // if (!Rooms[room].find(user => user.id == u.id)) {
            //     Rooms[room].push(u)
            // }
            if (!Rooms[message.body]) Rooms[message.body] = []
            if (message.from == User.name) {
                console.log('your request private')
                Chat.addTab(message.to, message.body)
                Chat.tabActivate(message.to)
            }
            if (message.to == User.name) {
                console.log('your request private')
                Chat.addTab(message.from, message.body)
                Chat.tabActivate(message.from)
            }
        }
    }

    let Signaling = {
        listenTimeout: null,
        listenAbort: null,
        handlers: MessageHandlers,

        async listen() {
            try {
                if (this.listenTimeout != null) clearTimeout(this.listenTimeout)
                this.listenAbort = new AbortController()

                let response = await fetch(`/chat/update?session=${User.session}`)
                let messages = await response.json() //messages must be a JSON array and suite protocol

                if (messages) {
                    messages.forEach(m => {
                        console.log(m)
                        switch (m.type) {
                            case 'rooms':
                                this.handlers.rooms(m)
                                break
                            case 'room.join':
                                this.handlers.roomJoin(m)
                                break
                            case 'room.leave':
                                this.handlers.roomLeave(m)
                                break
                            case 'room.users':
                                this.handlers.roomUsers(m)
                                break
                            case 'message':
                                this.handlers.addMessage(m)
                                break
                            case 'private':
                                this.handlers.privateRequest(m)
                                break
                        }
                    })
                }

                // Restart listener after recieve data
                this.listenTimeout = setTimeout(() => this.listen(), 500)
            } catch (err) {
                console.log(err)
            }

        },
        async send(message) {
            try {
                message.from = User.session
                let response = await fetch(`/chat/send?session=${User.session}`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify(message)
                })
                if (response.status == 200 || response.status == 500) {
                    let result = await response.json()
                }
            } catch (err) {
                console.log(err)
            }
        },
        // API calls
        getRooms() {
            this.send({
                type: 'rooms',
                body: null
            })
        },
        joinRoom(room) {
            let current = User.rooms.find(r => r == room)
            if (current === undefined) {
                this.send({
                    type: 'room.join',
                    body: room
                })
                User.rooms.push(room)
                Chat.addTab(room)
                Chat.tabActivate(room)
            }

     

        },
        callPrivate(to, name) {
            this.send({
                type: 'private',
                to: to
            })
        },
        leaveRoom(room) {

            this.send({
                type: 'room.leave',
                body: room
            })
            User.rooms = User.rooms.filter(r => r != room)

        },
        sendMessage(room, text) {
            this.send({
                type: 'message',
                body: text,
                to: room
            })
        }
        // heartbeat() {
        //     setInterval(() => {
        //         this.send({ type: 'heartbeat' })
        //     }, HEARTBEAT_TIMEOUT);
        // }
    }

    Signaling.listen()
    Signaling.getRooms()

    setTimeout(() => Signaling.joinRoom('room.default'), 100)




    /* LAYOUT */
    let Chat = {
        container: document.createElement('div'),
        roomList: document.createElement('div'),
        // userList: document.createElement('div'),
        tabsContainer: document.createElement('div'),
        tabsHeader: document.createElement('div'),
        tabs: [],
        init() {
            this.container.classList.add('chat-container')
            this.roomList.classList.add('chat-room-list')
            // this.userList.classList.add('chat-user-list')
            this.tabsContainer.classList.add('chat-tabs-container')
            this.tabsHeader.classList.add('chat-tabs-container-header')
            this.tabsContainer.appendChild(this.tabsHeader)
            this.container.appendChild(this.roomList)
            this.container.appendChild(this.tabsContainer)

        },
        roomListRow(name, type = 'room') {
            let row = document.createElement('div')
            row.classList.add('chat-room-list-row')
            let icon = document.createElement('div')
            icon.classList.add('chat-room-icon')
            icon.style.background = ROOM_COLORS.pop() //TODO: temp
            row.appendChild(icon)
            let text = document.createElement('span')
            text.innerText = name
            row.appendChild(text)
            row.dataset.name = name
            row.addEventListener('click', () => {
                console.log('whant to join', name)
                Signaling.joinRoom(name)
            })
            return row
        },
        addRoom(name) {
            this.roomList.querySelectorAll('.chat-room-list-row').forEach(n => {
                if (n.dataset.name == name)
                    return
            })
            row = this.roomListRow(name)
            this.roomList.appendChild(row)
        },
        // activateRoom(name) {
        //     this.roomList.querySelectorAll('.chat-room-list-row').forEach(n => {
        //         n.classList.remove('chat-room-list-active')
        //         if (n.dataset.name == name) {
        //             n.classList.add('chat-room-list-active')
        //         }
        //     })
        // },
        userListRow(user) {
            let row = document.createElement('div')
            row.classList.add('chat-user-list-row')
            row.innerText = user.name
            row.dataset.id = user.id
            row.addEventListener('click', () => {
                Signaling.callPrivate(user.id, user.name)

            })
            return row
        },
        userListAdd(room, user) {
            let tab = this.tabs.find(t => t.name == room)

            tab.users.appendChild(this.userListRow(user))
        },
        userListRemove(room, user) {
            let tab = this.tabs.find(t => t.name == room)

            tab.users.querySelectorAll('.chat-user-list-row').forEach(n => {
                if (n.dataset.id == user.id) {
                    tab.users.removeChild(n)
                }
            })
        },
        userListRefresh(room, users) {
            let tab = this.tabs.find(t => t.name == room)
            if(tab) {
            tab.users.replaceChildren()
            users.sort(u => u.name)
            users.forEach(u => {
                tab.users.appendChild(this.userListRow(u))
            })
            }
        },
        tabHeaderItem(room, name) {
            let header = document.createElement('div')
            header.classList.add('chat-tab-header')
            let text = document.createElement('span')
            text.innerText = name ?? room
            header.appendChild(text)
            let close = document.createElement('span')
            close.classList.add('chat-tab-header-close')
            close.innerText = 'x'
            header.appendChild(close)
            header.dataset.name = room
            header.addEventListener('click', () => {
                this.tabActivate(room)
            })
            close.addEventListener('click', (e) => {
                e.stopPropagation()
                Signaling.leaveRoom(room)

                let tabIndex = this.tabs.findIndex(t => t.room == room)
                let prevTabIndex = tabIndex - 1
                this.tabClose(room)
                if (prevTabIndex >= 0) {
                    this.tabActivate(this.tabs[prevTabIndex].name)
                }

            })
            return header
        },
        tabInnerContainer(room) {
            let container = document.createElement('div')
            container.classList.add('chat-tab-inner-container')
            container.dataset.name = room
            return container
        },
        addTab(room, private) {
            let tab = this.tabs.find(t => t.name == private ?? room)
            if (tab === undefined) {
                let userList = document.createElement('div')
                userList.classList.add('chat-user-list')
                let tabInner = this.tabsContainer.appendChild(this.tabInnerContainer(room))

                let messageContainer = document.createElement('div')
                messageContainer.classList.add('chat-tab-message-container')

                let input = document.createElement('textarea')
                input.classList.add('chat-input')
                input.addEventListener('keypress', (e) => {
                    if (e.key == "Enter") {
                        e.preventDefault()
                        let msg = e.target.value
                        Signaling.sendMessage(private ?? room, msg)
                        e.target.value = ''
                    }
                })

                let messages = document.createElement('div')
                messages.classList.add('chat-messages')

                messageContainer.appendChild(messages)
                messageContainer.appendChild(input)


                tabInner.appendChild(messageContainer)
                tabInner.appendChild(userList)

                t = {
                    name: room,
                    room: private ?? room,
                    header: this.tabsHeader.appendChild(this.tabHeaderItem(private ?? room, room )),
                    inner: tabInner,
                    messages: {
                        container: messageContainer,
                        target: messages,
                        input: input
                    },
                    users: userList
                }
                this.tabs.push(t)
                this.tabActivate(room)
            }
        },
        tabActivate(room) {
            let tab = this.tabs.find(t => t.name == room)
            this.tabsContainer.querySelectorAll('.chat-tab-inner-container').forEach(n => {
                n.style.display = 'none'
            })
            tab.inner.style.display = 'flex'
            this.tabsHeader.querySelectorAll('.chat-tab-header').forEach(n => {
                n.classList.remove('chat-tab-header-active')
            })
            tab.header.classList.add('chat-tab-header-active')
        },
        tabClose(room) {
            let tab = this.tabs.find(t => t.room == room)
            this.tabs = this.tabs.filter(t => t.room != room)
            this.tabsContainer.removeChild(tab.inner)
            this.tabsHeader.removeChild(tab.header)
        },
        addMessage(room, message, from, color) {
            let tab = this.tabs.find(t => t.room == room)
            let m = document.createElement('div')
            m.classList.add('chat-message')
            let user = document.createElement('b')
            user.innerText = from + ': '
            user.style.color = color
            m.appendChild(user)
            let text = document.createElement('span')
            text.innerText = message
            m.appendChild(text)

            if (tab) {
                tab.messages.target.prepend(m)
                let i = 0
                tab.messages.target.querySelectorAll('.chat-message').forEach(n => {
                    i++
                    if (i > MAX_MESSAGES_IN_TAB) {
                        tab.messages.target.removeChild(n)
                    }
                })
            }
        },
        show() {
            document.body.append(this.container)
        }
    }
    Chat.init()
    Chat.show()

})()

