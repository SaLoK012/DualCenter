// SPDX-License-Identifier: MIT
//go:build windows

package main

import (
	"sync"
	"time"
	"unsafe"
)

type gameEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type settingsFile struct {
	Version               int         `json:"version"`
	HideBatteryOverlay    bool        `json:"hideBatteryOverlay"`
	BatteryAlerts         bool        `json:"batteryAlerts"`
	BlockShutdown         bool        `json:"blockShutdown"`
	ShowTaskbarIcon       bool        `json:"showTaskbarIcon"`
	GameBarEnabled        bool        `json:"gameBarEnabled"`
	GameBarChoiceMade     bool        `json:"gameBarChoiceMade"`
	GameBarBackupCaptured bool        `json:"gameBarBackupCaptured"`
	GameBarOriginalExists bool        `json:"gameBarOriginalExists"`
	GameBarOriginalValue  uint32      `json:"gameBarOriginalValue"`
	WarningSeconds        int         `json:"warningSeconds"`
	TabOrder              []int       `json:"tabOrder,omitempty"`
	Games                 []gameEntry `json:"games,omitempty"`
}

type processEntry32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

type controllerRuntime struct {
	psDown         bool
	psDownAt       time.Time
	longTriggered  bool
	lastReportAt   time.Time
	lastPSPress    time.Time
	lastPSRelease  time.Time
	dpad           byte
	cross          bool
	circle         bool
	triangle       bool
	batteryKnown   bool
	batteryPercent int
	charging       bool
	connection     string
	model          int
}

// rawReportState guarda apenas os bytes do relatório que o DualCenter utiliza.
// O DualSense envia centenas de relatórios por segundo mesmo sem qualquer botão
// mudar. Comparar esses poucos bytes antes de bloquear o estado principal evita
// trabalho desnecessário e mantém o botão PS com resposta imediata.
type rawReportState struct {
	initialized        bool
	face               byte
	ps                 bool
	hasBattery         bool
	batteryStatus      byte
	connection         string
	enhancedCountdown  uint16
	psRefreshCountdown uint8
}

// rawInputRuntime concentra o estado quente de cada controle. O Windows pode
// entregar até centenas de relatórios HID por segundo; manter um ponteiro direto
// para o último dispositivo evita várias consultas a mapas em cada pacote.
type rawInputRuntime struct {
	device    uintptr
	bluetooth bool
	report    rawReportState
}

// menuSelectionKey reúne apenas o estado que altera o foco visual. Manter uma
// chave comparável evita animações reiniciadas por atualizações de bateria,
// áudio ou processos que não mudam a seleção do usuário.
type menuSelectionKey struct {
	panel, position, row      int
	gameMenuSelection         int
	organizing, gamesFocused  bool
	gameMenuOpen, audioPicker bool
	volumeAdjust              bool
}

type appState struct {
	mu              sync.Mutex
	batteryKnown    bool
	batteryPercent  int
	charging        bool
	connection      string
	controllerModel int
	activeDevice    uintptr
	menuDevice      uintptr
	controllers     map[uintptr]*controllerRuntime
	btRequest       map[uintptr]time.Time
	btAttempts      map[uintptr]int

	overlayMode               int
	messageTitle, messageBody string
	toastHideAt               time.Time
	pendingOverlayShow        int
	lastPSActionAt            time.Time
	lastLowBatteryAlert       time.Time

	menuPanel         int
	menuPos           int
	visibleStart      int
	tabOrder          []int
	organizing        bool
	organizeOriginal  []int
	row               [6]int
	games             []gameEntry
	gameRunning       []bool
	lastGamePoll      time.Time
	gamePollInFlight  bool
	gameMenuOpen      bool
	gameMenuGame      int
	gameMenuSelection int
	gamesFocused      bool
	gameDialogOpen    bool
	volumeAdjust      bool
	audioPicker       bool
	audioOutputs      []audioDevice
	audioOutputIndex  int
	audioName         string

	hideBatteryOverlay bool
	batteryAlerts      bool
	blockShutdown      bool
	showTaskbarIcon    bool
	gameBarEnabled     bool
	gameBarChoiceMade  bool
	gameBarBackup      gameBarBackup
	startupEnabled     bool
	warningSeconds     int

	confirmAction string
	confirmUntil  time.Time

	savedForeground      uintptr
	circleReleasePending bool
	circleReleaseDevice  uintptr
	cursorHidden         bool
	cursorHideBalance    int
	cursorPositionSaved  bool
	savedCursorPosition  point
	physicalInputBlocked bool
	menuMonitor          rect
	lastLayoutCheck      time.Time
	lastMenuFocusCheck   time.Time
	selectionKeyValid    bool
	selectionKey         menuSelectionKey
	selectionStartedAt   time.Time
	selectionFrame       int
	lastAudioPoll        time.Time
	defaultAudioDeviceID string
	displayScale         float64
	scale                float64
}

var state = appState{
	batteryAlerts:   true,
	showTaskbarIcon: true,
	gameBarEnabled:  true,
	warningSeconds:  10,
	connection:      "Indisponível",
	controllers:     map[uintptr]*controllerRuntime{},
	btRequest:       map[uintptr]time.Time{},
	btAttempts:      map[uintptr]int{},
	tabOrder:        []int{tabVolume, tabEnergy, tabModeGame, tabSettings, tabBattery, tabGames},
}

var (
	mainWindow, mutexHandle           uintptr
	logPath, settingsPath, appDataDir string
	legacySettingsLoaded              bool
	settingsRecovered                 bool
)

type controllerArtwork struct {
	w, h int32
	// pixels guarda a única cópia da arte em BGRA pré-multiplicado. Todas as
	// artes são compostas diretamente na janela layered, sem duplicação em um
	// HBITMAP GDI.
	pixels []byte
}

type uiIconMask struct {
	w, h  int32
	alpha []byte
}

type layeredFrame struct {
	bits      unsafe.Pointer
	pixels    []byte
	width     int32
	height    int32
	stride    int32
	hdc       uintptr
	bitmap    uintptr
	oldBitmap uintptr
}

var (
	controllerArtworks        []controllerArtwork
	batteryControllerArtworks []controllerArtwork
	uiIconMasks               = map[string]uiIconMask{}
	uiIconArtworkCache        = map[string]controllerArtwork{}
	activeLayeredFrame        *layeredFrame
	cachedLayeredFrame        *layeredFrame
)

var (
	gameIconCache       = map[string]uintptr{}
	selectionFrameCache = map[string]controllerArtwork{}
	panelSurfaceCache   = map[string]controllerArtwork{}
	iidIImageList       = guid{
		Data1: 0x46EB5926,
		Data2: 0x582E,
		Data3: 0x4017,
		Data4: [8]byte{0x9F, 0xDF, 0xE8, 0x99, 0x8D, 0xAA, 0x09, 0x50},
	}
)

var clsidFileOpenDialog = guid{
	Data1: 0xDC1C5A9C, Data2: 0xE88A, Data3: 0x4DDE,
	Data4: [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7},
}

var iidIFileOpenDialog = guid{
	Data1: 0xD57C7288, Data2: 0xD4AD, Data3: 0x4768,
	Data4: [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60},
}

var (
	audioEndpoint           unsafe.Pointer
	audioCOMInitialized     bool
	rawInputRegistered      bool
	lastRawInputRetry       time.Time
	timerInterval           uint32
	displayRelayoutArmed    bool
	displayTransitionUntil  time.Time
	pendingGameDialogMu     sync.Mutex
	pendingGameDialogPath   string
	pendingGameDialogDevice uintptr
)
