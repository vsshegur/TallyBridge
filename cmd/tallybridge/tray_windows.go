//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"tallybridge-live/internal/app"
	"unsafe"
)

const (
	wmDestroy       = 0x0002
	wmTimer         = 0x0113
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205
	wmUser          = 0x0400
	trayCallback    = wmUser + 41

	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002
	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	imageIcon      = 1
	lrLoadFromFile = 0x0010
	lrDefaultSize  = 0x0040

	mfString    = 0x0000
	mfGrayed    = 0x0001
	mfSeparator = 0x0800
	tpmRightBtn = 0x0002
	tpmReturnCmd = 0x0100

	cmdOpen        = 1001
	cmdDiagnostics = 1002
	cmdRestart     = 1003
	cmdExit        = 1004
)

type point struct { X int32; Y int32 }
type msg struct { Hwnd uintptr; Message uint32; WParam uintptr; LParam uintptr; Time uint32; Pt point }
type wndClass struct {
	Style uint32; WndProc uintptr; ClsExtra int32; WndExtra int32; Instance uintptr
	Icon uintptr; Cursor uintptr; Background uintptr; MenuName *uint16; ClassName *uint16
}
type notifyIconData struct {
	CbSize uint32; Hwnd uintptr; UID uint32; UFlags uint32; CallbackMessage uint32; HIcon uintptr
	Tip [128]uint16; State uint32; StateMask uint32; Info [256]uint16; VersionOrTimeout uint32
	InfoTitle [64]uint16; InfoFlags uint32; GuidItem [16]byte; BalloonIcon uintptr
}

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	shell32 = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClass       = user32.NewProc("RegisterClassW")
	procCreateWindowEx      = user32.NewProc("CreateWindowExW")
	procDefWindowProc       = user32.NewProc("DefWindowProcW")
	procGetMessage          = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessage     = user32.NewProc("DispatchMessageW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procLoadImage           = user32.NewProc("LoadImageW")
	procLoadIcon            = user32.NewProc("LoadIconW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenu          = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procRegisterWindowMsg   = user32.NewProc("RegisterWindowMessageW")
	procSetTimer            = user32.NewProc("SetTimer")
	procKillTimer           = user32.NewProc("KillTimer")
	procGetModuleHandle     = kernel32.NewProc("GetModuleHandleW")
	procShellNotifyIcon     = shell32.NewProc("Shell_NotifyIconW")

	trayAdminURL string
	trayDiagnosticsURL string
	trayStatus func() string
	trayLevel func() string
	trayIcons map[string]uintptr
	trayNID notifyIconData
	trayTaskbarCreated uint32
	trayExitOnce sync.Once
)

func utf16Ptr(s string) *uint16 { p,_:=syscall.UTF16PtrFromString(s); return p }
func fillUTF16(dst []uint16,s string){u:=syscall.StringToUTF16(s);if len(u)>len(dst){u=u[:len(dst)]};for i:=range dst{dst[i]=0};copy(dst,u)}
func addTrayIcon() bool {r,_,_:=procShellNotifyIcon.Call(nimAdd,uintptr(unsafe.Pointer(&trayNID)));return r!=0}
func removeTrayIcon(){_,_,_=procShellNotifyIcon.Call(nimDelete,uintptr(unsafe.Pointer(&trayNID)))}
func openTrayAdmin(){if trayAdminURL!=""{go app.OpenDesktop(trayAdminURL)}}
func currentTrayStatus() string {if trayStatus==nil{return "TallyBridge • read-only Tally companion"};s:=trayStatus();if len(s)>120{s=s[:120]};return s}
func currentTrayLevel() string {
	if trayLevel==nil{return "yellow"}
	switch trayLevel(){case "green","yellow","red":return trayLevel();default:return "yellow"}
}
func loadTrayIcon(path string) uintptr {
	if path==""{return 0}
	h,_,_:=procLoadImage.Call(0,uintptr(unsafe.Pointer(utf16Ptr(path))),imageIcon,0,0,lrLoadFromFile|lrDefaultSize)
	return h
}
func refreshTrayVisual(){
	fillUTF16(trayNID.Tip[:],currentTrayStatus())
	if h:=trayIcons[currentTrayLevel()];h!=0{trayNID.HIcon=h}
	trayNID.UFlags=nifMessage|nifIcon|nifTip
	_,_,_=procShellNotifyIcon.Call(nimModify,uintptr(unsafe.Pointer(&trayNID)))
}
func closeTray(hwnd uintptr){trayExitOnce.Do(func(){procKillTimer.Call(hwnd,1);removeTrayIcon();procDestroyWindow.Call(hwnd);procPostQuitMessage.Call(0)})}
func restartBridge(hwnd uintptr){
	exe,err:=os.Executable();if err!=nil{return}
	command:=fmt.Sprintf(`ping 127.0.0.1 -n 3 >nul & start "" "%s" --background`,exe)
	cmd:=exec.Command("cmd.exe","/C",command);cmd.SysProcAttr=&syscall.SysProcAttr{CreationFlags:0x08000000,HideWindow:true};_ = cmd.Start();closeTray(hwnd)
}
func showTrayMenu(hwnd uintptr){
	refreshTrayVisual()
	menu,_,_:=procCreatePopupMenu.Call();if menu==0{return};defer procDestroyMenu.Call(menu)
	status:=currentTrayStatus()
	procAppendMenu.Call(menu,mfString|mfGrayed,0,uintptr(unsafe.Pointer(utf16Ptr(status))))
	procAppendMenu.Call(menu,mfSeparator,0,0)
	procAppendMenu.Call(menu,mfString,cmdOpen,uintptr(unsafe.Pointer(utf16Ptr("Open TallyBridge"))))
	procAppendMenu.Call(menu,mfString,cmdDiagnostics,uintptr(unsafe.Pointer(utf16Ptr("Open Diagnostics"))))
	procAppendMenu.Call(menu,mfString,cmdRestart,uintptr(unsafe.Pointer(utf16Ptr("Restart Bridge"))))
	procAppendMenu.Call(menu,mfSeparator,0,0)
	procAppendMenu.Call(menu,mfString,cmdExit,uintptr(unsafe.Pointer(utf16Ptr("Exit TallyBridge"))))
	var p point;procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)));procSetForegroundWindow.Call(hwnd)
	cmd,_,_:=procTrackPopupMenu.Call(menu,tpmRightBtn|tpmReturnCmd,uintptr(p.X),uintptr(p.Y),0,hwnd,0)
	switch cmd{case cmdOpen:openTrayAdmin();case cmdDiagnostics:if trayDiagnosticsURL!=""{go app.OpenDesktop(trayDiagnosticsURL)};case cmdRestart:restartBridge(hwnd);case cmdExit:closeTray(hwnd)}
}
func trayWndProc(hwnd uintptr,message uint32,wParam,lParam uintptr) uintptr {
	if trayTaskbarCreated!=0&&message==trayTaskbarCreated{addTrayIcon();refreshTrayVisual();return 0}
	switch message{
	case trayCallback:
		switch uint32(lParam){case wmLButtonDblClk:openTrayAdmin();return 0;case wmRButtonUp:showTrayMenu(hwnd);return 0}
	case wmTimer:
		refreshTrayVisual();return 0
	case wmDestroy:
		removeTrayIcon();procPostQuitMessage.Call(0);return 0
	}
	r,_,_:=procDefWindowProc.Call(hwnd,uintptr(message),wParam,lParam);return r
}

