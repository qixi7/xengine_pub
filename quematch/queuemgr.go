/*
	quematch包初衷是设计一套相对通用的匹配系统
*/

package quematch

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/qixi7/xengine_core/xcontainer/job"
	"github.com/qixi7/xengine_core/xlog"
	"github.com/qixi7/xengine_core/xmetric"
	"github.com/qixi7/xengine_core/xmodule"
	"strings"
)

// 匹配基本配置信息
type MatchBaseCfg struct {
	MatchTickGap     int64 // 匹配帧数时间间隔, 一个队列一次匹配完之后才能进行下次匹配
	ShowMatchTickGap int64 // 打印匹配信息log帧数间隔
}

// 匹配策略类型
const (
	MatchStrategyNone   = iota // 无效值
	MatchStrategyNormal        // 常规匹配. 没有分数, 人够就行
)

// 匹配Key
type MatchQueueKey struct {
	MapID         uint32 // 地图ID
	MatchStrategy uint32 // 匹配策略
}

type SupplyInfo struct {
	InfoData   interface{}
	SupplyUUID uint64
}

type matchQueue struct {
	inMatch     bool
	matchElems  []*MatchElem        // 匹配elem
	supplyInfos []*SupplyInfo       // 增补请求队列
	supplyMap   map[uint64]struct{} // 增补map
}

func newMatchQueue() *matchQueue {
	return &matchQueue{
		matchElems:  make([]*MatchElem, 0),
		supplyInfos: make([]*SupplyInfo, 0),
		supplyMap:   make(map[uint64]struct{}),
	}
}

// 获取正常匹配元素数量
func (mq *matchQueue) getElemLen() int {
	return len(mq.matchElems)
}

// 获取增补匹配元素数量
func (mq *matchQueue) getSupplyLen() int {
	return len(mq.supplyInfos)
}

func (mq *matchQueue) findMatchIdx(elemKey MatchElemKey) int {
	for i := 0; i < len(mq.matchElems); i++ {
		if mq.matchElems[i].ElemKey == elemKey {
			return i
		}
	}
	return -1
}

func (mq *matchQueue) addMatch(elem *MatchElem) {
	mq.matchElems = append(mq.matchElems, elem)
}

func (mq *matchQueue) hasSupply() bool {
	if len(mq.supplyInfos) <= 0 {
		return false
	}
	return true
}

func (mq *matchQueue) addSupply(info *SupplyInfo) bool {
	if _, ok := mq.supplyMap[info.SupplyUUID]; ok {
		return false
	}
	for i := 0; i < len(mq.supplyInfos); i++ {
		if mq.supplyInfos[i].SupplyUUID == info.SupplyUUID {
			mq.supplyInfos = append(mq.supplyInfos[:i], mq.supplyInfos[i+1:]...)
			break
		}
	}
	mq.supplyInfos = append(mq.supplyInfos, info)
	return true
}

func (mq *matchQueue) popSupply() *SupplyInfo {
	if len(mq.supplyInfos) <= 0 {
		return nil
	}
	firstSupply := mq.supplyInfos[0]
	mq.supplyInfos = append(mq.supplyInfos[:0], mq.supplyInfos[1:]...)
	delete(mq.supplyMap, firstSupply.SupplyUUID)

	xlog.Debugf("popSupply, UUID=%d", firstSupply.SupplyUUID)
	return firstSupply
}

func (mq *matchQueue) delSupply(SupplyUUID uint64) bool {
	delete(mq.supplyMap, SupplyUUID)
	if len(mq.supplyInfos) <= 0 {
		return false
	}
	delIdx := -1
	for idx, info := range mq.supplyInfos {
		if info.SupplyUUID == SupplyUUID {
			delIdx = idx
			xlog.Debugf("delSupply, UUID=%v", info.SupplyUUID)
			break
		}
	}
	if delIdx > 0 {
		mq.supplyInfos = append(mq.supplyInfos[:delIdx], mq.supplyInfos[delIdx+1:]...)
		return true
	}
	return false
}

func (mq *matchQueue) copyCanMatchElems() []*MatchElem {
	newGroup := make([]*MatchElem, 0, len(mq.matchElems))
	for i := 0; i < len(mq.matchElems); i++ {
		newGroup = append(newGroup, mq.matchElems[i].clone())
	}
	return newGroup
}

