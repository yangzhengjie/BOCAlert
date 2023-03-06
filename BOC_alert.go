package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	port    int
	host    string
	system  string
	version bool
	h       bool
)

/**
 运行方式: go run BOC_alert.go  -host 172.16.6.194 -port 8899 -system C-TIQ,C-PSI,C-CBD
 20230217 初始版本
 20230227
		1 原SystemName显示TiDB集群名称，现在SystemName 统一改为C-TIQ,C-PSI,C-CBD
		2 磁盘告警，需要增加告警磁盘的挂载点,涉及到的告警有:NODE_disk_used_more_than_80%,NODE_disk_used_more_than_90%,NODE_disk_used_more_than_95%
		3 调整告警转发程序打印日志的内容，打印完整的告警信息。
*/
func init() {
	flag.IntVar(&port, "port", 8899, "alert server port")
	flag.StringVar(&host, "host", "0.0.0.0", "alert server host")
	flag.StringVar(&system, "system", "C-TIQ", "alert server host")

	flag.BoolVar(&h, "h", false, "this help")
	flag.BoolVar(&version, "version", false, "view alert_trigger version info")
	log.SetPrefix("TRACE: ")
	log.SetFlags(log.Ldate | log.Lmicroseconds)
	tr = &http.Transport{MaxIdleConns: 100}
}

type Alerts []Alert
type KV map[string]string
type Alert struct {
	Status       string    `json:"status"`
	Labels       KV        `json:"labels"`
	Annotations  KV        `json:"-"`
	StartsAt     time.Time `json:"startsAt"`
	EndsAt       time.Time `json:"endsAt"`
	GeneratorURL string    `json:"-"`
	Fingerprint  string    `json:"fingerprint"`
}
type Data struct {
	Receiver          string `json:"receiver"`
	Status            string `json:"status"`
	Alerts            Alerts `json:"alerts"`
	GroupLabels       KV     `json:"groupLabels"`
	CommonLabels      KV     `json:"commonLabels"`
	CommonAnnotations KV     `json:"commonAnnotations"`
	ExternalURL       string `json:"externalURL"`
}

var tr *http.Transport

type BOCAlert struct {
	//告警消息列定义:原始告警级别|当前级别|告警团队|主机名|IP地址|系统名称|告警发生时间秒串|告警改变时间秒串|告警内容msg
	OriginalSeverity string
	/*
		中心BPPM和一体化监控平台告警级别，
		1级 紧急 CRITICAL（红色，短信+告警单）
		2级 重要 MAJOR（橙色，短信+告警单）
		3级 次要 MINOR（黄色，短信）
		4级 提示 WARNING（青色）
		5级 正常 INFO（绿色）
	*/
	CurrentSeverity  string
	Team             string
	Host             string
	Ip               string
	SystemName       string
	ArrivalTime      string
	DateModification string
	Message          string
}

func (alert *BOCAlert) String() string {
	//定义发送一条告警的格式，以\n结尾
	return alert.OriginalSeverity + "|" + alert.CurrentSeverity + "|" + alert.Team + "|" + alert.Host + "|" + alert.Ip + "|" + alert.SystemName + "|" + alert.ArrivalTime + "|" + alert.DateModification + "|" + alert.Message + "\n"
}
func SendTcpMessage(alertString string) {
	//fmt.Println("开始发送tcp消息")
	var remoteAddress, _ = net.ResolveTCPAddr("tcp", host+":"+strconv.Itoa(port)) //生成一个net.TcpAddr对像。
	var conn, err = net.DialTCP("tcp", nil, remoteAddress)                        //传入协议，本机地址（传了nil），远程地址，获取连接。

	if err != nil { //如果连接失败。则返回。
		fmt.Println("连接出错：", err)
		return
	}

	conn.Write([]byte(alertString)) //发送信息。
	conn.Close()
	fmt.Println("Send Time:", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Print("Send Info:", alertString)
	//fmt.Println("发送程序结束")
}
func handlerAlert(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	data := Data{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Printf("error decoding alert response: %v", err)
		if e, ok := err.(*json.SyntaxError); ok {
			log.Printf("syntax error at byte offset %d", e.Offset)
		}
		log.Printf("alert response: %q", r.Body)
		return
	}
	//byteData, _ := json.Marshal(&data)
	//fmt.Println("开始打印告警消息")
	//fmt.Println(string(byteData))
	//fmt.Println(jsonToAlert(&data))
	//SendTcpMessage(jsonToAlert(&data))
	arrLen := len(data.Alerts)
	//fmt.Println("告警数量",arrLen)
	i := 1
	//for i := 1; i <= arrLen; i++ {
	if arrLen == i {
		bocAlert := BOCAlert{}
		//arrLevel 存放拆分后的告警级别|告警团队名称
		arrLevel := strings.Split(data.Alerts[arrLen-i].Labels["level"], "|")
		bocAlert.OriginalSeverity = arrLevel[0]
		bocAlert.CurrentSeverity = arrLevel[0]
		bocAlert.Team = arrLevel[1]
		//arrLevel 存放拆分后的IP:端口
		arrInstance := strings.Split(data.GroupLabels["instance"], ":")
		bocAlert.Host = arrInstance[0]
		bocAlert.Ip = arrInstance[0]
		//系统名称
		bocAlert.SystemName = system
		bocAlert.ArrivalTime = strconv.FormatInt(data.Alerts[arrLen-i].StartsAt.Unix(), 10)
		//无改变时间，获取当前时间
		bocAlert.DateModification = strconv.FormatInt(time.Now().Unix(), 10)
		//获取告警名称
		alertName := data.Alerts[arrLen-i].Labels["alertname"]
		//对于文件系统告警，添加文件系统的挂载点
		switch alertName {
		case "NODE_disk_used_more_than_80%", "NODE_disk_used_more_than_90%", "NODE_disk_used_more_than_95%":
			mountPoint := data.Alerts[arrLen-i].Labels["mountpoint"]
			bocAlert.Message = "【Tidb】告警名称:" + alertName + " 挂载点:" + mountPoint + " 集群:" + data.CommonLabels["cluster"] + " 当前状态:" + data.Alerts[arrLen-i].Status + " 当前值:" + data.CommonAnnotations["value"]
			//fmt.Print(data.Alerts[arrLen-i].Labels["alertname"] + " 挂载点:" + mountPoint)
		default:
			bocAlert.Message = "【Tidb】告警名称:" + alertName + " 集群:" + data.CommonLabels["cluster"] + " 当前状态:" + data.Alerts[arrLen-i].Status + " 当前值:" + data.CommonAnnotations["value"]
			//fmt.Print(data.Alerts[arrLen-i].Labels["alertname"])
		}
		SendTcpMessage(bocAlert.String())

	}

}

func main() {
	flag.Parse()
	fmt.Println("port:", port)
	fmt.Println("host:", host)
	fmt.Println("system:", system)
	// fmt.Println("version:", version)
	if h {
		flag.Usage()
		os.Exit(0)
	}
	http.HandleFunc("/send", handlerAlert)
	fmt.Println("start BOC_alert webhook:..............")
	err := http.ListenAndServe("0.0.0.0:8000", nil)
	if err != nil {
		fmt.Println("http listen failed")
	}

}
