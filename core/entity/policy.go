package entity

import (
	"gorm.io/gorm"
	"testzilla/core/global"
)

type TestCase struct {
	gorm.Model
	ID                                  string
	TimeStamp                           string
	TestName                            string
	TestProtocolName                    string
	TestProtocolOptions                 string
	TestHttpProtocolPostOptionsFileSize string

	TestMaxThreadPerNode int64
	TestMaxRequest       int64
	TestDuration         int
	NodeIPList           string
	TestPassed           bool
	TestFailed           bool
	TestRunning          bool
	TestStarted          bool
	TestFinished         bool

	SSHUserName string
	SSHPassword string
	SSHPort     int
	TargetIP    string
	TargetPort  string
}

func UpdateTestStatus(test TestCase, TestStarted bool, TestRunning bool, TestFailed bool, TestPassed bool, TestFinished bool) {
	global.DBConnection.Model(&test).Where("id =?", test.ID).Update("test_started", TestStarted)
	global.DBConnection.Model(&test).Where("id =?", test.ID).Update("test_running", TestRunning)
	global.DBConnection.Model(&test).Where("id =?", test.ID).Update("test_passed", TestPassed)
	global.DBConnection.Model(&test).Where("id =?", test.ID).Update("test_finished", TestFinished)
	global.DBConnection.Model(&test).Where("id =?", test.ID).Update("test_failed", TestFailed)
	return
}