// 匹配队列管理类
type MatchQueueMgr struct {
	baseCfg          MatchBaseCfg                   // 基本匹配配置
	tickTotal        int64                          // tick总帧数
	waitingQueue     map[MatchQueueKey]*matchQueue  // 不同matchKey对应的队列
	elem2MatchQueue  map[MatchElemKey]MatchQueueKey // 通过elemKey查找匹配队列Key
	matchClientInfo  map[ClientKey]*matchClient     // 匹配client key -> 匹配client info
	mapsInfo         map[uint32]MapInfo             // mapID->map Info
	jobCtrlGetter    xmodule.DModuleGetter          // job getter
	selfGetter       xmodule.DModuleGetter          // 获取自己的getter
	successDo        IMatchSuccess                  // 匹配成功回调(业务实现)
	matchExtAchieve  map[uint32]IMatchAchieve       // 匹配算法(业务实现)
	supplyExtAchieve map[uint32]ISupplyAchieve      // 增补算法(业务实现)
}

// new
func NewMatchQueueMgr(do IMatchSuccess) *MatchQueueMgr {
	return &MatchQueueMgr{
		baseCfg: MatchBaseCfg{
			MatchTickGap:     10,
			ShowMatchTickGap: 100,
		},
		successDo:        do,
		waitingQueue:     make(map[MatchQueueKey]*matchQueue),
		elem2MatchQueue:  make(map[MatchElemKey]MatchQueueKey),
		matchClientInfo:  make(map[ClientKey]*matchClient),
		matchExtAchieve:  make(map[uint32]IMatchAchieve),
		supplyExtAchieve: make(map[uint32]ISupplyAchieve),
		mapsInfo:         make(map[uint32]MapInfo),
	}
}

// 必须设置! 否则宕机失败
func (mqm *MatchQueueMgr) SetJobGetter(jobGetter xmodule.DModuleGetter) {
	mqm.jobCtrlGetter = jobGetter
}

func (mqm *MatchQueueMgr) getJobController() *job.Controller {
	return mqm.jobCtrlGetter.Get().(*job.Controller)
}

func (mqm *MatchQueueMgr) getMatchQueueKeyByMapID(mapID uint32) []MatchQueueKey {
	keys := make([]MatchQueueKey, 0, 10)
	for queKey := range mqm.waitingQueue {
		if queKey.MapID == mapID {
			// 当只有匹配元素数量>0时, 再返回, 不然也没有意义
			if mqm.waitingQueue[queKey].getElemLen() > 0 ||
				mqm.waitingQueue[queKey].getSupplyLen() > 0 {
				keys = append(keys, queKey)
			}
		}
	}
	return keys
}

func (mqm *MatchQueueMgr) findMatchQueue(queKey MatchQueueKey) *matchQueue {
	findQue, ok := mqm.waitingQueue[queKey]
	if !ok {
		return nil
	}
	return findQue
}

func (mqm *MatchQueueMgr) findQueKeyByElemKey(elemKey MatchElemKey) *MatchQueueKey {
	findSearch, ok := mqm.elem2MatchQueue[elemKey]
	if !ok {
		return nil
	}
	return &findSearch
}

// elem进对应的queue key匹配队列
func (mqm *MatchQueueMgr) push(queKey MatchQueueKey, elem *MatchElem) {
	matchQue := mqm.findMatchQueue(queKey)
	if matchQue == nil {
		matchQue = newMatchQueue()
		mqm.waitingQueue[queKey] = matchQue
	}
	elemSearch := mqm.findQueKeyByElemKey(elem.ElemKey)
	if elemSearch != nil {
		panic("MatchElem in mut MatchQueue")
	}
	matchQue.addMatch(elem)
	mqm.elem2MatchQueue[elem.ElemKey] = queKey
	elem.OnEnterQueue(queKey, elem)
	xlog.InfoF("<queue_match> enter queue: key=%v, elem=%v", queKey, *elem)
}

