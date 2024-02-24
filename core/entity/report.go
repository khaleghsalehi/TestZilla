package entity

import (
	"gorm.io/gorm"
)

type TestingReport struct {
	gorm.Model
	ReportID            string
	RelatedTestPolicyID string
	TimeStamp           string
	AgentIP             string
	TestResult          string
}
