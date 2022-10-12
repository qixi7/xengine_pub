/*
	cfgloader主要用于统一档的加载与Reload更新流程
*/
package cfgloader

import (
	"github.com/json-iterator/go"
	"github.com/qixi7/xengine_core/xlog"
	"github.com/qixi7/xengine_core/xmodule"
	"io/ioutil"
)

// 档数据接口
type IFileData interface {
	Path() string
}

// 热更接口
type IReloadData interface {
	xmodule.SModule
	ReloadCreate() IReloadData
	ReloadName() string
	ReloadCopy()
}

// 读json档
func LoadJsonFile(data IFileData) bool {
	f, err := ioutil.ReadFile(data.Path())
	if err != nil {
		xlog.Errorf("LoadJsonFile ReadFile err=%v", err)
		return false
	}
	err = jsoniter.Unmarshal(f, data)
	if err != nil {
		xlog.Errorf("LoadJsonFile Unmarshal err=%v", err)
		return false
	}
	return true
}