// 匹配中是否存在该匹配元素
func (mqm *MatchQueueMgr) FindMatchElem(elemKey MatchElemKey) (*MatchElem, MatchQueueKey) {
	// 看是否在elem2MatchQueue中
	queKey := mqm.findQueKeyByElemKey(elemKey)
	if queKey == nil {
		return nil, MatchQueueKey{}
	}
	// 看是否存在该queKey的队列
	matchQue := mqm.findMatchQueue(*queKey)
	if matchQue == nil {
		return nil, *queKey
	}
	// 该matchQueue是否含有该elem
	elemIdx := matchQue.findMatchIdx(elemKey)
	if elemIdx < 0 {
		panic("elem search no elem")
	}
	return matchQue.matchElems[elemIdx], *queKey
}

// EnterWaitQueue...
func (mqm *MatchQueueMgr) EnterWaitQueue(queKey MatchQueueKey, elem *MatchElem) bool {
	if elem == nil {
		return false
	}
	// 保险起见, 让这些elem key先离开匹配再进入匹配
	allKeys := elem.allTypeKey()
	for i := 0; i < len(allKeys); i++ {
		mqm.LeaveQueue(*allKeys[i], false)
	}
	mqm.push(queKey, elem)
	return true
}

// LeaveQueue...
func (mqm *MatchQueueMgr) LeaveQueue(elemKey MatchElemKey, success bool) bool {
	queKey := mqm.findQueKeyByElemKey(elemKey)
	if queKey == nil {
		return false
	}
	matchQue := mqm.findMatchQueue(*queKey)
	if matchQue != nil {
		elemIdx := matchQue.findMatchIdx(elemKey)
		if elemIdx >= 0 {
			elem := matchQue.matchElems[elemIdx]
			elem.OnLeaveQueue(*queKey, elem, success)
			xlog.InfoF("<queue_match> leave queue: queKey=%v, elem=%v",
				queKey, *elem)
		}
		// 从该queKey的匹配队列中删除匹配元素
		matchQue.matchElems = append(matchQue.matchElems[:elemIdx], matchQue.matchElems[elemIdx+1:]...)
	}
	// 删除查找索引
	delete(mqm.elem2MatchQueue, elemKey)
	return true
}

// 添加增补
func (mqm *MatchQueueMgr) AddSubWorldSupply(queKey MatchQueueKey, info *SupplyInfo) bool {
	if info == nil {
		return false
	}
	matchQue := mqm.findMatchQueue(queKey)
	if matchQue == nil {
		matchQue = newMatchQueue()
		mqm.waitingQueue[queKey] = matchQue
	}
	ok := matchQue.addSupply(info)
	if ok {
		xlog.InfoF("<queue_match> require supply: queKey=%v, info=%v", queKey, *info)
	}
	return ok
}

// 删除增补
func (mqm *MatchQueueMgr) DelSubWorldSupply(queKey MatchQueueKey, SupplyUUID uint64) bool {
	matchQue := mqm.findMatchQueue(queKey)
	if matchQue == nil {
		return false
	}
	ret := matchQue.delSupply(SupplyUUID)
	if ret {
		xlog.InfoF("<queue_match> delete supply: queKey=%v, info=%v", queKey, SupplyUUID)
	}
	return ret
}

// 更新map信息
func (mqm *MatchQueueMgr) UpdateMatchMap(info MapInfo) {
	mqm.mapsInfo[info.MapID] = info
}

// 遍历所有ClientKey
func (mqm *MatchQueueMgr) ForeachClientKey(runFunc func(key ClientKey, info ClientInfo, notUse bool) bool) {
	for cliKey, cliInfo := range mqm.matchClientInfo {
		runFunc(cliKey, cliInfo.load, cliInfo.notUse)
	}
}

func (mqm *MatchQueueMgr) GetMatchClientInfo(clientKey ClientKey) *ClientInfo {
	cliInfo, ok := mqm.matchClientInfo[clientKey]
	if !ok {
		mqm.matchClientInfo[clientKey] = &matchClient{
			key:  clientKey,
			load: ClientInfo{},
		}
		cliInfo = mqm.matchClientInfo[clientKey]
	}
	return &cliInfo.load
}

