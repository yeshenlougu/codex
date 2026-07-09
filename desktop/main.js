const { app, BrowserWindow, ipcMain, Tray, Menu, screen, globalShortcut } = require('electron');
const path = require('path');
const http = require('http');
const { spawn } = require('child_process');

// Backend
const isWindows = process.platform === 'win32';
const backendName = isWindows ? 'codex-go.exe' : 'codex-go';
const backendPath = app.isPackaged
  ? path.join(process.resourcesPath, 'backend', backendName)
  : path.join(__dirname, backendName);
const API_URL = 'http://localhost:1977';

let mainWindow = null;
let petWindow = null;
let tray = null;
let petState = 'sleeping';
let isQuitting = false;
let backendProc = null;

// ============ Backend Lifecycle ============

function startBackend() {
  return new Promise((resolve, reject) => {
    if (app.isPackaged) {
      // In packaged app, spawn the bundled Go binary
      backendProc = spawn(backendPath, ['--serve', '--addr', ':1977'], {
        stdio: ['ignore', 'pipe', 'pipe'],
        env: { ...process.env },
      });
      backendProc.stdout.on('data', (data) => {
        console.log(`[backend] ${data.toString().trim()}`);
        if (data.toString().includes('listening')) resolve();
      });
      backendProc.stderr.on('data', (data) => {
        console.error(`[backend-err] ${data.toString().trim()}`);
      });
      backendProc.on('error', reject);
      backendProc.on('exit', (code) => {
        if (!isQuitting) console.log(`[backend] exited with code ${code}`);
      });
      // Resolve after timeout if backend doesn't log "listening"
      setTimeout(resolve, 3000);
    } else {
      // Dev mode: assume backend is already running
      resolve();
    }
  });
}

function stopBackend() {
  if (backendProc) {
    backendProc.kill();
    backendProc = null;
  }
}

// ============ Main Window ============

function createMainWindow() {
  mainWindow = new BrowserWindow({
    width: 1200, height: 800, minWidth: 900, minHeight: 600,
    title: 'Codex Go',
    webPreferences: { nodeIntegration: false, contextIsolation: true },
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
    <html><body style="background:#0d1117;color:#e6edf3;display:flex;align-items:center;justify-content:center;height:100vh;font-family:sans-serif">
    <div style="text-align:center"><h2>Codex Go</h2><p>Backend not running.</p></div></body></html>`);
}

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
    { label: 'Switch Pet → Cat', click: () => sendToPet('pet-type', 'cat') },
    { label: 'Switch Pet → Dog', click: () => sendToPet('pet-type', 'dog') },
    { label: 'Switch Pet → Fox', click: () => sendToPet('pet-type', 'fox') },
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
    const data = await new Promise((res, rej) => {
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
    const data = await new Promise((res, rej) => {
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

app.on('activate', () => { if (mainWindow) mainWindow.show(); });

// ============ Lifecycle ============

app.whenReady().then(async () => {
  await startBackend();
  createMainWindow();
  createPetWindow();
  createTray();
  setInterval(pollPetState, 3000);
  checkForUpdates();
  setInterval(checkForUpdates, 3600000);
  globalShortcut.register('CommandOrControl+Shift+C', () => {
    if (mainWindow) mainWindow.isVisible() ? mainWindow.hide() : mainWindow.show();
  });
});

app.on('window-all-closed', () => {});
app.on('before-quit', () => {
  isQuitting = true;
  globalShortcut.unregisterAll();
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