func runSystemTray(adminURL,diagnosticsURL,iconPath string,status func() string,level func() string) error {
	runtime.LockOSThread();defer runtime.UnlockOSThread()
	trayAdminURL=adminURL;trayDiagnosticsURL=diagnosticsURL;trayStatus=status;trayLevel=level;trayExitOnce=sync.Once{}
	instance,_,_:=procGetModuleHandle.Call(0);className:=utf16Ptr("TallyBridgeTrayWindow");wndProc:=syscall.NewCallback(trayWndProc)
	wc:=wndClass{WndProc:wndProc,Instance:instance,ClassName:className}
	if r,_,e:=procRegisterClass.Call(uintptr(unsafe.Pointer(&wc)));r==0&&e!=syscall.Errno(0){return e}
	hwnd,_,e:=procCreateWindowEx.Call(0,uintptr(unsafe.Pointer(className)),uintptr(unsafe.Pointer(utf16Ptr("TallyBridge"))),0,0,0,0,0,0,0,instance,0)
	if hwnd==0{return e}

	base:=loadTrayIcon(iconPath)
	if base==0{base,_,_=procLoadIcon.Call(0,32512)}
	dir:=filepath.Dir(iconPath)
	trayIcons=map[string]uintptr{
		"green":loadTrayIcon(filepath.Join(dir,"TallyBridge-green.ico")),
		"yellow":loadTrayIcon(filepath.Join(dir,"TallyBridge-yellow.ico")),
		"red":loadTrayIcon(filepath.Join(dir,"TallyBridge-red.ico")),
	}
	for _,k:=range []string{"green","yellow","red"}{if trayIcons[k]==0{trayIcons[k]=base}}
	trayNID=notifyIconData{Hwnd:hwnd,UID:1,UFlags:nifMessage|nifIcon|nifTip,CallbackMessage:trayCallback,HIcon:trayIcons[currentTrayLevel()]}
	trayNID.CbSize=uint32(unsafe.Sizeof(trayNID));fillUTF16(trayNID.Tip[:],currentTrayStatus())
	if !addTrayIcon(){procDestroyWindow.Call(hwnd);return errors.New("could not add TallyBridge system tray icon")}
	taskbarMsg:=utf16Ptr("TaskbarCreated");if v,_,_:=procRegisterWindowMsg.Call(uintptr(unsafe.Pointer(taskbarMsg)));v!=0{trayTaskbarCreated=uint32(v)}
	procSetTimer.Call(hwnd,1,10000,0)
	var m msg
	for{r,_,_:=procGetMessage.Call(uintptr(unsafe.Pointer(&m)),0,0,0);if int32(r)<=0{break};procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)));procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))}
	removeTrayIcon();return nil
}
