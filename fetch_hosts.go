package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/h2non/filetype"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	Windows = "windows"
	Linux   = "linux"
	Darwin  = "darwin"
)

func startClient(ticker *FetchTicker, url string, flog *fetchLog) {
	flog.Print("远程hosts获取链接：" + url)
	fn := func() {
		if err := ClientFetchHosts(url); err != nil {
			flog.Print("更新Hosts失败：" + err.Error())
		} else {
			flog.Print("更新Hosts成功！")
		}
	}
	fn()
	for {
		select {
		case <-ticker.Ticker.C:
			fn()
		case <-ticker.CloseChan:
			flog.Print("停止获取hosts")
			return
		}
	}
}

func startServer(ticker *FetchTicker, port int, flog *fetchLog) {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Println("服务启动失败（可能是目标端口已被占用）：", err.Error())
		return
	}
	flog.Print(fmt.Sprintf("已监听HTTP服务成功：http://127.0.0.1:%d", port))
	flog.Print(fmt.Sprintf("hosts文件链接：http://127.0.0.1:%d/hosts.txt", port))
	flog.Print(fmt.Sprintf("hosts的JSON格式链接：http://127.0.0.1:%d/hosts.json", port))
	go http.Serve(listen, &serverHandle{flog})
	fn := func() {
		if err := ServerFetchHosts(); err != nil {
			flog.Print("执行更新Hosts失败：" + err.Error())
		} else {
			flog.Print("执行更新Hosts成功！")
		}
	}
	fn()
	for {
		select {
		case <-ticker.Ticker.C:
			fn()
		case <-ticker.CloseChan:
			flog.Print("正在停止更新hosts服务")
			if err := listen.Close(); err != nil {
				flog.Print("关闭端口监听失败")
			}
			flog.Print("已停止更新hosts服务")
			return
		}
	}
}

type serverHandle struct {
	flog *fetchLog
}

func (s *serverHandle) ServeHTTP(resp http.ResponseWriter, request *http.Request) {
	p := request.URL.Path
	if p == "/" || p == "/hosts.txt" || p == "/hosts.json" {
		if p == "/" {
			p = "/index.html"
		}
		file, err := ioutil.ReadFile(AppExecDir() + p)
		if err != nil {
			resp.WriteHeader(http.StatusInternalServerError)
			resp.Write([]byte("server error"))
			s.flog.Print("获取首页文件失败: " + err.Error())
			return
		}
		resp.Write(file)
		return
	}
	if strings.HasPrefix(p, "/public/") {
		file, _ := assetsFs.ReadFile("assets" + p)
		kind, _ := filetype.Match(file)
		resp.Header().Set("Content-Type", kind.MIME.Value)
		resp.Write(file)
		return
	}
	http.Redirect(resp, request, "/", http.StatusMovedPermanently)
}

// ClientFetchHosts 获取最新的host并写入hosts文件
func ClientFetchHosts(url string) (err error) {
	hosts, err := getCleanGithubHosts()
	if err != nil {
		return
	}

	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		err = ComposeError("获取最新的hosts失败", err)
		return
	}

	fetchHosts, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		err = ComposeError("读取最新的hosts失败", err)
		return
	}

	newlineChar := GetNewlineChar()

	fetchHostsLines := strings.Split(string(fetchHosts), "\n")

	for i, fetchLine := range fetchHostsLines {
		line := strings.TrimSpace(fetchLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		hosts.WriteString(fetchLine)
		if i != len(fetchHostsLines)-1 {
			hosts.WriteString(newlineChar)
		}
	}
	if err = ioutil.WriteFile(GetSystemHostsPath(), hosts.Bytes(), os.ModeType); err != nil {
		err = ComposeError("写入hosts文件失败，请用超级管理员身份启动本程序！", err)
		return
	}

	return
}

