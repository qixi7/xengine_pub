package cfgloader

import (
	"github.com/pkg/errors"
	"github.com/qixi7/xengine_core/xcontainer/job"
	"github.com/qixi7/xengine_core/xlog"
	"github.com/qixi7/xengine_core/xmodule"
)

// 热更结束项目回调处理(nil为不回调)
type IAfterReload interface {
	AfterDo(string, bool)
}

// 多线程Reload方法接口
func ReloadGameData(data IReloadData, reloadMgr xmodule.DModuleGetter) {
	mgr, ok := reloadMgr.Get().(*ReloadMgr)
	if ok {
		jobCtrl := mgr.getJobController()
		jobCtrl.PostJob(&reloadJob{data: data, reloadMgr: reloadMgr})
	}
}

type reloadJob struct {
	reloadMgr xmodule.DModuleGetter
	data      IReloadData
	success   bool
}

func (job *reloadJob) DoJob() job.Done {
	job.success = job.data.Load()
	return job
}

func (job *reloadJob) DoReturn() {
	reloadName := job.data.ReloadName()
	if job.success {
		job.data.ReloadCopy()
		xlog.InfoF("<reload> cfg -- %s -- success", reloadName)
	} else {
		xlog.InfoF("<reload> cfg -- %s -- fail", reloadName)
	}
	reloadMgr, ok := job.reloadMgr.Get().(*ReloadMgr)
	if ok {
		reloadMgr.afterReload(reloadName, job.success)
	}
}

type ReloadMgr struct {
	reloadMap  map[string]IReloadData
	needReload map[string]IAfterReload
	reloadArgs map[string][]string
	jobGetter  xmodule.DModuleGetter
}

func NewReloadMgr(jobGetter xmodule.DModuleGetter) *ReloadMgr {
	return &ReloadMgr{
		reloadMap:  make(map[string]IReloadData),
		needReload: make(map[string]IAfterReload),
		reloadArgs: make(map[string][]string),
		jobGetter:  jobGetter,
	}
}

// 主线程使用!!
func (mgr *ReloadMgr) getJobController() *job.Controller {
	return mgr.jobGetter.Get().(*job.Controller)
}

// Reload注册(不会保存传入接口,只会ReloadCreate获得方法调用权)
func (mgr *ReloadMgr) Register(reloadData IReloadData) bool {
	nullData := reloadData.ReloadCreate()
	reloadName := nullData.ReloadName()
	if _, ok := mgr.reloadMap[reloadName]; ok {
		xlog.Errorf("Register error, repeated name=%s", reloadName)
		return false
	}
	mgr.reloadMap[reloadName] = nullData
	return true
}

// 热更新调用这个接口!!!
func (mgr *ReloadMgr) Reload(module string, afterDo IAfterReload, extArgs ...string) error {
	if _, ok := mgr.needReload[module]; ok {
		return errors.New("frequently reload wait please")
	}
	if module == "all" {
		for modName, nullData := range mgr.reloadMap {
			mgr.needReload[modName] = afterDo
			nullData.ReloadCreate().Reload()
		}
		return nil
	}
	if nullData, ok := mgr.reloadMap[module]; ok {
		mgr.needReload[module] = afterDo
		mgr.reloadArgs[module] = append([]string{}, extArgs...)
		nullData.ReloadCreate().Reload()
		return nil
	}
	return errors.New("game data name not register")
}

func (mgr *ReloadMgr) afterReload(module string, success bool) {
	afterDo, ok := mgr.needReload[module]
	if ok && afterDo != nil {
		afterDo.AfterDo(module, success)
	}
	delete(mgr.needReload, module)
	delete(mgr.reloadArgs, module)
	xlog.InfoF("<reload> cfg -- %s -- job Done", module)
}

func (mgr *ReloadMgr) GetReloadExtArgs(module string) []string {
	extArgs, ok := mgr.reloadArgs[module]
	if !ok {
		return nil
	}
	return extArgs
}

func (mgr *ReloadMgr) Init(selfGetter xmodule.DModuleGetter) bool {
	return true
}

func (mgr *ReloadMgr) Run(delta int64) {
}

func (mgr *ReloadMgr) Destroy() {
}
