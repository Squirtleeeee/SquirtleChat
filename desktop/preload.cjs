const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('squirtleDesktop', {
  isElectron: true,
  openChatWindow: (payload) => ipcRenderer.invoke('desktop:open-chat-window', payload),
  setAlwaysOnTop: (flag) => ipcRenderer.invoke('desktop:set-always-on-top', flag),
  getWindowRole: () => ipcRenderer.invoke('desktop:get-window-role'),
  focusMain: () => ipcRenderer.invoke('desktop:focus-main'),
  setShellMode: (mode) => ipcRenderer.invoke('desktop:set-shell-mode', mode),
  windowMinimize: () => ipcRenderer.invoke('desktop:window-minimize'),
  windowMaximize: () => ipcRenderer.invoke('desktop:window-maximize'),
  windowClose: () => ipcRenderer.invoke('desktop:window-close'),
  isMaximized: () => ipcRenderer.invoke('desktop:window-is-maximized'),
})
