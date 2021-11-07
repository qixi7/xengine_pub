package dispatchcollector

import (
	"time"
	"xcore/xcontainer/timer"
	"xcore/xlog"
)

/*
	dispatchcollect.go: 分发-收集器, 用于比如匹配确认机制, 匹配成功时分发所有人, 当所有人确认后
*/

// 上层依赖接口
type dispatchMgrInterface interface {
	GetTimerMgr() *timer.Controller // timer mgr
}

// 分发-收集器 管理类
type DispatchCollectMgr struct {
	interf     dispatchMgrInterface   // impl
	collectMap map[uint32]*oneCollect // key->collID, value->一次分发-收集信息
	collIDBase uint32                 // 自增ID基数
}

// --------------------------- 内置函数 ---------------------------

// 生成自增ID
func (mgr *DispatchCollectMgr) genID() uint32 {
	// 注意, 理论上这里的回滚可能在并发量特别大的时候存在风险, 但是应该不会出现这种情况
	mgr.collIDBase++
	if mgr.collIDBase >= 1000000000 {
		mgr.collIDBase = 1 // 回滚
	}
	return mgr.collIDBase
}

// 删除一次收集
func (mgr *DispatchCollectMgr) delOneCollect(collID uint32, op string) {
	//xlog.InfoF("delOneCollect, collID=%d, op=%s", collID, op)
	delete(mgr.collectMap, collID)
}

// --------------------------- 外置函数 ---------------------------

// new
func NewDispatchCollectMgr(interf dispatchMgrInterface) *DispatchCollectMgr {
	return &DispatchCollectMgr{
		interf:     interf,
		collectMap: make(map[uint32]*oneCollect),
		collIDBase: 0,
	}
}

// 新建一个收集器
// 返回: 新建的收集器
func (mgr *DispatchCollectMgr) CreateOneCollect(timeout time.Duration, interf dispatchInterface) *oneCollect {
	collID := mgr.genID()
	timeItem := mgr.interf.GetTimerMgr().Add(timeout, &collectTimer{mgr: mgr, collID: collID})
	oneColl := newOneCollect(collID, timeItem, interf)
	mgr.collectMap[collID] = oneColl
	//xlog.InfoF("CreateOneCollect, ID=%d", collID)
	return oneColl
}

// get
func (mgr *DispatchCollectMgr) GetCollect(collID uint32) *oneCollect {
	coll, ok := mgr.collectMap[collID]
	if !ok {
		return nil
	}
	return coll
}

// 收集一个结果
func (mgr *DispatchCollectMgr) CollectOne(collID uint32, key uint32, agree bool) {
	coll := mgr.GetCollect(collID)
	if coll == nil {
		xlog.Errorf("CollectOne err, collID=%d, not exist. key=%d, agree=%t",
			collID, key, agree)
		return
	}
	coll.collectOne(key, agree)
	// 检查收集是否结束
	if coll.checkCollectOver() {
		mgr.interf.GetTimerMgr().Remove(coll.timeout)
		mgr.delOneCollect(collID, "collect one")
	}
}

// 收集器个数
func (mgr *DispatchCollectMgr) Count() int {
	return len(mgr.collectMap)
}

// --------------------------- 其他工具类 ---------------------------

// 收集超时timer
type collectTimer struct {
	mgr    *DispatchCollectMgr // mgr
	collID uint32              // collID
}

func (t *collectTimer) Invoke(ctl *timer.Controller, item *timer.Item) {
	// 只要超时, 一定是因为收集失败
	coll := t.mgr.GetCollect(t.collID)
	if coll == nil {
		return
	}
	coll.timeout = nil
	var timeoutRoleID uint32
	coll.ForeachColl(func(roleID uint32, state int, exData interface{}) bool {
		if state != ECollectAccept {
			timeoutRoleID = roleID
			return false
		}
		return true
	})
	coll.interf.OnCollectFailed(t.collID, timeoutRoleID)
	t.mgr.delOneCollect(t.collID, "timeout")
}
