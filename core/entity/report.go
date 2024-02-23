package entity

import (
	"gorm.io/gorm"
)

type TestReport struct {
	gorm.Model
	ReportID            string
	RelatedTestPolicyID string
	TimeStamp           string
	AgentIP             string
	TestResult          string
}
