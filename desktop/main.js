const { app, BrowserWindow, ipcMain, Tray, Menu, screen, globalShortcut } = require('electron');
const path = require('path');
const http = require('http');

// Backend API URL
const API_URL = 'http://localhost:1977';

let mainWindow = null;
let petWindow = null;
let tray = null;
let petState = 'sleeping';
let isQuitting = false;

// ============ Main Window (React Web UI) ============

function createMainWindow() {
  mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    minWidth: 900,
    minHeight: 600,
    title: 'Codex Go',
    icon: path.join(__dirname, 'assets', 'icon.png'),
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
    },
    show: false,
  });

  // Load from Go backend (which serves the React frontend)
  loadMainUI();

  mainWindow.once('ready-to-show', () => {
    mainWindow.show();
  });

  mainWindow.on('close', (e) => {
    if (!isQuitting) {
      e.preventDefault();
      mainWindow.hide(); // minimize to tray
    }
  });

  mainWindow.on('closed', () => { mainWindow = null; });
}

async function loadMainUI() {
  // Wait for Go backend to be ready, then load the React app
  let attempts = 0;
  while (attempts < 30) {
    try {
      await new Promise((resolve, reject) => {
        http.get(API_URL + '/health', (res) => {
          if (res.statusCode === 200) resolve();
          else reject();
        }).on('error', reject);
      });
      mainWindow.loadURL(API_URL);
      return;
    } catch {
      attempts++;
      await new Promise(r => setTimeout(r, 1000));
    }
  }
  // Fallback: show offline message
  mainWindow.loadURL(`data:text/html,
    <html><body style="background:#0d1117;color:#e6edf3;display:flex;align-items:center;justify-content:center;height:100vh;font-family:sans-serif">
    <div style="text-align:center"><h2>Codex Go</h2><p>Backend not running.</p><p style="color:#8b949e;font-size:13px">Run <code style="color:#58a6ff">codex-go --serve</code> first</p></div>
    </body></html>`);
}

// ============ Pet Window (Transparent Desktop Pet) ============

function createPetWindow() {
  const { width: screenW, height: screenH } = screen.getPrimaryDisplay().workAreaSize;

  petWindow = new BrowserWindow({
    width: 180,
    height: 220,
    x: screenW - 200,
    y: screenH - 240,
    frame: false,
    transparent: true,
    alwaysOnTop: true,
    resizable: false,
    skipTaskbar: true,
    hasShadow: false,
    focusable: true,
    webPreferences: {
      nodeIntegration: true,
      contextIsolation: false,
    },
  });

  petWindow.loadFile(path.join(__dirname, 'pet.html'));
  petWindow.setVisibleOnAllWorkspaces(true, { visibleOnFullScreen: true });

  // Keep position in sync
  petWindow.on('moved', () => {
    const [x, y] = petWindow.getPosition();
    petState = { ...petState, posX: x, posY: y };
  });

  petWindow.on('closed', () => { petWindow = null; });
}

// ============ System Tray ============

function createTray() {
  // Create a simple 16x16 tray icon (blue circle)
  const { nativeImage } = require('electron');
  const buf = Buffer.alloc(16 * 16 * 4);
  for (let i = 0; i < 256; i++) {
    const x = i % 16, y = Math.floor(i / 16);
    const dist = Math.sqrt((x - 8) ** 2 + (y - 8) ** 2);
    if (dist < 7) {
      buf[i * 4] = 88; buf[i * 4 + 1] = 166; buf[i * 4 + 2] = 255; buf[i * 4 + 3] = 255;
    }
  }
  tray = new Tray(nativeImage.createFromBuffer(buf, { width: 16, height: 16 }));
  tray.setToolTip('Codex Go');

  updateTrayMenu();
  tray.on('double-click', () => {
    if (mainWindow) {
      mainWindow.isVisible() ? mainWindow.hide() : mainWindow.show();
    }
  });
}

function updateTrayMenu() {
  const menu = Menu.buildFromTemplate([
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
  ]);
  tray.setContextMenu(menu);
}

function sendToPet(channel, data) {
  if (petWindow && !petWindow.isDestroyed()) {
    petWindow.webContents.send(channel, data);
  }
}

// ============ Pet State Polling ============

async function pollPetState() {
  try {
    const data = await new Promise((resolve, reject) => {
      http.get(API_URL + '/api/pet-state', (res) => {
        let body = '';
        res.on('data', c => body += c);
        res.on('end', () => resolve(body));
      }).on('error', () => resolve('{}'));
    });
    const state = JSON.parse(data);
    if (state && state.status && state.status !== petState) {
      petState = state.status;
      sendToPet('pet-state', petState);
      updateTrayMenu();
    }
  } catch { /* backend not ready */ }
}

// ============ IPC ============

ipcMain.on('pet-action', (event, action) => {
  if (action === 'wake') {
    petState = 'idle';
    sendToPet('pet-state', 'idle');
    updateTrayMenu();
  }
});

// Restore main window when dock icon clicked (macOS)
app.on('activate', () => {
  if (mainWindow) mainWindow.show();
});

// ============ Auto Update ============

async function checkForUpdates() {
  try {
    const data = await new Promise((resolve, reject) => {
      http.get(API_URL + '/api/update', (res) => {
        let body = '';
        res.on('data', c => body += c);
        res.on('end', () => resolve(body));
      }).on('error', () => resolve('{}'));
    });
    const info = JSON.parse(data);
    if (info && info.has_update) {
      // Show update notification
      if (mainWindow && !mainWindow.isDestroyed()) {
        mainWindow.webContents.executeJavaScript(`
          document.dispatchEvent(new CustomEvent('codex-update', { detail: ${JSON.stringify(info)} }));
        `);
      }
      // Update tray tooltip
      if (tray) {
        tray.setToolTip(`Codex Go - Update available: ${info.latest}`);
      }
      console.log(`[update] New version available: ${info.current} → ${info.latest}`);
    }
  } catch { /* offline, ignore */ }
}

// ============ App Lifecycle ============

app.whenReady().then(() => {
  createMainWindow();
  createPetWindow();
  createTray();
  setInterval(pollPetState, 3000);
  checkForUpdates();
  setInterval(checkForUpdates, 3600000); // check every hour

  // Global shortcut: Ctrl+Shift+C to toggle Codex
  globalShortcut.register('CommandOrControl+Shift+C', () => {
    if (mainWindow) {
      mainWindow.isVisible() ? mainWindow.hide() : mainWindow.show();
    }
  });
});

app.on('window-all-closed', () => {
  // Don't quit - keep running in tray
});

app.on('before-quit', () => {
  isQuitting = true;
  globalShortcut.unregisterAll();
});

// Prevent multiple instances
const gotLock = app.requestSingleInstanceLock();
if (!gotLock) {
  app.quit();
} else {
  app.on('second-instance', () => {
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      mainWindow.show();
      mainWindow.focus();
    }
  });
}
