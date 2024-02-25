package core

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-ping/ping"
	"github.com/google/uuid"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testzilla/core/entity"
	"testzilla/core/global"
	"testzilla/core/net_service"
	"time"
)

func isAvailable(ip string) bool {
	println("ping ip ->", ip)
	pinger, err := ping.NewPinger(ip)
	pinger.Timeout = 5 * time.Second
	if err != nil {
		println(err)
		return false
	}
	pinger.Count = 3
	err = pinger.Run() // Blocks until finished.
	if err != nil {
		println(err)
		return false
	}
	stats := pinger.Statistics() // get send/receive/duplicate/rtt stats
	if stats.PacketLoss > 0 {
		println("node with ip  -> " + ip + " is not available")
		return false
	} else {
		println("node with ip  -> " + ip + " available")
		return true
	}
}
func newTask(ip string, test entity.TestCase) {
	/*
		here we ask agent to run command below according to above test case
		Example
			curl http://agent_ip_address:8080/start?testid=251c2700-559f-40a1-8702-11ddd2b1f380&url=http://localhost:5656\&maxThread=50000\&maxRequest=1000000\&testDuration=5
	*/

failPoint:
	maxThread := strconv.Itoa(int(test.TestMaxThreadPerNode))
	maxReq := strconv.Itoa(int(test.TestMaxRequest))
	td := strconv.Itoa(test.TestDuration)
	extractUrl := strings.Split(ip, ":")
	requestUrl := "http://" + extractUrl[0] + ":8080/start?uuid=" + test.ID + "&url=" + test.TestProtocolName + "://" + test.TargetIP + ":" +
		test.TargetPort + "&maxThread=" + maxThread + "&maxRequest=" + maxReq +
		"&testDuration=" + td + "&method=" + test.TestProtocolOptions + "&testPostFileSize=" + test.TestHttpProtocolPostOptionsFileSize

	println("requested agent url  -> ", requestUrl)

	req, err := http.NewRequest(http.MethodGet, requestUrl, nil)
	if err != nil {
		fmt.Printf("client: could not create request: %s, try again", err)
		time.Sleep(5 * time.Second)
		goto failPoint
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("client: error making http request: %s, try again", err)
		time.Sleep(5 * time.Second)
		goto failPoint
	}

	fmt.Printf("client: got response!\n")
	fmt.Printf("client: status code: %d\n", res.StatusCode)

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("client: could not read response body: %s\n", err)
		time.Sleep(5 * time.Second)
		goto failPoint
	}
	fmt.Printf("client: response body for policy -> %s \n %s\n", test.ID, resBody)
	//todo here we are going to store resBody
	var reportObj entity.TestingReport
	reportObj.ReportID = uuid.New().String()
	reportObj.RelatedTestPolicyID = test.ID
	reportObj.TimeStamp = time.Now().Format("01-02-2006 15:04:05")
	reportObj.AgentIP = ip
	reportObj.TestResult = string(resBody)
	global.DBConnection.Create(&reportObj)
	entity.UpdateTestStatus(test, false, false, false, false, true)

}
func checkAgentHealth(url string) bool {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Printf("client: could not create request: %s, try again", err)
		return false
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}

	if res.StatusCode == 200 {
		resBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return false
		}
		if string(resBody) == "\"ok\"" {
			return true
		}
	}
	return false
}
func RunTestScenario(test entity.TestCase) bool {
	entity.UpdateTestStatus(test, false, true, false, false, false)
	totalIPList := strings.Split(test.NodeIPList, ",")

	// step 1: deploy and run agent
	for _, ip := range totalIPList {
		fmt.Println("Start to deploy agent on ->", ip)
		net_service.RunCommandOnAgent(test.SSHUserName, test.SSHPassword, ip, "killall agent")
		net_service.RunCommandOnAgent(test.SSHUserName, test.SSHPassword, ip, "killall agent")
		net_service.RunCommandOnAgent(test.SSHUserName, test.SSHPassword, ip, "killall agent")
		filePath := "/home/" + test.SSHUserName + "/Zilla"
		net_service.RunCommandOnAgent(test.SSHUserName, test.SSHPassword, ip, "mkdir "+filePath)
		net_service.RunCommandOnAgent(test.SSHUserName, test.SSHPassword, ip, "rm  "+filePath+"/agent")
		err, status := net_service.DeployAgent(test.SSHUserName, test.SSHPassword, ip, "./agent", filePath+"/agent")
		if status == false {
			println("error -> ", err)
			return false
		}
		net_service.RunCommandOnAgent(test.SSHUserName, test.SSHPassword, ip, "chmod +x "+filePath+"/agent")
		go net_service.RunCommandOnAgent(test.SSHUserName, test.SSHPassword, ip, filePath+"/agent agent_distributed")
	}

	// step 2:  heck if agents are up
	errorCount := 0
startPoint:
	for _, ip := range totalIPList {
		url := "http://" + ip + ":8080/health"
		if checkAgentHealth(url) == false {
			println("agent with ip -> ", ip, " is down,all nodes MUST be up to start test,  try again...")
			time.Sleep(3 * time.Second)
			errorCount++
			if errorCount >= 6 {
				return false
			}
			goto startPoint
		} else {
			println("agent with ip -> ", ip, " is up.")
		}
	}
	// step 3: run task on agents
	for _, ip := range totalIPList {
		sshPort := strconv.Itoa(test.SSHPort)
		sshServer := ip + ":" + sshPort
		go newTask(sshServer, test)
	}
	return true
}
func DownloadTestReport(ctx *gin.Context) {
	pass := false
	testID := ctx.Request.URL.Query().Get("uuid")

	var testInfo entity.TestCase
	global.DBConnection.Where("id =?", testID).Find(&testInfo)

	var fullReport []entity.TestingReport
	global.DBConnection.Where("related_test_policy_id =?", testID).Find(&fullReport)

	filename := "/tmp/" + uuid.New().String() + ".txt"
	ts := time.Now().Format("01-02-2006 15:04:05")
	maxRequest := strconv.Itoa(int(testInfo.TestMaxRequest))
	maxThreadPerNode := strconv.Itoa(int(testInfo.TestMaxThreadPerNode))
	i := 0
	f, err := os.Create(filename)
	if err != nil {
		pass = false
		goto jmpPoint
	}
	defer f.Close()
	_, _ = f.WriteString("TestZilla " + TestzillaVersion + "\n")
	_, _ = f.WriteString(ts + "\n")
	_, _ = f.WriteString("Test Details:\n")
	_, _ = f.WriteString("Test Name  [" + testInfo.TestName + "]\n")
	_, _ = f.WriteString("Candidate Protocol & Options  [" + testInfo.TestProtocolName + "," + testInfo.TestProtocolOptions + "]\n")
	_, _ = f.WriteString("Test Max Request  [" + maxRequest + "]\n")
	_, _ = f.WriteString("Test Max Request  [" + maxThreadPerNode + "]\n")
	_, _ = f.WriteString("Target (System Under Test)  [" + testInfo.TargetIP + ":" + testInfo.TargetPort + "]\n")
	_, _ = f.WriteString("Test Agent List [" + testInfo.NodeIPList + "]\n")
	_, _ = f.WriteString("======================[ Agent Activity Log  ]==================================\n")
	i = len(fullReport)
	for index, report := range fullReport {
		rowNumber := strconv.Itoa(index)
		_, _ = f.WriteString("[" + rowNumber + "] Agent IP [" + report.AgentIP + "]\n")
		_, _ = f.WriteString(report.TestResult)
		if index < i {
			println("\n")
		}

	}
	_, _ = f.WriteString("=============================================================================\n")
	_, _ = f.WriteString("Report End.\n\n\n")

	// Check if the file exists
	_, err = os.Stat(filename)
	if err != nil {
		pass = false
		goto jmpPoint
	}

	// Set headers for download
	ctx.Header("Content-Description", "File Transfer")
	ctx.Header("Content-Transfer-Encoding", "binary")
	ctx.Header("Content-Disposition", "attachment; filename="+filename)
	ctx.Header("Content-Type", "txt/plain")
	ctx.Header("Content-Length", "0")
	pass = true
	ctx.File(filename)
jmpPoint:
	if pass == true {
		ctx.Redirect(302, "/report")
	} else {
		ctx.Redirect(302, "/report?msg=failed to get test report for test "+testID)
	}
	return
}
func GetAgentReports(ctx *gin.Context) {
	//todo authentication and authorization
	data, err := io.ReadAll(ctx.Request.Body)

	if err != nil {
		return
	}
	agentIp := ctx.ClientIP()
	println("report from "+agentIp+" report body -> ", string(data))
	return
}
func PageRanger(ctx *gin.Context, data gin.H, templateName string) {

	switch ctx.Request.Header.Get("Accept") {
	case "application/json":
		ctx.JSON(http.StatusOK, data["payload"])
	case "application/xml":
		ctx.XML(http.StatusOK, data["payload"])
	default:
		ctx.HTML(http.StatusOK, templateName, data)
	}
}
func Index(ctx *gin.Context) {
	PageRanger(ctx,
		gin.H{},
		"index.html",
	)
}
func NewTestForm(ctx *gin.Context) {
	PageRanger(ctx,
		gin.H{},
		"new.html",
	)
}
func ShowTestForm(ctx *gin.Context) {
	var testsList []entity.TestCase
	global.DBConnection.Find(&testsList)
	PageRanger(ctx,
		gin.H{
			"allTest": testsList,
		},
		"report.html",
	)
}
func DeployAgentOnNodes(ctx *gin.Context) {
	//todo authentication and authorization
	//todo input and type validation

	var testName = ctx.PostForm("testName")

	// catch protocol name and options
	var testProtocolName = strings.ToLower(ctx.PostForm("protocolName"))
	var testProtocolOptions = strings.ToLower(ctx.PostForm("protocolOptions"))
	var testPostFileSize = ctx.PostForm("testPostFileSize")
	// validate protocol name
	pass := false
	for _, protocol := range ProtocolsList {
		if protocol == testProtocolName {
			pass = true
			break
		}
	}
	if pass == false {
		println(testProtocolName + " protocol option not support by now.")
		ctx.Redirect(302, "/new?msg="+testProtocolName+" protocol opt not support by now.")
	}

	// validate option
	pass = false
	for _, option := range ProtocolOption {
		if option == testProtocolOptions {
			pass = true
			break
		}
	}
	if pass == false {
		println(testProtocolOptions + " option not support by now.")
		ctx.Redirect(302, "/new?msg="+testProtocolOptions+" option not support by now.")
	}

	var testMaxThreadPerNode = ctx.PostForm("testMaxThreadPerNode")
	var testDuration = ctx.PostForm("testDuration")

	var testMaxRequest = ctx.PostForm("testMaxRequest")
	var testNodeIPList = ctx.PostForm("testNodeIPList")
	var testTargetIP = ctx.PostForm("targetIP")
	var testTargetPort = ctx.PostForm("targetPort")

	var sshUserName = ctx.PostForm("nodeSSHUserName")
	var sshPassword = ctx.PostForm("nodeSSHPassword")
	var sshPort = ctx.PostForm("nodeSSHPort")
	//todo input validation
	println("get TestName -> " + testName + " testMaxThreadPerNode -> " + testMaxThreadPerNode + " testMaxRequest -> " + testMaxRequest)
	if len(testNodeIPList) > 0 {
		/*
			e.g 192.168.1.1,192.168.2.2,192.168.3.3
		*/
		totalIPList := strings.Split(testNodeIPList, ",")

		// persist test scenario
		var newTest entity.TestCase
		newTest.ID = uuid.New().String()
		newTest.TimeStamp = time.Now().Format("01-02-2006 15:04:05")
		newTest.TestName = testName

		newTest.TestProtocolName = testProtocolName
		newTest.TestProtocolOptions = testProtocolOptions
		newTest.TestHttpProtocolPostOptionsFileSize = testPostFileSize

		tmr, _ := strconv.Atoi(testMaxRequest)
		newTest.TestMaxRequest = int64(tmr)
		tmt, _ := strconv.Atoi(testMaxThreadPerNode)
		newTest.TestMaxThreadPerNode = int64(tmt)
		newTest.NodeIPList = testNodeIPList

		td, _ := strconv.Atoi(testDuration)
		newTest.TestDuration = td

		newTest.TestStarted = true
		newTest.TestPassed = false
		newTest.TestRunning = false
		newTest.TestFinished = false
		newTest.TestFailed = false

		newTest.TargetIP = testTargetIP
		newTest.TargetPort = testTargetPort

		newTest.SSHUserName = sshUserName
		newTest.SSHPassword = sshPassword
		port, _ := strconv.Atoi(sshPort)
		newTest.SSHPort = port

		for _, ip := range totalIPList {
			sshPort = strconv.Itoa(newTest.SSHPort)
			sshServer := ip + ":" + sshPort
			if net_service.PingSSHServer(newTest.SSHUserName, newTest.SSHPassword, sshServer, "ls -hal") == false {
				///	if isAvailable(ip) == false {
				msg := ip + " is not available, please remove it and try again"
				println(msg)
				ctx.Redirect(302, "/new?msg="+msg)
				return
			}
		}
		global.DBConnection.Create(&newTest)
		if RunTestScenario(newTest) == false {
			entity.UpdateTestStatus(newTest, false, false, true, false, false)
			println("error while staring test, all agents seems not up, please check it")
			ctx.Redirect(302, "/new?msg=error while staring test, all agents seems not up, please check it")
		}
	} else {
		ctx.Redirect(302, "/new?msg=error, ip list is null or empty")
	}
	ctx.Redirect(302, "/report")
	return

}
