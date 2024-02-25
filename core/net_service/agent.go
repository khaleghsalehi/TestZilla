package net_service

import (
	"fmt"
	"github.com/scottkiss/gosshtool"
	"log"
	"testzilla/core/global"
)

func DeployAgent(username string, password string, ip string, src string, dst string) (error, bool) {
	config := &gosshtool.SSHClientConfig{
		User:     username,
		Password: password,
		Host:     ip,
	}
	sshClient := gosshtool.NewSSHClient(config)
	sshClient.MaxDataThroughput = 6553600
	stdout, stderr, err := gosshtool.UploadFile(ip, src, dst)
	if err != nil {
		return err, false
	}
	if stderr != "" {
		return err, false
	}
	log.Println("agent deployed succeeded " + stdout)
	return nil, true
}

func PingSSHServer(username string, password string, ip string, command string) bool {
	println("ssh ping agent -> ", ip)
	sshConfig := &gosshtool.SSHClientConfig{
		User:     username,
		Password: password,
		Host:     ip,
	}
	sshClient := gosshtool.NewSSHClient(sshConfig)
	sshClient.DialTimeoutSecond = global.SSHPingTimeOutSecond
	//fmt.Println(sshClient.Host)
	_, _, _, err := sshClient.Cmd(command, nil, nil, 0)
	if err != nil {
		return false
	}
	return true
}
func RunCommandOnAgent(username string, password string, ip string, command string) {
	sshConfig := &gosshtool.SSHClientConfig{
		User:     username,
		Password: password,
		Host:     ip,
	}
	sshClient := gosshtool.NewSSHClient(sshConfig)
	//fmt.Println(sshClient.Host)
	//stdout, stderr, _, err := sshClient.Cmd(command, nil, nil, 0)
	_, _, _, err := sshClient.Cmd(command, nil, nil, 0)
	if err != nil {
		fmt.Println(err)
	}
	//fmt.Println(stdout)
	//fmt.Println(stderr)
}
