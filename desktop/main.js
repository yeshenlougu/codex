const { app, BrowserWindow, ipcMain, Tray, Menu, screen, globalShortcut, dialog, nativeTheme } = require('electron');
const path = require('path');
const http = require('http');
const fs = require('fs');
const { spawn } = require('child_process');

// Backend
const isWindows = process.platform === 'win32';
const backendName = isWindows ? 'codex-go.exe' : 'codex-go';
const backendPath = app.isPackaged
  ? path.join(process.resourcesPath, 'backend', backendName)
  : path.join(__dirname, backendName);
const API_URL = 'http://localhost:1977';
const LOG_DIR = path.join(app.getPath('userData'), 'logs');

let mainWindow = null;
let petWindow = null;
let tray = null;
let petState = 'sleeping';
let isQuitting = false;
let backendProc = null;

// Remove default menu bar
Menu.setApplicationMenu(null);

// ============ Backend Lifecycle ============

function startBackend() {
  return new Promise((resolve, reject) => {
    if (!app.isPackaged) { resolve(); return; }

    try { fs.mkdirSync(LOG_DIR, { recursive: true }); } catch {}

    if (!fs.existsSync(backendPath)) {
      const msg = `Backend binary not found:\n${backendPath}`;
      dialog.showErrorBox('Codex Go — Startup Error', msg);
      reject(new Error(msg)); return;
    }

    const logStream = fs.createWriteStream(path.join(LOG_DIR, 'backend.log'), { flags: 'a' });
    logStream.write(`[${new Date().toISOString()}] Starting backend: ${backendPath}\n`);

    backendProc = spawn(backendPath, ['--serve', '--addr', ':1977'], {
      stdio: ['ignore', 'pipe', 'pipe'],
      env: { ...process.env },
    });

    backendProc.stdout.on('data', (data) => {
      const text = data.toString().trim();
      logStream.write(`[out] ${text}\n`);
      if (text.includes('listening')) resolve();
    });

    backendProc.stderr.on('data', (data) => {
      logStream.write(`[err] ${data.toString().trim()}\n`);
    });

    backendProc.on('error', (err) => {
      const msg = `Failed to start backend:\n${backendPath}\n\n${err.message}`;
      logStream.write(`[error] ${msg}\n`);
      dialog.showErrorBox('Codex Go — Backend Error', msg);
      reject(err);
    });

    backendProc.on('exit', (code) => {
      if (!isQuitting) logStream.write(`[exit] code ${code}\n`);
    });

    setTimeout(() => {
      logStream.write(`[${new Date().toISOString()}] Startup timeout, proceeding...\n`);
      resolve();
    }, 3000);
  });
}

function stopBackend() {
  if (backendProc) { backendProc.kill(); backendProc = null; }
}

// ============ Main Window (frameless) ============

function createMainWindow() {
  mainWindow = new BrowserWindow({
    width: 1200, height: 800, minWidth: 900, minHeight: 600,
    title: 'Codex Go',
    frame: false,
    titleBarStyle: 'hidden',
    backgroundColor: '#ffffff',
    webPreferences: { nodeIntegration: false, contextIsolation: true, preload: path.join(__dirname, 'preload.js') },
    show: false,
  });

  loadMainUI();
  mainWindow.once('ready-to-show', () => mainWindow.show());
  mainWindow.on('close', (e) => {
    if (!isQuitting) { e.preventDefault(); mainWindow.hide(); }
  });
  mainWindow.on('closed', () => { mainWindow = null; });
}

async function loadMainUI() {
  for (let i = 0; i < 30; i++) {
    try {
      await new Promise((res, rej) => {
        http.get(API_URL + '/health', r => r.statusCode === 200 ? res() : rej()).on('error', rej);
      });
      mainWindow.loadURL(API_URL);
      return;
    } catch { await new Promise(r => setTimeout(r, 1000)); }
  }
  mainWindow.loadURL(`data:text/html,
    <html><body style="background:#0c0c0d;color:#d0d6e0;display:flex;align-items:center;justify-content:center;height:100vh;font-family:system-ui">
    <div style="text-align:center"><h2>Codex Go</h2><p>Backend not running.</p></div></body></html>`);
}

// ============ Window controls IPC ============

ipcMain.on('win-minimize', () => mainWindow?.minimize());
ipcMain.on('win-maximize', () => {
  if (mainWindow?.isMaximized()) mainWindow.unmaximize(); else mainWindow?.maximize();
});
ipcMain.on('win-close', () => { isQuitting = true; app.quit(); });

// ============ Folder picker IPC ============

ipcMain.handle('dialog-select-folder', async () => {
  if (!mainWindow) return null;
  const result = await dialog.showOpenDialog(mainWindow, {
    properties: ['openDirectory'],
    title: '选择项目文件夹',
  });
  if (result.canceled || result.filePaths.length === 0) return null;
  return result.filePaths[0];
});

ipcMain.handle('get-default-path', () => {
  if (process.platform === 'win32') return process.env.USERPROFILE || 'C:\\';
  return process.env.HOME || '/';
});