// 设置client服务器是否可用
func (mqm *MatchQueueMgr) SetClientUse(clientKey ClientKey, noUse bool) {
	cliInfo, ok := mqm.matchClientInfo[clientKey]
	if ok {
		cliInfo.notUse = noUse
	}
}

// 设置基础配置
func (mqm *MatchQueueMgr) SetMatchBaseCfg(cfg MatchBaseCfg) {
	if cfg.MatchTickGap > 0 {
		mqm.baseCfg.MatchTickGap = cfg.MatchTickGap
	}
	if cfg.ShowMatchTickGap > 0 {
		mqm.baseCfg.ShowMatchTickGap = cfg.ShowMatchTickGap
	}
}

func (mqm *MatchQueueMgr) findMatchAchieve(strategyType uint32) IMatchAchieve {
	achi, ok := mqm.matchExtAchieve[strategyType]
	if !ok {
		return nil
	}
	return achi
}

func (mqm *MatchQueueMgr) findSupplyAchieve(strategyType uint32) ISupplyAchieve {
	achi, ok := mqm.supplyExtAchieve[strategyType]
	if !ok {
		return nil
	}
	return achi
}

// 注册匹配算法
func (mqm *MatchQueueMgr) RegisterMatchAchieve(strategyType uint32, achieve IMatchAchieve) bool {
	if strategyType <= uint32(MatchStrategyNone) {
		xlog.Errorf("RegisterMatchAchieve Common Type=%d, Error", strategyType)
		return false
	}
	mqm.matchExtAchieve[strategyType] = achieve
	return true
}

// 注册增补算法
func (mqm *MatchQueueMgr) RegisterSupplyAchieve(strategyType uint32, achieve ISupplyAchieve) bool {
	if strategyType <= uint32(MatchStrategyNone) {
		xlog.Errorf("RegisterSupplyAchieve Common Type=%d, Error", strategyType)
		return false
	}
	mqm.supplyExtAchieve[strategyType] = achieve
	return true
}

// --------------------------- impl interface ---------------------------

func (mqm *MatchQueueMgr) Init(selfGetter xmodule.DModuleGetter) bool {
	mqm.selfGetter = selfGetter
	return true
}

func (mqm *MatchQueueMgr) Run(delta int64) {
	if mqm.tickTotal == 0 && mqm.getJobController() == nil {
		panic("MatchQueueMgr init fail, no job controller")
	}
	mqm.tickTotal++
	// 这里只为打印
	if mqm.tickTotal%mqm.baseCfg.ShowMatchTickGap == 0 {
		// for print match information
		buff := strings.Builder{}
		for queKey, oneQue := range mqm.waitingQueue {
			if len(oneQue.matchElems) > 0 || len(oneQue.supplyInfos) > 0 {
				baseStr := "\n\t\t\t<map=%d, strategy=%d> queueNum=%d, supplyNum=%d"
				buff.WriteString(fmt.Sprintf(baseStr,
					queKey.MapID, queKey.MatchStrategy, len(oneQue.matchElems), len(oneQue.supplyInfos)))
			}
		}
		if buff.Len() > 0 {
			xlog.InfoF("<queue_match> state: %s", buff.String())
		}
	}
	// 检测是否需要调用匹配
	if mqm.tickTotal%mqm.baseCfg.MatchTickGap != 0 {
		return
	}
	// 调用一次匹配
	tryMatchOnce(mqm)
}

func (mqm *MatchQueueMgr) Destroy() {
}

// --------------- 性能收集 ---------------

type Metric struct {
	totalMatchNum  int // 匹配中总人数
	totalSupplyNum int // 增补中总人数
}

func (m *Metric) Pull(mqm *MatchQueueMgr) {
	m.totalMatchNum = 0
	m.totalSupplyNum = 0
	for _, oneQue := range mqm.waitingQueue {
		m.totalMatchNum += len(oneQue.matchElems)
		m.totalSupplyNum += len(oneQue.supplyInfos)
	}
}

func (m *Metric) Push(gather *xmetric.Gather, ch chan<- prometheus.Metric) {
	gather.PushGaugeMetric(ch, "match_totalmatch_len", float64(m.totalMatchNum), nil)
	gather.PushGaugeMetric(ch, "match_totalsupply_len", float64(m.totalSupplyNum), nil)
}
