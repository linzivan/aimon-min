package widget

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	"ai-monitor/internal/logger"
	"ai-monitor/internal/types"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")

	procCreateWindowExW = user32.NewProc("CreateWindowExW")
	procDestroyWindow   = user32.NewProc("DestroyWindow")
	procShowWindow      = user32.NewProc("ShowWindow")
	procUpdateWindow    = user32.NewProc("UpdateWindow")
	procGetMessageW     = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procPostQuitMessage = user32.NewProc("PostQuitMessage")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procMoveWindow      = user32.NewProc("MoveWindow")
	procGetWindowRect   = user32.NewProc("GetWindowRect")
	procSetCapture      = user32.NewProc("SetCapture")
	procReleaseCapture  = user32.NewProc("ReleaseCapture")
	procSetWindowTextW  = user32.NewProc("SetWindowTextW")
	procRegisterClassW  = user32.NewProc("RegisterClassW")
	procDefWindowProcW  = user32.NewProc("DefWindowProcW")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject    = gdi32.NewProc("DeleteObject")
	procSetTextColor    = gdi32.NewProc("SetTextColor")
	procSetBkColor      = gdi32.NewProc("SetBkColor")
	procSetBkMode       = gdi32.NewProc("SetBkMode")
	procSendMessageW    = user32.NewProc("SendMessageW")
	procCreateFontW     = gdi32.NewProc("CreateFontW")
)

const (
	wndClass = "AIMonitorV5"
	wsPopup  = 0x80000000
	wsChild  = 0x40000000
	wsVisible = 0x10000000

	wsExLayered    = 0x80000
	wsExToolWindow = 0x80
	wsExTopMost    = 0x8

	wmDestroy   = 0x0002
	wmNcHitTest = 0x0084
	wmLButtonDown = 0x0201
	wmMouseMove   = 0x0200
	wmLButtonUp   = 0x0202
	wmCtlColorStatic = 0x0138
	wmEraseBknd  = 0x0014
	wmSetFont    = 0x0030

	htCaption = 2
	swShow    = 5

	// Widget dimensions
	widgetW = 140
	widgetH = 85

	// Control IDs for WM_CTLCOLORSTATIC differentiation
	idProvider = 201
	idBalance  = 202

	dwMWA_WINDOW_CORNER = 33
	dwMWCP_ROUND        = 2
)

type Widget struct {
	mu          sync.RWMutex
	hwnd        windows.Handle
	metrics     *types.Metrics
	running     bool
	wg          sync.WaitGroup
	store       PositionStore
	wndProcCallback uintptr

	hProvider  windows.Handle
	hBalance   windows.Handle
	hFontSmall uintptr
	hFontBig   uintptr
	hBgBrush   uintptr
}

type PositionStore interface {
	GetConfig(key string) (string, error)
	SetConfig(key, value string) error
}

func New(store PositionStore) *Widget {
	bg, _, _ := procCreateSolidBrush.Call(uintptr(0x1E1E2E))
	return &Widget{store: store, hBgBrush: bg}
}

func (w *Widget) Update(m *types.Metrics) {
	w.mu.Lock()
	w.metrics = m
	w.mu.Unlock()
	w.updateUI()
}

func (w *Widget) updateUI() {
	w.mu.RLock()
	m := w.metrics
	hp := w.hProvider
	hb := w.hBalance
	w.mu.RUnlock()
	if hp == 0 || hb == 0 {
		return
	}
	if m == nil || m.Error != "" {
		procSetWindowTextW.Call(uintptr(hp),
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("AI Monitor"))))
		procSetWindowTextW.Call(uintptr(hb),
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("--.--"))))
		return
	}
	procSetWindowTextW.Call(uintptr(hp),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(m.ProviderName))))
	procSetWindowTextW.Call(uintptr(hb),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(fmt.Sprintf("\u00a5%.2f", m.Balance)))))
}

func (w *Widget) Start() error {
	w.mu.Lock()
	if w.running { w.mu.Unlock(); return nil }
	w.running = true
	w.mu.Unlock()

	hInst, _, _ := procGetModuleHandleW.Call(0)
	if hInst == 0 { return fmt.Errorf("get module handle failed") }

	className, _ := syscall.UTF16PtrFromString(wndClass)
	w.wndProcCallback = windows.NewCallback(w.windowProc)

	wc := struct {
		style      uint32
		lpfnWndProc uintptr
		cbClsExtra int32
		cbWndExtra int32
		hInstance  windows.Handle
		hIcon      windows.Handle
		hCursor    windows.Handle
		hbrBackground windows.Handle
		menuName   *uint16
		className  *uint16
	}{lpfnWndProc: w.wndProcCallback, hInstance: windows.Handle(hInst), className: className}
	procRegisterClassW.Call(uintptr(unsafe.Pointer(&wc)))

	x, y := w.loadPosition()
	hwnd, _, _ := procCreateWindowExW.Call(
		uintptr(wsExLayered|wsExToolWindow),
		uintptr(unsafe.Pointer(className)),
		0, uintptr(wsPopup),
		uintptr(x), uintptr(y), widgetW, widgetH,
		0, 0, hInst, 0,
	)
	if hwnd == 0 { return fmt.Errorf("create window failed") }
	w.hwnd = windows.Handle(hwnd)

	cornerPref := uint32(dwMWCP_ROUND)
	procDwmSetWindowAttribute.Call(hwnd, dwMWA_WINDOW_CORNER, uintptr(unsafe.Pointer(&cornerPref)), 4)
	procSetLayeredWindowAttributes.Call(hwnd, 0, 235, 2)

	// Create fonts
	// WM_SETFONT for static text: we store fonts, apply them via WM_SETFONT after creating controls
	fnSmall, _, _ := procCreateFontW.Call(uintptr(15), 0, 0, 0, uintptr(700), 0, 0, 0, 0x86, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Segoe UI"))))
	fnBig, _, _ := procCreateFontW.Call(uintptr(26), 0, 0, 0, uintptr(700), 0, 0, 0, 0x86, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Segoe UI"))))
	w.hFontSmall = fnSmall
	w.hFontBig = fnBig

	// Create two STATIC controls
	cls, _ := syscall.UTF16PtrFromString("Static")

	// Provider name (top line, muted)
	hp, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("DeepSeek"))),
		uintptr(uint32(wsChild|wsVisible|0x01)), // SS_CENTER
		0, 8, widgetW, 26, hwnd, idProvider, 0, 0)
	w.hProvider = windows.Handle(hp)
	if hp != 0 && fnSmall != 0 {
		procSendMessageW.Call(hp, wmSetFont, fnSmall, 0)
	}

	// Balance amount (bottom line, big)
	hbal, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("\u00a50.00"))),
		uintptr(uint32(wsChild|wsVisible|0x01)), // SS_CENTER
		0, 36, widgetW, 38, hwnd, idBalance, 0, 0)
	w.hBalance = windows.Handle(hbal)
	if hbal != 0 && fnBig != 0 {
		procSendMessageW.Call(hbal, wmSetFont, fnBig, 0)
	}

	procShowWindow.Call(hwnd, swShow)
	procUpdateWindow.Call(hwnd)
	logger.Info("[widget] started")

	w.wg.Add(1); go w.messageLoop()
	return nil
}

