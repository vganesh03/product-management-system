package logging

import (
	"github.com/sirupsen/logrus"
)

var Logger = logrus.New()

func Init() {
	Logger.SetFormatter(&logrus.JSONFormatter{})
	Logger.SetLevel(logrus.InfoLevel)
}
