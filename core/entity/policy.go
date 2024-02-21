package entity

import (
	"gorm.io/gorm"
)

type TestCase struct {
	gorm.Model
	ID                   string
	TimeStamp            string
	TestName             string
	TestMaxThreadPerNode int64
	TestMaxRequest       int64
	TestDuration         int
	NodeIPList           string
	TestPassed           bool
	TestRunning          bool
	TestStarted          bool
	TestFinished         bool
	SSHUserName          string
	SSHPassword          string
	SSHPort              int
	TargetIP             string
	TargetPort           string
}
