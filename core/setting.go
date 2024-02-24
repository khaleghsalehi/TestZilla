package core

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"testzilla/core/entity"
)

var TestZillaServerPortNumber = "9090"
var TestZillaAgentPortNumber = "8080"

var DBUserName = "testzilla"
var DBPassword = "123456"
var DBName = "tz"

var ProtocolsList []string
var ProtocolOption []string

func InitTables(db *gorm.DB) bool {
	err := db.AutoMigrate(&entity.TestCase{})
	if err != nil {
		println("error while initialize test policy tables")
		return false
	}
	err = db.AutoMigrate(&entity.TestingReport{})
	if err != nil {
		println("error while initialize test test reports tables")
		return false
	}

	return true
}
func InitServer() {
	//update protocols list
	ProtocolsList = append(ProtocolsList, "http")
	ProtocolsList = append(ProtocolsList, "https")

	// update protocol options
	ProtocolOption = append(ProtocolOption, "GET")
	ProtocolOption = append(ProtocolOption, "POST")
}
func InitDB() *gorm.DB {
	dsn := "host=localhost user=" + DBUserName + " password=" + DBPassword + " dbname=" + DBName + " port=5432 sslmode=disable "

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		println("error while connecting database " + DBName)
		return nil
	} else {
		println("connection to database " + DBName + " done")
		if InitTables(db) == false {
			return nil
		}
		return db
	}
}
