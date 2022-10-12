package quematch

import (
	"github.com/qixi7/xengine_core/xcontainer/job"
	"github.com/qixi7/xengine_core/xlog"
	"github.com/qixi7/xengine_core/xmodule"
)

// 多线程匹配基于xcontainer/job

// 匹配实现接口
type IMatchAchieve interface {
	DoThreadMatch(base *MatchJobBase)
	CreateNewSelf() IMatchAchieve
}

// 增补实现接口
type ISupplyAchieve interface {
	DoThreadSupply(base *SupplyJobBase)
	CreateNewSelf() ISupplyAchieve
}

// newMatchAchieve...
func newMatchAchieve(matchType uint32, mqm *MatchQueueMgr) IMatchAchieve {
	achiBase := mqm.findMatchAchieve(matchType)
	if achiBase == nil {
		return nil
	}
	return achiBase.CreateNewSelf()
}

// newSupplyAchieve...
func newSupplyAchieve(matchType uint32, mqm *MatchQueueMgr) ISupplyAchieve {
	achiBase := mqm.findSupplyAchieve(matchType)
	if achiBase == nil {
		return nil
	}
	return achiBase.CreateNewSelf()
}

// 匹配基本job
type MatchJobBase struct {
	IMatchAchieve                        // 匹配算法接口
	matchMgrGetter xmodule.DModuleGetter // 匹配mgr getter
	cliKey         ClientKey             // client服务器key

	QueKey    MatchQueueKey // 匹配队列key
	QueMap    MapInfo       // 地图ID信息
	QueElems  []*MatchElem  // 匹配elem
	QueResult *MatchResult  // 匹配结果
}

func newMatchJob(ach IMatchAchieve) *MatchJobBase {
	return &MatchJobBase{
		IMatchAchieve: ach,
		QueElems:      make([]*MatchElem, 0),
		QueResult:     NewMatchResult(),
	}
}

// 主线程使用
func (mj *MatchJobBase) getMatchQueueMgr() *MatchQueueMgr {
	return mj.matchMgrGetter.Get().(*MatchQueueMgr)
}

func (mj *MatchJobBase) init(mgrGetter xmodule.DModuleGetter, queKey MatchQueueKey,
	matchQue *matchQueue, cliKey ClientKey, mapInfo MapInfo) bool {
	mj.matchMgrGetter = mgrGetter
	mj.cliKey = cliKey
	mj.QueKey = queKey
	mj.QueMap = mapInfo
	elemQueue := matchQue.copyCanMatchElems()
	mj.QueElems = append(mj.QueElems, elemQueue...)
	return len(mj.QueElems) > 0
}

func (mj *MatchJobBase) DoJob() job.Done {
	mj.DoThreadMatch(mj)
	return mj
}

func (mj *MatchJobBase) DoReturn() {
	queMgr := mj.getMatchQueueMgr()
	matchQue := queMgr.findMatchQueue(mj.QueKey)
	if matchQue == nil {
		xlog.Errorf("<queue_match> match return no queueKey=%v", mj.QueKey)
		return
	}
	matchQue.inMatch = false
	groupLen := len(mj.QueResult.Groups)
	// 匹配结果为空, 说明匹配失败
	if groupLen <= 0 {
		return
	}

	// 保底检查一下这些匹配元素是否存在. 避免幽灵匹配
	allElemExist := true
	mj.QueResult.ForeachMatchElem(func(oneElem *MatchElem, _ int) {
		if findElem, _ := queMgr.FindMatchElem(oneElem.ElemKey); findElem == nil {
			allElemExist = false
			return
		}
	})
	if !allElemExist {
		return
	}
	// 匹配成功回调
	allok := queMgr.successDo.MatchSuccess(mj.QueResult, mj.cliKey, mj.QueMap)
	if !allok {
		return
	}
	// log
	xlog.InfoF("<queue_match> match success queKey=%v, result:", mj.QueKey)
	mj.QueResult.ForeachMatchElem(func(oneElem *MatchElem, elemIdx int) {
		xlog.InfoF("\t<queue_match> elemIdx=%d, elem=%v", elemIdx, *oneElem)
	})
	// 离开匹配
	mj.QueResult.ForeachMatchElem(func(oneElem *MatchElem, elemIdx int) {
		queMgr.LeaveQueue(oneElem.ElemKey, true)
	})
}

// 增补基本job
type SupplyJobBase struct {
	ISupplyAchieve
	matchMgrGetter xmodule.DModuleGetter
	SupInfo        *SupplyInfo

	QueKey    MatchQueueKey
	QueMap    MapInfo
	QueElems  []*MatchElem
	QueResult *MatchResult
}

func newSupplyJob(ach ISupplyAchieve) *SupplyJobBase {
	return &SupplyJobBase{
		ISupplyAchieve: ach,
		QueElems:       make([]*MatchElem, 0),
		QueResult:      NewMatchResult(),
	}
}

// 主线程使用
func (sj *SupplyJobBase) getMatchQueueMgr() *MatchQueueMgr {
	return sj.matchMgrGetter.Get().(*MatchQueueMgr)
}

func (sj *SupplyJobBase) init(mgrGetter xmodule.DModuleGetter, queKey MatchQueueKey,
	matchQue *matchQueue, supInfo *SupplyInfo, mapInfo MapInfo) bool {
	sj.matchMgrGetter = mgrGetter
	sj.SupInfo = supInfo
	sj.QueKey = queKey
	sj.QueMap = mapInfo
	sj.QueElems = append(sj.QueElems, matchQue.copyCanMatchElems()...)
	return len(sj.QueElems) > 0
}

func (sj *SupplyJobBase) DoJob() job.Done {
	sj.DoThreadSupply(sj)
	return sj
}

func (sj *SupplyJobBase) DoReturn() {
	queMgr := sj.getMatchQueueMgr()
	matchQue := queMgr.findMatchQueue(sj.QueKey)
	if matchQue == nil {
		xlog.Errorf("<queue_match> supply return no queueKey=%v", sj.QueKey)
		return
	}
	matchQue.inMatch = false
	groupLen := len(sj.QueResult.Groups)
	// 增补结果为空, 说明增补失败
	if groupLen <= 0 {
		return
	}

	// 保底检查一下这些匹配元素是否存在. 避免幽灵匹配
	allElemExist := true
	sj.QueResult.ForeachMatchElem(func(oneElem *MatchElem, elemIdx int) {
		if findElem, _ := queMgr.FindMatchElem(oneElem.ElemKey); findElem == nil {
			allElemExist = false
			return
		}
	})
	if !allElemExist {
		return
	}
	allok := queMgr.successDo.SupplySuccess(sj.QueResult, sj.SupInfo)
	if !allok {
		return
	}
	// log
	xlog.InfoF("<queue_match> supply success queKey=%v, result:", sj.QueKey)
	sj.QueResult.ForeachMatchElem(func(oneElem *MatchElem, elemIdx int) {
		xlog.InfoF("\t<queue_match> elemIdx=%d, elem=%v", elemIdx, *oneElem)
	})
	// 离开匹配队列
	sj.QueResult.ForeachMatchElem(func(oneElem *MatchElem, _ int) {
		queMgr.LeaveQueue(oneElem.ElemKey, true)
	})
}
