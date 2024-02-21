package entity

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TestReport struct {
	gorm.Model
	ReportID            uuid.UUID
	RelatedTestPolicyID string
	TimeStamp           string
	AgentIP             string
	TestResult          string
}
