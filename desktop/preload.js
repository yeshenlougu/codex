const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('electronAPI', {
  minimize: () => ipcRenderer.send('win-minimize'),
  maximize: () => ipcRenderer.send('win-maximize'),
  close: () => ipcRenderer.send('win-close'),
  isMaximized: () => ipcRenderer.invoke('win-is-maximized'),
  platform: process.platform,
  selectFolder: () => ipcRenderer.invoke('dialog-select-folder'),
  getDefaultPath: () => ipcRenderer.invoke('get-default-path'),
});