// ============ Pet Window ============

function createPetWindow() {
  const { width: screenW, height: screenH } = screen.getPrimaryDisplay().workAreaSize;
  petWindow = new BrowserWindow({
    width: 180, height: 220, x: screenW - 200, y: screenH - 240,
    frame: false, transparent: true, alwaysOnTop: true,
    resizable: false, skipTaskbar: true, hasShadow: false, focusable: true,
    webPreferences: { nodeIntegration: true, contextIsolation: false },
  });
  petWindow.loadFile(path.join(__dirname, 'pet.html'));
  petWindow.setVisibleOnAllWorkspaces(true, { visibleOnFullScreen: true });
  petWindow.on('closed', () => { petWindow = null; });
}

// ============ Tray ============

function createTray() {
  const { nativeImage } = require('electron');
  const buf = Buffer.alloc(16 * 16 * 4);
  for (let i = 0; i < 256; i++) {
    const x = i % 16, y = Math.floor(i / 16);
    if (Math.sqrt((x - 8) ** 2 + (y - 8) ** 2) < 7) {
      buf[i * 4] = 88; buf[i * 4 + 1] = 166; buf[i * 4 + 2] = 255; buf[i * 4 + 3] = 255;
    }
  }
  tray = new Tray(nativeImage.createFromBuffer(buf, { width: 16, height: 16 }));
  tray.setToolTip('Codex Go');
  updateTrayMenu();
  tray.on('double-click', () => { if (mainWindow) mainWindow.isVisible() ? mainWindow.hide() : mainWindow.show(); });
}

function updateTrayMenu() {
  tray.setContextMenu(Menu.buildFromTemplate([
    { label: 'Show Codex', click: () => { if (mainWindow) { mainWindow.show(); mainWindow.focus(); } } },
    { label: 'Show Pet', click: () => { if (petWindow) petWindow.show(); } },
    { label: 'Hide Pet', click: () => { if (petWindow) petWindow.hide(); } },
    { type: 'separator' },
    { label: `Pet: ${petState}`, enabled: false },
    { type: 'separator' },
    { label: 'Switch → Cat', click: () => sendToPet('pet-type', 'cat') },
    { label: 'Switch → Dog', click: () => sendToPet('pet-type', 'dog') },
    { label: 'Switch → Fox', click: () => sendToPet('pet-type', 'fox') },
    { type: 'separator' },
    { label: 'Quit Codex', click: () => { isQuitting = true; app.quit(); } },
  ]));
}

function sendToPet(ch, data) {
  if (petWindow && !petWindow.isDestroyed()) petWindow.webContents.send(ch, data);
}

// ============ Polling ============

async function pollPetState() {
  try {
    const data = await new Promise((res) => {
      http.get(API_URL + '/api/pet-state', r => { let b = ''; r.on('data', c => b += c); r.on('end', () => res(b)); }).on('error', () => res('{}'));
    });
    const state = JSON.parse(data);
    if (state?.status && state.status !== petState) {
      petState = state.status;
      sendToPet('pet-state', petState);
      updateTrayMenu();
    }
  } catch {}
}

async function checkForUpdates() {
  try {
    const data = await new Promise((res) => {
      http.get(API_URL + '/api/update', r => { let b = ''; r.on('data', c => b += c); r.on('end', () => res(b)); }).on('error', () => res('{}'));
    });
    const info = JSON.parse(data);
    if (info?.has_update && tray) tray.setToolTip(`Codex Go - Update: ${info.latest}`);
  } catch {}
}

// ============ IPC ============

ipcMain.on('pet-action', (_, action) => {
  if (action === 'wake') { petState = 'idle'; sendToPet('pet-state', 'idle'); updateTrayMenu(); }
});

// ============ Lifecycle ============

app.on('activate', () => { if (mainWindow) mainWindow.show(); });

app.whenReady().then(async () => {
  try {
    await startBackend();
    createMainWindow();
    createPetWindow();
    createTray();
    setInterval(pollPetState, 3000);
    checkForUpdates();
    setInterval(checkForUpdates, 3600000);
    if (app.isReady()) {
      globalShortcut.register('CommandOrControl+Shift+C', () => {
        if (mainWindow) mainWindow.isVisible() ? mainWindow.hide() : mainWindow.show();
      });
    }
  } catch (err) {
    dialog.showErrorBox('Codex Go — Fatal Error',
      `Failed to start:\n\n${err.message}\n\nCheck logs at:\n${path.join(LOG_DIR, 'backend.log')}`);
    app.quit();
  }
});

app.on('window-all-closed', () => {});
app.on('before-quit', () => {
  isQuitting = true;
  if (app.isReady()) globalShortcut.unregisterAll();
  stopBackend();
});

const gotLock = app.requestSingleInstanceLock();
if (!gotLock) {
  app.quit();
} else {
  app.on('second-instance', () => {
    if (mainWindow) { if (mainWindow.isMinimized()) mainWindow.restore(); mainWindow.show(); mainWindow.focus(); }
  });
}