func (w *Widget) messageLoop() {
	defer w.wg.Done()
	var msg struct {
		Hwnd windows.Handle; Message uint32; WParam uintptr; LParam uintptr
		Time uint32; Pt struct{ X, Y int32 }
	}
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 { break }
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("[widget] windowProc panic on msg %d: %v", msg.Message, r)
				}
			}()
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}()
	}
	w.mu.Lock(); w.running = false; w.mu.Unlock()
}

func (w *Widget) Stop() {
	w.mu.Lock(); hwnd := w.hwnd; w.mu.Unlock()
	if hwnd != 0 { procDestroyWindow.Call(uintptr(hwnd)) }
	w.wg.Wait()
	if w.hFontSmall != 0 { procDeleteObject.Call(w.hFontSmall) }
	if w.hFontBig != 0 { procDeleteObject.Call(w.hFontBig) }
	if w.hBgBrush != 0 { procDeleteObject.Call(w.hBgBrush) }
}

func (w *Widget) loadPosition() (int, int) {
	x, y := 1200, 200
	if w.store == nil { return x, y }
	if xs, _ := w.store.GetConfig("widget_pos_x"); xs != "" { fmt.Sscanf(xs, "%d", &x) }
	if ys, _ := w.store.GetConfig("widget_pos_y"); ys != "" { fmt.Sscanf(ys, "%d", &y) }
	return x, y
}

func (w *Widget) savePosition() {
	if w.store == nil { return }
	w.mu.RLock(); hwnd := w.hwnd; w.mu.RUnlock()
	if hwnd == 0 { return }
	var rect struct{ Left, Top, Right, Bottom int32 }
	procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect)))
	w.store.SetConfig("widget_pos_x", fmt.Sprintf("%d", rect.Left))
	w.store.SetConfig("widget_pos_y", fmt.Sprintf("%d", rect.Top))
}

var (
	dragging bool
	dX, dY   int32
	wX, wY   int32
)

func (w *Widget) windowProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmEraseBknd:
		return 1
	case wmCtlColorStatic:
		// lParam = HWND of the static control being painted
		// wParam = HDC
		ctrlHwnd := windows.Handle(lParam)
		w.mu.RLock()
		m := w.metrics
		w.mu.RUnlock()

		if ctrlHwnd == w.hProvider {
			// Provider name: muted gray
			procSetTextColor.Call(wParam, uintptr(0xCDD6F4)) // Catppuccin text (white)
		} else if ctrlHwnd == w.hBalance {
			// Balance: colored by amount
			if m != nil && m.Error == "" {
				if m.Balance < 10 {
					procSetTextColor.Call(wParam, uintptr(0xF38BA8)) // Catppuccin red
				} else {
					procSetTextColor.Call(wParam, uintptr(0xA6E3A1)) // Catppuccin green
				}
			} else {
				procSetTextColor.Call(wParam, uintptr(0x6C7086)) // gray
			}
		}
		procSetBkMode.Call(wParam, 1) // TRANSPARENT
		if w.hBgBrush != 0 { return w.hBgBrush }
		bg, _, _ := procCreateSolidBrush.Call(uintptr(0x1E1E2E))
		return bg

	case wmNcHitTest:
		return htCaption
	case wmLButtonDown:
		dragging = true
		dX = int32(lParam & 0xFFFF)
		dY = int32((lParam >> 16) & 0xFFFF)
		var rect struct{ Left, Top, Right, Bottom int32 }
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
		wX, wY = rect.Left, rect.Top
		procSetCapture.Call(hwnd)
		return 0
	case wmMouseMove:
		if !dragging { break }
		dx := int32(lParam&0xFFFF) - dX
		dy := int32((lParam>>16)&0xFFFF) - dY
		procMoveWindow.Call(hwnd, uintptr(wX+dx), uintptr(wY+dy), widgetW, widgetH, 1)
		return 0
	case wmLButtonUp:
		if dragging {
			dragging = false
			procReleaseCapture.Call()
			w.savePosition()
		}
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}
