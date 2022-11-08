package main

import (
	"encoding/json"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var mainWindow fyne.Window
var fetchConf *FetchConf

var _fileLog *fetchLog

func bootGui() {
	logFile, err := os.OpenFile(AppExecDir()+"/log.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		_cliLog.Print("日志文件创建失败")
		return
	}
	_fileLog = &fetchLog{w: logFile}
	fetchConf = LoadFetchConf()
	logoResource := getLogoResource()
	a := app.New()
	a.Settings().SetTheme(&fghGuiTheme{})

	mainWindow = a.NewWindow(fmt.Sprintf("Fetch Github Hosts - V%.1f", VERSION))
	mainWindow.Resize(fyne.NewSize(800, 580))
	mainWindow.SetIcon(logoResource)

	logoImage := canvas.NewImageFromResource(logoResource)
	logoImage.SetMinSize(fyne.NewSize(240, 240))

	tabs := container.NewAppTabs(
		container.NewTabItem("客户端模式", guiClientMode()),
		container.NewTabItem("服务端模式", guiServerMode()),
		container.NewTabItem("关于", container.NewVBox(
			widget.NewLabel(""),
			container.New(layout.NewCenterLayout(), logoImage),
			guiAbout(),
		)),
	)

	mainWindow.SetCloseIntercept(func() {
//		mainWindow.Hide()
                runtime.Goexit()
	})

	mainWindow.CenterOnScreen()
	mainWindow.SetContent(container.NewVBox(tabs))

	if err := GetCheckPermissionResult(); err != nil {
		time.AfterFunc(time.Second, func() {
			showAlert(err.Error())
		})
	}


	trayMenu := fyne.NewMenu("TrayMenu", fyne.NewMenuItem("打开主界面", func() {
		mainWindow.Show()
	}))

	if desk, ok := a.(desktop.App); ok {
		desk.SetSystemTrayMenu(trayMenu)
		desk.SetSystemTrayIcon(logoResource)
	}

	mainWindow.ShowAndRun()
}

func getLogoResource() fyne.Resource {
	content, err := assetsFs.ReadFile("assets/public/logo.png")
	if err != nil {
		return nil
	}
	return &fyne.StaticResource{StaticName: "logo", StaticContent: content}
}

func getTicker(interval int) *time.Ticker {
	d := time.Minute
	if IsDebug() {
		d = time.Second
	}
	return time.NewTicker(d * time.Duration(interval))
}

func guiClientMode() (content fyne.CanvasObject) {
	logs, addFn := newLogScrollComponent(fyne.NewSize(800, 260))
	var cLog = NewFetchLog(NewGuiLogWriter(addFn))
	var startBtn, stopBtn *widget.Button
	var interval, customUrl, selectUrl = strconv.Itoa(fetchConf.Client.Interval), fetchConf.Client.CustomUrl, fetchConf.Client.SelectOrigin
	var isCustomOrigin bool
	intervalInput, urlInput := widget.NewEntryWithData(binding.BindString(&interval)), widget.NewEntryWithData(binding.BindString(&customUrl))
	var ticker *FetchTicker

	originSelectOpts := make([]string, 0, len(HostsOrigins))
	for k := range HostsOrigins {
		originSelectOpts = append(originSelectOpts, k)
	}

	originMethodOpts := []string{
		"官方指定hosts源",
		"自定义hosts源",
	}

	originSelect := widget.NewSelect(originSelectOpts, func(s string) {
		fetchConf.Client.SelectOrigin = s
		selectUrl = HostsOrigins[s]
	})
	originSelect.Selected = fetchConf.Client.SelectOrigin
	selectUrl = HostsOrigins[originSelect.Selected]

	intervalForm := widget.NewFormItem("获取间隔(分钟)", intervalInput)
	originSelectForm := widget.NewForm(widget.NewFormItem("hosts源", originSelect))

	originCustomForm := widget.NewForm(widget.NewFormItem("远程hosts链接", urlInput))

	if fetchConf.Client.Method == originMethodOpts[0] {
		originCustomForm.Hide()
	} else {
		originSelectForm.Hide()
	}

	var form *widget.Form
	originMethod := widget.NewRadioGroup(originMethodOpts, nil)
	originMethodForm := widget.NewFormItem("远程hosts来源", originMethod)
	originMethod.OnChanged = func(s string) {
		fetchConf.Client.Method = s
		if s == originMethodOpts[0] {
			originCustomForm.Hide()
			originSelectForm.Show()
			isCustomOrigin = false
		} else {
			originSelectForm.Hide()
			originCustomForm.Show()
			isCustomOrigin = true
		}
	}

	originMethod.Selected = fetchConf.Client.Method

	form = widget.NewForm(
		intervalForm,
		originMethodForm,
	)

	startBtn = widget.NewButton("启动", func() {
		intervalInt := parseStrIsNumberNotShowAlert(&interval, "获取间隔必须为整数")
		if intervalInt == nil {
			return
		}
		stopBtn.Enable()
		componentsStatusChange(false, startBtn, intervalInput, urlInput, originMethod, originSelect)
		ticker = NewFetchTicker(*intervalInt)
		if isCustomOrigin {
			go startClient(ticker, customUrl, cLog)
		} else {
			go startClient(ticker, selectUrl, cLog)
		}
		fetchConf.Client.CustomUrl = customUrl
		fetchConf.Client.Interval = *intervalInt
		fetchConf.Storage()
	})
	stopBtn = widget.NewButton("停止", func() {
		stopBtn.Disable()
		componentsStatusChange(true, startBtn, intervalInput, urlInput, originMethod, originSelect)
		ticker.Stop()
	})
	stopBtn.Disable()

	buttons := container.New(layout.NewGridLayout(3), startBtn, stopBtn, widget.NewButton("清除hosts", func() {
		if err := flushCleanGithubHosts(); err != nil {
			showAlert("清除hosts中的github记录失败：" + err.Error())
		} else {
			showAlert("hosts文件中的github记录已经清除成功！")
		}
	}))

	return container.NewVBox(widget.NewLabel(""), form, originSelectForm, originCustomForm, buttons, logs)
}

func guiServerMode() (content fyne.CanvasObject) {
	logs, addFn := newLogScrollComponent(fyne.NewSize(800, 260))
	var sLog = NewFetchLog(NewGuiLogWriter(addFn))
	var startBtn, stopBtn *widget.Button
	var interval, port = strconv.Itoa(fetchConf.Server.Interval), strconv.Itoa(fetchConf.Server.Port)
	var ticker *FetchTicker
	intervalInput, portInput := widget.NewEntryWithData(binding.BindString(&interval)), widget.NewEntryWithData(binding.BindString(&port))
	statusLabel := widget.NewHyperlink("监听地址：待启动", nil)
	startBtn = widget.NewButton("启动", func() {
		portInt := parseStrIsNumberNotShowAlert(&port, "端口号必须为整数")
		if portInt == nil {
			return
		}
		intervalInt := parseStrIsNumberNotShowAlert(&interval, "获取间隔必须为整数")
		if intervalInt == nil {
			return
		}
		stopBtn.Enable()
		componentsStatusChange(false, startBtn, intervalInput, portInput)
		ticker = NewFetchTicker(*intervalInt)
		go startServer(ticker, *portInt, sLog)
		fetchConf.Server.Interval = *intervalInt
		fetchConf.Server.Port = *portInt
		fetchConf.Storage()
		listenerUrl := fmt.Sprintf("http://127.0.0.1:%d", *portInt)
		statusLabel.SetText("监听地址：" + listenerUrl)
		statusLabel.SetURLFromString(listenerUrl)
	})
	stopBtn = widget.NewButton("停止", func() {
		stopBtn.Disable()
		componentsStatusChange(true, startBtn, intervalInput, portInput)
		statusLabel.SetText("监听地址：待启动")
		ticker.Stop()
	})
	stopBtn.Disable()
	form := widget.NewForm(
		widget.NewFormItem("获取间隔(分钟)", intervalInput),
		widget.NewFormItem("启动端口号", portInput),
	)
	buttons := container.New(layout.NewGridLayout(2), startBtn, stopBtn)
	return container.NewVBox(widget.NewLabel(""), form, buttons,
		container.New(layout.NewCenterLayout(), statusLabel),
		logs,
	)
}

func guiAbout() (content fyne.CanvasObject) {
	aboutNote := widget.NewRichTextFromMarkdown(`
# 版本号

` + fmt.Sprintf("V%.1f", VERSION))
	for i := range aboutNote.Segments {
		if seg, ok := aboutNote.Segments[i].(*widget.TextSegment); ok {
			seg.Style.Alignment = fyne.TextAlignCenter
		}
		if seg, ok := aboutNote.Segments[i].(*widget.HyperlinkSegment); ok {
			seg.Alignment = fyne.TextAlignCenter
		}
	}

	return container.NewVBox(aboutNote, container.New(layout.NewCenterLayout()))
}


func showAlert(msg string) {
	dialog.NewCustom("提示", "确认", widget.NewLabel(msg), mainWindow).Show()
}

func parseStrIsNumberNotShowAlert(str *string, msg string) *int {
	res, err := strconv.Atoi(*str)
	if err != nil {
		showAlert(msg)
		return nil
	}
	return &res
}

func newLogScrollComponent(size fyne.Size) (scroll *container.Scroll, addFn func(string)) {
	var logs string
	textarea := widget.NewMultiLineEntry()
	textarea.Wrapping = fyne.TextWrapBreak
	textarea.Disable()
	scroll = container.NewScroll(textarea)
	scroll.SetMinSize(size)
	addFn = func(s string) {
		logs = s + logs
		textarea.SetText(logs)
	}
	return
}

func componentsStatusChange(enable bool, components ...fyne.Disableable) {
	for _, v := range components {
		if enable {
			v.Enable()
		} else {
			v.Disable()
		}
	}
}

func openUrl(urlStr string) func() {
	return func() {
		u, _ := url.Parse(urlStr)
		_ = fyne.CurrentApp().OpenURL(u)
	}
}
