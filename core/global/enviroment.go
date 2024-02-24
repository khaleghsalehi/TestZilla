package global

import "gorm.io/gorm"

var DBConnection *gorm.DB
var SSHPingTimeOutSecond = 3
var MaxFileSizeAgentPostMethod = 1000000000
