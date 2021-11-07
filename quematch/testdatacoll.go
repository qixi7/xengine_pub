package quematch

import (
	"math"
	"time"
	"xcore/xcontainer/job"
	"xcore/xlog"
	"xcore/xmodule"
)

/*
	测试代码, 用于测试匹配和增补
*/

// 用于匹配数据测试和统计

type IMatchDataCollOK interface {
	CollMatchOK(result *MatchResult)
	CollSupplyOK(result *MatchResult, info *SupplyInfo)
}

type CollOkImpl struct {
}

func (c *CollOkImpl) CollMatchOK(result *MatchResult) {
	xlog.Debugf("CollMatchOK.")
}

func (c *CollOkImpl) CollSupplyOK(result *MatchResult, info *SupplyInfo) {
	xlog.Debugf("CollSupplyOK., SupplyUUID=%d", info.SupplyUUID)
}

type collMatchSuccess struct {
	do   IMatchDataCollOK
	coll *MatchDataCollector
}

func (okdo *collMatchSuccess) MatchSuccess(result *MatchResult, clientKey ClientKey, info MapInfo) bool {
	successNum := 0
	result.ForeachMatchElem(func(oneElem *MatchElem, _ int) {
		successNum += oneElem.ElemData.GamerNum()
	})
	okdo.coll.matchMgr.matchClientInfo[clientKey].load.CurPlayerNum += int32(successNum)
	okdo.do.CollMatchOK(result)
	return true
}

func (okdo *collMatchSuccess) SupplySuccess(result *MatchResult, supplyInfo *SupplyInfo) bool {
	successNum := 0
	result.ForeachMatchElem(func(oneElem *MatchElem, _ int) {
		successNum += oneElem.ElemData.GamerNum()
	})
	//clientInfo, ok := okdo.coll.CollClientInfo[supplyInfo.SupplyUUID]
	//if ok {
	//	clientInfo.CurPlayerNum += int32(successNum)
	//}
	//okdo.coll.matchMgr.UpdateMatchClient(supplyInfo.SupplyUUID.ClientKey, *clientInfo)
	okdo.do.CollSupplyOK(result, supplyInfo)
	return true
}

type MatchDataCollector struct {
	matchMgr    *MatchQueueMgr
	dyModuleMgr xmodule.DModuleMgr
}

func NewMatchDataCollector(do IMatchDataCollOK) *MatchDataCollector {
	coll := &MatchDataCollector{}
	dyModuleMgr := xmodule.NewDModuleMgr(2)
	jobCtrl := job.NewController(1024, 4)
	jobGetter := dyModuleMgr.Register(0, jobCtrl)
	matchMgr := NewMatchQueueMgr(&collMatchSuccess{
		do:   do,
		coll: coll,
	})
	matchMgr.SetJobGetter(jobGetter)
	dyModuleMgr.Register(1, matchMgr)
	if !dyModuleMgr.InitAll() {
		return nil
	}
	coll.matchMgr = matchMgr
	coll.dyModuleMgr = dyModuleMgr
	return coll
}

func (coll *MatchDataCollector) InitClientMapInfo(client ClientKey, mapInfo ...MapInfo) {
	coll.matchMgr.matchClientInfo[client] = &matchClient{
		key: client,
		load: ClientInfo{
			MaxPlayerNum: math.MaxInt32,
		},
	}
	for i := 0; i < len(mapInfo); i++ {
		coll.matchMgr.UpdateMatchMap(mapInfo[i])
	}
}

func (coll *MatchDataCollector) PushMatchElem(queKey MatchQueueKey, elem *MatchElem) bool {
	return coll.matchMgr.EnterWaitQueue(queKey, elem)
}

func (coll *MatchDataCollector) AddMapSupply(queKey MatchQueueKey, info *SupplyInfo) bool {
	return coll.matchMgr.AddSubWorldSupply(queKey, info)
}

func (coll *MatchDataCollector) RegisterMatchAchieve(strategyType uint32, achieve IMatchAchieve) bool {
	return coll.matchMgr.RegisterMatchAchieve(strategyType, achieve)
}

func (coll *MatchDataCollector) RegisterSupplyAchieve(strategyType uint32, achieve ISupplyAchieve) bool {
	return coll.matchMgr.RegisterSupplyAchieve(strategyType, achieve)
}

func (coll *MatchDataCollector) TryMatch(matchNum int) {
	for i := 0; i < matchNum; i++ {
		for tickNum := int64(0); tickNum < coll.matchMgr.baseCfg.MatchTickGap; tickNum++ {
			coll.dyModuleMgr.RunAll(1)
		}
		time.Sleep(time.Millisecond * 50)
		coll.dyModuleMgr.RunAll(1)
	}
}
