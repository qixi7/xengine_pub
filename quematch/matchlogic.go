package quematch

import (
	"xcore/xlog"
)

type IMatchSuccess interface {
	// 返回是否从队列中删除MatchResult中的elem
	MatchSuccess(result *MatchResult, clientKey ClientKey, mapInfo MapInfo) bool
	SupplySuccess(result *MatchResult, supplyInfo *SupplyInfo) bool
}

type MatchResult struct {
	Groups []*MatchElem
}

func (mr *MatchResult) ForeachMatchElem(runFunc func(elem *MatchElem, elemIdx int)) {
	for elemIdx := 0; elemIdx < len(mr.Groups); elemIdx++ {
		runFunc(mr.Groups[elemIdx], elemIdx)
	}
}

func (mr *MatchResult) AddGroup(elemList ...*MatchElem) {
	mr.Groups = append(mr.Groups, elemList...)
}

func NewMatchResult() *MatchResult {
	return &MatchResult{
		Groups: make([]*MatchElem, 0),
	}
}

// 匹配一次
func tryMatchOnce(mqm *MatchQueueMgr) {
	// 获取所有能匹配的client服务器
	hungryList := make([]*matchClient, 0, len(mqm.matchClientInfo))
	for _, cliInfo := range mqm.matchClientInfo {
		if cliInfo.canMatch() {
			hungryList = append(hungryList, cliInfo)
		}
	}
	if len(hungryList) <= 0 {
		return
	}

	// 逐个取出满足条件的client服务器和mapInfo进行匹配
	matchedQue := make(map[MatchQueueKey]interface{}) // 已匹配过的队列, 用于检验busy
	for _, oneHungry := range hungryList {
		// 找出一个可以匹配的map
		var canMatchMap *MapInfo = nil
		for _, aMap := range mqm.mapsInfo {
			// 剔除负载不足的mapInfo
			if oneHungry.load.hungry() > aMap.MatchTotalNeed {
				canMatchMap = &aMap
				break
			}
		}
		if canMatchMap == nil {
			continue
		}
		// 获取所有该地图的待匹配队列
		queKeys := mqm.getMatchQueueKeyByMapID(canMatchMap.MapID)
		for i := 0; i < len(queKeys); i++ {
			// 不满足负载了
			if oneHungry.load.hungry() <= canMatchMap.MatchTotalNeed {
				break
			}
			if tryMatchOnceQueue(mqm, queKeys[i], oneHungry.key, canMatchMap, matchedQue) {
				mqm.matchClientInfo[oneHungry.key].load.CurPlayerNum += canMatchMap.MatchTotalNeed
			}
		}
	}
}

// 匹配一次实现
func tryMatchOnceQueue(mqm *MatchQueueMgr, queKey MatchQueueKey, cliKey ClientKey,
	oneMap *MapInfo, matchedQue map[MatchQueueKey]interface{}) bool {
	// 通过Key获取匹配队列
	matchQue := mqm.findMatchQueue(queKey)
	if matchQue == nil {
		return false
	}
	if matchQue.inMatch {
		if _, ok := matchedQue[queKey]; ok {
			// 一个clientKey对应一次match,到这里原因有两个:
			// 1. 已经有一个clientKey分配到了这一次match	<正常情况>
			// 2. 原来的match算法还没有返回结果			<bug>
			//xlog.Warnf("<queue_match> busy match queKey=%v", queKey)
		}
		return false
	}
	// 有增补先处理增补
	if matchQue.hasSupply() {
		supplyAchieve := newSupplyAchieve(queKey.MatchStrategy, mqm)
		if supplyAchieve == nil {
			xlog.Errorf("<queue_match> no supply strategy=%d achieve.", queKey.MatchStrategy)
			return false
		}
		supplyInfo := matchQue.popSupply()
		supplyJob := newSupplyJob(supplyAchieve)
		if !supplyJob.init(mqm.selfGetter, queKey, matchQue, supplyInfo, *oneMap) {
			return false
		}
		matchedQue[queKey] = nil // 占位
		matchQue.inMatch = true
		mqm.getJobController().PostJob(supplyJob)
		return true
	}
	matchAchieve := newMatchAchieve(queKey.MatchStrategy, mqm)
	if matchAchieve == nil {
		xlog.Errorf("<queue_match> no match strategy=%d achieve.", queKey.MatchStrategy)
		return false
	}
	matchJob := newMatchJob(matchAchieve)
	if !matchJob.init(mqm.selfGetter, queKey, matchQue, cliKey, *oneMap) {
		return false
	}
	matchedQue[queKey] = nil // 占位
	matchQue.inMatch = true
	mqm.getJobController().PostJob(matchJob)
	return true
}