// ServerFetchHosts 服务端获取github最新的hosts并写入到对应文件及更新首页
func ServerFetchHosts() (err error) {
	execDir := AppExecDir()
	domains, err := getGithubDomains()
	if err != nil {
		return
	}

	hostJson, hostFile, now, err := FetchHosts(domains)
	if err != nil {
		err = ComposeError("获取Host失败", err)
		return
	}

	if err = ioutil.WriteFile(execDir+"/hosts.json", hostJson, 0775); err != nil {
		err = ComposeError("写入数据到hosts.json文件失败", err)
		return
	}

	if err = ioutil.WriteFile(execDir+"/hosts.txt", hostFile, 0775); err != nil {
		err = ComposeError("写入数据到hosts.txt文件失败", err)
		return
	}

	var templateFile []byte
	templateFile, err = GetExecOrEmbedFile(&assetsFs, "assets/index.html")
	if err != nil {
		err = ComposeError("读取首页模板文件失败", err)
		return
	}

	templateData := strings.Replace(string(templateFile), "<!--time-->", now, 1)
	if err = ioutil.WriteFile(execDir+"/index.html", []byte(templateData), 0775); err != nil {
		err = ComposeError("写入更新信息到首页文件失败", err)
		return
	}

	return
}

func FetchHosts(domains []string) (hostsJson, hostsFile []byte, now string, err error) {
	now = time.Now().Format("2006-01-02 15:04:05")
	hosts := make([][]string, 0, len(domains))
	hostsFileData := bytes.NewBufferString("# fetch-github-hosts begin\n")
	for _, domain := range domains {
		host, err := net.LookupHost(domain)
		if err != nil {
			fmt.Println("获取主机记录失败: ", err.Error())
			continue
		}
		item := []string{host[0], domain}
		hosts = append(hosts, item)
		hostsFileData.WriteString(fmt.Sprintf("%-28s%s\n", item[0], item[1]))
	}
	hostsFileData.WriteString("# last update time: ")
	hostsFileData.WriteString(now)
	hostsFileData.WriteString("\n# update url: http://106.52.55.138/hosts/hosts.txt\n# MiniYun-hosts end\n\n")
	hostsFile = hostsFileData.Bytes()
	hostsJson, err = json.Marshal(hosts)
	return
}

func getCleanGithubHosts() (hosts *bytes.Buffer, err error) {
	hostsPath := GetSystemHostsPath()
	hostsBytes, err := ioutil.ReadFile(hostsPath)
	if err != nil {
		err = ComposeError("读取文件hosts错误", err)
		return
	}

	domains, err := getGithubDomains()
	if err != nil {
		return
	}

	newlineChar := GetNewlineChar()

	// clear local hosts github domain
	localHostsLines := strings.Split(string(hostsBytes), newlineChar)
	hosts = &bytes.Buffer{}

	for i, localLine := range localHostsLines {
		line := strings.TrimSpace(localLine)
		if line == "" || strings.HasPrefix(line, "#") {
			hosts.WriteString(localLine)
			if i != len(localHostsLines)-1 || !strings.HasSuffix(hosts.String(), newlineChar) {
				hosts.WriteString(newlineChar)
			}
			continue
		}
		var clearLine bool
		for _, domain := range domains {
			if strings.Contains(line, domain) {
				clearLine = true
				break
			}
		}
		if !clearLine {
			hosts.WriteString(localLine)
			hosts.WriteString(newlineChar)
		}
	}

	return
}

func getGithubDomains() (domains []string , err error) {
        dojson = "http://106.52.55.138/hosts/domains.json"

	resp, err := http.Get(dojson)
	if err != nil || resp.StatusCode != http.StatusOK {
		err = ComposeError("获取domains.json失败", err)
		return
	}

	fetchData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		err = ComposeError("读取domains.json失败", err)
		return
	}
        fileData = []byte(string(fetchData))
	if err = json.Unmarshal(fileData, &domains); err != nil {
		err = ComposeError("domain.json解析失败", err)
		return
	}
	return
}

func flushCleanGithubHosts() (err error) {
	hosts, err := getCleanGithubHosts()
	if err != nil {
		return
	}
	if err = ioutil.WriteFile(GetSystemHostsPath(), hosts.Bytes(), os.ModeType); err != nil {
		err = ComposeError("写入hosts文件失败，请用超级管理员身份启动本程序！", err)
	}
	return
}
