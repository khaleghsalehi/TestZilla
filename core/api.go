package core

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-ping/ping"
	"io"
	"net/http"
	"strings"
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
func newTask(ip string) {
	fmt.Println("Start to deploy agent on ->", ip)
	net_service.RunCommandOnAgent("tomcat", "khalegh 123", ip, "killall agent")
	net_service.RunCommandOnAgent("tomcat", "khalegh 123", ip, "killall agent")
	net_service.RunCommandOnAgent("tomcat", "khalegh 123", ip, "killall agent")
	net_service.RunCommandOnAgent("tomcat", "khalegh 123", ip, "rm  /home/tomcat/Zilla/agent")
	net_service.DeployAgent("tomcat", "khalegh 123", ip, "./testzilla", "/home/tomcat/Zilla/agent")
	net_service.RunCommandOnAgent("tomcat", "khalegh 123", ip, "chmod +x /home/tomcat/Zilla/agent")
	net_service.RunCommandOnAgent("tomcat", "khalegh 123", ip, "/home/tomcat/Zilla/agent agent_distributed")
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
	PageRanger(ctx,
		gin.H{},
		"report.html",
	)
}
func DeployAgentOnNodes(ctx *gin.Context) {
	//todo authentication and authorization

	var testName = ctx.PostForm("testName")
	var nodeIPList = ctx.PostForm("nodeIPList")
	//todo input validation
	println("get TestName -> " + testName)
	if len(nodeIPList) > 0 {
		/*
			e.g 192.168.1.1,192.168.2.2,192.168.3.3
		*/
		totalIPList := strings.Split(nodeIPList, ",")
		for _, ip := range totalIPList {
			if isAvailable(ip) == false {
				msg := ip + " is not available, please remove it and try again"
				println(msg)
				ctx.Redirect(302, "/new?msg="+msg)
				return
			}
		}
		for _, ip := range totalIPList {
			//todo validate ip, check if command and deploy agent already done well (all of them) , if error, reject request or report it
			go newTask(ip)
		}
	}
	ctx.Redirect(302, "/report")
	return
	// here we call agents
	/**
	Example
		curl http://agent_ip_address:8080/start?url=http://localhost:5656\&maxThread=50000\&maxRequest=1000000\&testDuration=5
	*/
}
