package input

import (
	i18nLoc "ariaflow/i18n"
	"ariaflow/internal/config"
	"ariaflow/tool/input/aria2"
	logger "ariaflow/tool/zap"
)

type Tool struct {
	aria2.Aria2
}

var ToolApp = new(Tool)

func PauseTask(sign string) {
	switch config.InputToolMethod() {
	case 0:
		ToolApp.Aria2.Pause(sign)
	default:
		logger.Error(i18nLoc.LocText("No input tool is selected"))
	}
}
func UnpauseTask(sign string) {
	switch config.InputToolMethod() {
	case 0:
		ToolApp.Aria2.Unpause(sign)
	default:
		logger.Error(i18nLoc.LocText("No input tool is selected"))
	}
}

func ForceRemoveTask(sign string) {
	switch config.InputToolMethod() {
	case 0:
		ToolApp.Aria2.ForceRemove(sign)
	default:
		logger.Error(i18nLoc.LocText("No input tool is selected"))
	}
}
func PauseAllTask() {
	switch config.InputToolMethod() {
	case 0:
		ToolApp.Aria2.PauseAll()
	default:
		logger.Error(i18nLoc.LocText("No input tool is selected"))
	}
}
func UnpauseAllTask() {
	switch config.InputToolMethod() {
	case 0:
		ToolApp.Aria2.UnpauseAll()
	default:
		logger.Error(i18nLoc.LocText("No input tool is selected"))
	}
}
