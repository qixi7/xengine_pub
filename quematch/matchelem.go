package quematch

import (
	"time"
)

type MatchElemType uint32

const (
	MatchElemPerson MatchElemType = iota
	MatchElemTeam
	MatchElemMax
)

type MatchElemKey struct {
	ElemType MatchElemType
	ElemID   uint64
}

type iElemData interface {
	clone() iElemData
	GamerNum() int
}

type IElemFunc interface {
	OnEnterQueue(MatchQueueKey, *MatchElem)
	OnLeaveQueue(MatchQueueKey, *MatchElem, bool)
}

type MatchElem struct {
	IElemFunc
	ElemKey   MatchElemKey
	StartTime time.Time
	ElemData  iElemData
}

// 已经等待的时间.单位: 秒
func (me *MatchElem) WaitSecond() int64 {
	return int64(time.Now().Sub(me.StartTime)) / int64(time.Second)
}

func (me *MatchElem) allTypeKey() []*MatchElemKey {
	var keyArr []*MatchElemKey
	if me.ElemKey.ElemType == MatchElemPerson {
		keyArr = append(keyArr, &me.ElemKey)
	}
	if me.ElemKey.ElemType == MatchElemTeam {
		// add team key
		keyArr = append(keyArr, &me.ElemKey)
		// add all team member personal key
		for _, ply := range me.ElemData.(*ScoreMatchElemData).Gamers {
			keyArr = append(keyArr, &MatchElemKey{
				ElemType: MatchElemPerson,
				ElemID:   ply.GamerID,
			})
		}
	}

	return keyArr
}

func (me *MatchElem) clone() *MatchElem {
	cloneElem := NewMatchElem(me.ElemKey, me.ElemData.clone(), me.IElemFunc)
	cloneElem.StartTime = me.StartTime
	return cloneElem
}

// new matchElem
func NewMatchElem(key MatchElemKey, data iElemData, elemFunc IElemFunc) *MatchElem {
	return &MatchElem{
		IElemFunc: elemFunc,
		ElemKey:   key,
		StartTime: time.Now(),
		ElemData:  data,
	}
}

// ------------------------- 匹配MatchElemData --------------------------

// 分数匹配玩家额外数据
type IScoreMatchGamerExt interface {
	Clone() IScoreMatchGamerExt
}

// 分数匹配玩家
type ScoreMatchGamer struct {
	GamerID   uint64
	GamerData IScoreMatchGamerExt
}

// 分数匹配单元
type ScoreMatchElemData struct {
	Gamers []ScoreMatchGamer
}

func (smed *ScoreMatchElemData) clone() iElemData {
	cloneData := &ScoreMatchElemData{}
	*cloneData = *smed
	cloneData.Gamers = make([]ScoreMatchGamer, len(smed.Gamers))
	copy(cloneData.Gamers, smed.Gamers)
	for i := 0; i < len(smed.Gamers); i++ {
		if smed.Gamers[i].GamerData != nil {
			cloneData.Gamers[i].GamerData = smed.Gamers[i].GamerData.Clone()
		}
	}
	return cloneData
}

func (smed *ScoreMatchElemData) GamerNum() int {
	return len(smed.Gamers)
}

// new
func NewScoreMatchElemData() *ScoreMatchElemData {
	return &ScoreMatchElemData{
		Gamers: make([]ScoreMatchGamer, 0),
	}
}
