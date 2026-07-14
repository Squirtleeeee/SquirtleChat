const { app, BrowserWindow, ipcMain, Tray, Menu, nativeImage, screen } = require('electron')
const path = require('path')
const fs = require('fs')

const isDev = !app.isPackaged
const DEV_URL = process.env.SQUIRTLE_DEV_URL || 'http://127.0.0.1:5173'
const chatWindows = new Map()
let mainWindow = null
let tray = null

const statePath = () => path.join(app.getPath('userData'), 'window-state.json')

function loadState() {
  try {
    return JSON.parse(fs.readFileSync(statePath(), 'utf8'))
  } catch {
    return {}
  }
}

function saveState(patch) {
  const cur = loadState()
  fs.writeFileSync(statePath(), JSON.stringify({ ...cur, ...patch }, null, 2))
}

function appUrl(hashPath = '/') {
  const hash = hashPath.startsWith('#') ? hashPath : `#${hashPath}`
  if (isDev) return `${DEV_URL}/${hash}`
  return `file://${path.join(__dirname, '..', 'frontend', 'dist', 'index.html')}${hash}`
}

function createMainWindow() {
  const st = loadState().main || {}
  // 启动先按登录小窗尺寸，进入主界面后再放大
  mainWindow = new BrowserWindow({
    width: 400,
    height: 640,
    x: st.x,
    y: st.y,
    minWidth: 360,
    minHeight: 520,
    resizable: false,
    frame: false,
    title: 'SquirtleChat',
    show: false,
    autoHideMenuBar: true,
    backgroundColor: '#ffffff',
    webPreferences: {
      preload: path.join(__dirname, 'preload.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: false,
    },
  })
  mainWindow.setMenuBarVisibility(false)

  mainWindow.loadURL(appUrl('/'))
  mainWindow.once('ready-to-show', () => mainWindow.show())
  mainWindow.on('closed', () => {
    mainWindow = null
  })

  const persist = () => {
    if (!mainWindow || mainWindow.isDestroyed() || mainWindow.isMinimized()) return
    const b = mainWindow.getBounds()
    saveState({ main: b })
  }
  mainWindow.on('resize', persist)
  mainWindow.on('move', persist)

  mainWindow.on('close', (e) => {
    if (!app.isQuitting && tray) {
      e.preventDefault()
      mainWindow.hide()
    }
  })
}

function chatWindowKey(payload) {
  return `${payload.type}:${payload.id}`
}

function openChatWindow(payload) {
  const key = chatWindowKey(payload)
  const existing = chatWindows.get(key)
  if (existing && !existing.isDestroyed()) {
    existing.focus()
    return { ok: true, focused: true }
  }

  const st = (loadState().chats || {})[key] || {}
  const { width: sw, height: sh } = screen.getPrimaryDisplay().workAreaSize
  const win = new BrowserWindow({
    width: st.width || Math.min(480, Math.floor(sw * 0.38)),
    height: st.height || Math.min(720, Math.floor(sh * 0.82)),
    x: st.x,
    y: st.y,
    minWidth: 380,
    minHeight: 520,
    title: payload.title || '会话',
    show: false,
    frame: false,
    autoHideMenuBar: true,
    backgroundColor: '#ededed',
    webPreferences: {
      preload: path.join(__dirname, 'preload.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: false,
    },
  })
  win.setMenuBarVisibility(false)

  const q = new URLSearchParams({
    type: String(payload.type || 'friend'),
    id: String(payload.id || ''),
    title: String(payload.title || ''),
  })
  win.loadURL(appUrl(`/popup-chat?${q.toString()}`))
  win.once('ready-to-show', () => win.show())
  win.on('closed', () => chatWindows.delete(key))

  const persist = () => {
    if (win.isDestroyed() || win.isMinimized()) return
    const all = loadState()
    const chats = { ...(all.chats || {}), [key]: win.getBounds() }
    saveState({ chats })
  }
  win.on('resize', persist)
  win.on('move', persist)

  chatWindows.set(key, win)
  return { ok: true, focused: false }
}

function createTray() {
  // Simple blue square icon
  const size = 16
  const buf = Buffer.alloc(size * size * 4)
  for (let i = 0; i < size * size; i++) {
    buf[i * 4] = 37
    buf[i * 4 + 1] = 99
    buf[i * 4 + 2] = 235
    buf[i * 4 + 3] = 255
  }
  const img = nativeImage.createFromBuffer(buf, { width: size, height: size })
  tray = new Tray(img)
  const menu = Menu.buildFromTemplate([
    {
      label: '显示主窗口',
      click: () => {
        if (!mainWindow) createMainWindow()
        else {
          mainWindow.show()
          mainWindow.focus()
        }
      },
    },
    { type: 'separator' },
    {
      label: '退出',
      click: () => {
        app.isQuitting = true
        app.quit()
      },
    },
  ])
  tray.setToolTip('SquirtleChat')
  tray.setContextMenu(menu)
  tray.on('double-click', () => {
    if (!mainWindow) createMainWindow()
    else {
      mainWindow.show()
      mainWindow.focus()
    }
  })
}

function registerIpc() {
  ipcMain.handle('desktop:is-electron', () => true)
  ipcMain.handle('desktop:open-chat-window', (_e, payload) => openChatWindow(payload || {}))
  ipcMain.handle('desktop:set-always-on-top', (e, flag) => {
    const win = BrowserWindow.fromWebContents(e.sender)
    if (win) win.setAlwaysOnTop(!!flag)
    return { ok: true }
  })
  ipcMain.handle('desktop:get-window-role', (e) => {
    const win = BrowserWindow.fromWebContents(e.sender)
    if (!win) return 'unknown'
    if (win === mainWindow) return 'main'
    for (const [key, w] of chatWindows) {
      if (w === win) return `chat:${key}`
    }
    return 'unknown'
  })
  ipcMain.handle('desktop:focus-main', () => {
    if (!mainWindow) createMainWindow()
    else {
      mainWindow.show()
      mainWindow.focus()
    }
    return { ok: true }
  })
  ipcMain.handle('desktop:window-minimize', (e) => {
    BrowserWindow.fromWebContents(e.sender)?.minimize()
    return { ok: true }
  })
  ipcMain.handle('desktop:window-maximize', (e) => {
    const win = BrowserWindow.fromWebContents(e.sender)
    if (!win) return { ok: false }
    if (win.isMaximized()) win.unmaximize()
    else win.maximize()
    return { ok: true, maximized: win.isMaximized() }
  })
  ipcMain.handle('desktop:window-close', (e) => {
    BrowserWindow.fromWebContents(e.sender)?.close()
    return { ok: true }
  })
  ipcMain.handle('desktop:window-is-maximized', (e) => {
    return !!BrowserWindow.fromWebContents(e.sender)?.isMaximized()
  })
  ipcMain.handle('desktop:set-shell-mode', (e, mode) => {
    const win = BrowserWindow.fromWebContents(e.sender)
    if (!win) return { ok: false }
    if (mode === 'login') {
      win.setMinimumSize(360, 520)
      win.setSize(400, 640, true)
      win.setResizable(false)
      win.center()
      return { ok: true, mode: 'login' }
    }
    win.setResizable(true)
    win.setMinimumSize(800, 560)
    const st = loadState().main
    if (st?.width && st?.height) {
      win.setSize(st.width, st.height, true)
      if (typeof st.x === 'number' && typeof st.y === 'number') win.setPosition(st.x, st.y)
    } else {
      win.setSize(1100, 720, true)
      win.center()
    }
    return { ok: true, mode: 'main' }
  })
}

app.whenReady().then(() => {
  registerIpc()
  createMainWindow()
  createTray()
  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) createMainWindow()
  })
})

app.on('before-quit', () => {
  app.isQuitting = true
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin' && app.isQuitting) app.quit()
})
