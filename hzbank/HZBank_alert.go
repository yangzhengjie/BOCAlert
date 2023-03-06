package hzbank

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"
)

var (
	port     int
	filePath string
	version  bool
	h        bool
)

func init() {
	flag.IntVar(&port, "port", 8080, "web server port")
	flag.StringVar(&filePath, "filepath", "", "scripts file path")
	flag.BoolVar(&h, "h", false, "this help")
	flag.BoolVar(&version, "version", false, "view alert_trigger version info")
	log.SetPrefix("TRACE: ")
	log.SetFlags(log.Ldate | log.Lmicroseconds)
	tr = &http.Transport{MaxIdleConns: 100}
}

var tr *http.Transport

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

func (data *Data) toStringForHZBank() []string {
	res := []string{}
	for _, a := range data.Alerts {
		if a.Status == "firing" {
			alertStr := fmt.Sprintf("Cluster:%s firing a alert:%s at %s from %s", a.Labels["cluster"], a.Labels["alertname"], a.StartsAt.Format("2006-01-02 15:04:05"), a.Labels["instance"])
			res = append(res, alertStr)
		} else if a.Status == "resolved" {
			alertStr := fmt.Sprintf("Cluster:%s resolved a alert:%s at %s. start at %s from %s", a.Labels["cluster"], a.Labels["alertname"], a.EndsAt.Format("2006-01-02 15:04:05"), a.StartsAt.Format("2006-01-02 15:04:05"), a.Labels["instance"])
			res = append(res, alertStr)
		}
		// alertStr := fmt.Sprintf("{status:%s labels:%+v StartsAt:%+v EndsAt:%+v}", a.Status, a.Labels, a.StartsAt, a.EndsAt)
		// alertStr = strings.Replace(alertStr, "map[", "[", -1)
		// res = append(res, alertStr)
	}
	return res
}

type ApiData struct {
	//短信接口配置文件,
	Severity         string `json:"Severity"`         //它指示受管对象观察能力的受影响程度。事件列表中警报的颜色受严重性值控制：0：清除 1：不确定 2：警告 3：次要 4：主要 5：紧急`
	Summary          string `json:"Summary"`          //文字描述`
	Status           string `json:"Status"`           //1，OPEN，默认状态为1     2，CLOSE，事件关闭标示
	SourceEventId    string `json:"SourceEventId"`    //监控数据源的事件序列号或其他主键字段`
	SourceAlertKey   string `json:"SourceAlertKey"`   //发送的告警消息ID，告警指标的唯一标示，如situation名称：OCBS_CICS_SysDumps_Crit，与指标管理信息SourceName字段相同`
	SourceIdentifier string `json:"SourceIdentifier"` //用于标示唯一事件的压缩标示字段，双向同步过程中，集中事件管理平台会将该字段回传给事件源。对应Netcool中的Identifier字段。如果无法提供填“”字符`
	LastOccurrence   string `json:"LastOccurrence"`   //如事件源不发送，则取接收时间填充.,格式MM/DD/YYYY   hh:mm:ss`
	SourceId         string `json:"SourceId"`         //集成时只用发送代码数字即可1：开放平台监控2：大机监控3：网管5：机房环境监控6：设备硬件监控7：云平台监控8：应用交易监控，新增事件源需提前与管理员联系确认事件源编号`
	SourceCIName     string `json:"SourceCIName"`     //如设备名、主机名或IP等，与配置管理信息CIName字段相同` //模板文件路径
	SourceSeverity   string `json:"SourceSeverity"`
}

func (conf *ApiData) String() string {
	b, err := json.Marshal(*conf)
	if err != nil {
		return fmt.Sprintf("%+v", *conf)
	}
	var out bytes.Buffer
	err = json.Indent(&out, b, "", "    ")
	if err != nil {
		return fmt.Sprintf("%+v", *conf)
	}
	return out.String()
}
func Command(cmd string) (string, error) {
	//c := exec.Command("cmd", "/C", cmd)   // windows
	c := exec.Command("bash", "-c", cmd) // mac or linux
	stdout_reader, err := c.StdoutPipe()
	s, err := ioutil.ReadAll(stdout_reader)
	return string(s), err
}

func GenApiData(promData Data) *ApiData {
	//发送给短信接口的内容
	apiData := &ApiData{}
	switch promData.GroupLabels["level"] {
	case "critical":
		apiData.Severity = "5"
	case "emergency":
		apiData.Severity = "4"
	case "warning":
		apiData.Severity = "3"
	default:
		apiData.Severity = "2"
	}
	apiData.Summary = promData.CommonAnnotations["summary"]
	if promData.Status == "firing" {
		apiData.Status = "1"
	} else {
		apiData.Status = "2"
	}
	apiData.SourceEventId = "7"
	apiData.SourceAlertKey = promData.GroupLabels["alertname"]
	arrLen := len(promData.Alerts)
	timeTemplate1 := "2006-01-02 15:04:05"
	apiData.LastOccurrence = promData.Alerts[arrLen-1].StartsAt.Format(timeTemplate1)
	apiData.SourceId = "7"
	apiData.SourceCIName = promData.GroupLabels["instance"]
	apiData.SourceSeverity = promData.GroupLabels["level"]
	return apiData
}
func PostSms(apiData *ApiData) (resp *http.Response, err error) {
	//log.Println(apiData)
	client := &http.Client{
		Transport: tr,
		Timeout:   3 * time.Second, // 超时加在这里，是每次调用的超时
	}
	res, err := client.PostForm("http://128.192.138.103:8101/iomp-mon/services/event/sendevent3", url.Values{
		"Severity":       {apiData.Severity},
		"Summary":        {apiData.Summary},
		"Status":         {apiData.Status},
		"SourceEventId":  {apiData.SourceEventId},
		"SourceAlertKey": {apiData.SourceAlertKey},
		"SourceId":       {apiData.SourceId},
		"LastOccurrence": {apiData.LastOccurrence},
		"SourceCIName":   {apiData.SourceCIName},
		"SourceSeverity": {apiData.SourceSeverity},
	})
	if err != nil {
		log.Println(apiData.String())
		log.Fatal(err)
	}
	body, err := ioutil.ReadAll(res.Body)
	log.Println(body)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	//return resp.Body
	return res, nil
}
func handler1(w http.ResponseWriter, r *http.Request) {
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
	if _, err := PostSms(GenApiData(data)); err != nil {
		log.Println("send sms ERROR: " + err.Error())
		return
	}
}
func handlerhzbank(w http.ResponseWriter, r *http.Request) {
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
	cmd_str := fmt.Sprintf("%s \"%+v\"", filePath, data.toStringForHZBank())
	log.Printf("exec bash command: %s", cmd_str)
	cmd := exec.Command("/bin/bash", "-c", cmd_str)

	_, err := cmd.Output()
	if err != nil {
		log.Printf("Execute Shell:%s failed with error:%s", cmd_str, err.Error())
		return
	}
}
func main() {
	//  var v map[string]interface{}
	flag.Parse()
	//service.GetAppVersion(version)
	if h {
		flag.Usage()
		os.Exit(0)
	}
	log.Println("starting handler")
	http.HandleFunc("/sms", handler1)
	http.HandleFunc("/hzbank", handlerhzbank)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Fatalln(err)
		return
	}
}
