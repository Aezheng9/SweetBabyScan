package task_scan_host

import (
	"SweetBabyScan/core/plugins/plugin_scan_host"
	"SweetBabyScan/models"
	"SweetBabyScan/utils"
	"bufio"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type taskScanHost struct {
	params models.Params
}

type IpRangeStruct struct {
	Key   string
	Value int
}

var ip []string
var (
	IpRange = map[string]int{}
)

// 1.迭代方法
func (t *taskScanHost) doIter(wg *sync.WaitGroup, worker chan bool, result chan utils.CountResult, task utils.Task, data ...interface{}) {
	items := data[0]
	for _, item := range items.([]string) {
		wg.Add(1)
		worker <- true
		go task(wg, worker, result, item)
	}
}

// 2.任务方法
func (t *taskScanHost) doTask(wg *sync.WaitGroup, worker chan bool, result chan utils.CountResult, data ...interface{}) {
	defer wg.Done()
	item := data[0].(string)
	status := false
	methodAlive := t.params.MethodScanHost
	iFace := t.params.IFace
	mac := ""
	if methodAlive == "ICMP" {
		status = plugin_scan_host.ScanHostByICMP(item, time.Duration(t.params.TimeOutScanHost)*time.Second)
	} else if methodAlive == "PING" {
		status = plugin_scan_host.ScanHostByPing(item)
	} else if methodAlive == "ARP" {
		status, mac = plugin_scan_host.ScanHostByARP(item, iFace, time.Duration(t.params.TimeOutScanHost)*time.Second)
	}
	if status {
		result <- utils.CountResult{
			Count: 1,
			Result: models.ScanHost{
				Ip:      item,
				IpNum:   uint(utils.InetAtoN(item)),
				IpRange: strings.Join(strings.Split(item, ".")[0:3], ".") + ".1/24",
				Mac:     mac,
			},
		}
	} else {
		result <- utils.CountResult{
			Count:  1,
			Result: nil,
		}
	}
	<-worker
}

// 3.保存结果
func (t *taskScanHost) doDone(item interface{}, buf *bufio.Writer) error {
	result := item.(models.ScanHost)
	if _, ok := IpRange[result.IpRange]; ok {
		IpRange[result.IpRange] += 1
	} else {
		IpRange[result.IpRange] = 1
	}
	ip = append(ip, result.Ip)

	buf.WriteString(fmt.Sprintf(`%s`, result.Ip) + "\n")

	if t.params.IsLog {
		fmt.Println(fmt.Sprintf("[+]发现存活主机 %s", result.Ip))
	}

	return nil
}

// 4.记录数量
func (t *taskScanHost) doAfter(data uint) {

}

// 执行并发存活检测
func DoTaskScanHost(req models.Params) []string {
	task := taskScanHost{params: req}

	totalTask := uint(len(req.IPs))
	totalTime := uint(math.Ceil(float64(totalTask)/float64(req.WorkerScanHost)) * float64(req.TimeOutScanHost))

	utils.MultiTask(
		totalTask,
		uint(req.WorkerScanHost),
		totalTime,
		req.IsLog,
		task.doIter,
		task.doTask,
		task.doDone,
		task.doAfter,
		fmt.Sprintf(
			"开始主机存活检测\r\n\r\n> 存活并发：%d\r\n> 存活超时：%d\r\n> 检测方式：%s\r\n> 出口网卡：%s\r\n> 检测网段：%s\r\n> 排除网段：%s\r\n",
			req.WorkerScanHost,
			req.TimeOutScanHost,
			req.MethodScanHost,
			req.IFace,
			req.Host,
			req.HostBlack,
		),
		//"主机存活检测中",
		"完成主机存活检测",
		"ip.txt",
		req.IPs,
	)

	var listIpRange []IpRangeStruct
	total := 0
	for k, v := range IpRange {
		listIpRange = append(listIpRange, IpRangeStruct{Key: k, Value: v})
		total += v
	}
	sort.Slice(listIpRange, func(i, j int) bool {
		return listIpRange[i].Value > listIpRange[j].Value
	})

	fmt.Println(fmt.Sprintf("共发现：%d个IP", total))
	for _, v := range listIpRange {
		fmt.Println(fmt.Sprintf(`%-18s ->  %d`, v.Key, v.Value))
	}
	fmt.Println(fmt.Sprintf("共发现：%d个网段", len(listIpRange)))
	return ip
}